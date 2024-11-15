package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

var port = flag.Int("p", 8080, "destination port number")
var isClient = flag.Bool("c", false, "client mode")
var isBinary = flag.Bool("b", false, "binary data transfer")

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func readBinary(rd io.Reader, cb func([]byte) error) error {
	r := bufio.NewReader(rd)
	b := make([]byte, upgrader.WriteBufferSize)
	for {
		n, err := r.Read(b)
		if n > 0 {
			err := cb(b[0:n])
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}
}

func readText(r io.Reader, cb func(string) error) error {
	s := bufio.NewScanner(r)
	for s.Scan() {
		err := cb(s.Text())
		if err != nil {
			return err
		}
	}
	return s.Err()
}

func stringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func bytesToString(b []byte) string {
	return unsafe.String(&b[0], len(b))
}

type connection struct {
	mu      sync.Mutex
	closing bool
	conn    *websocket.Conn
	server  *server
	send    chan []byte
}

func newConnection(c *websocket.Conn, s *server) *connection {
	return &connection{
		conn:   c,
		server: s,
		send:   make(chan []byte, upgrader.WriteBufferSize),
	}
}

func (c *connection) open() {
	c.server.register <- c
	go c.write()
	go c.read()
}

func (c *connection) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return
	}
	c.closing = true
	c.server.unregister <- c
	c.conn.Close()
}

func (c *connection) read() {
	defer c.close()
	for {
		_, b, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.server.output <- b
	}
}

func (c *connection) write() {
	defer c.close()
	var t int
	if *isBinary {
		t = websocket.BinaryMessage
	} else {
		t = websocket.TextMessage
	}
	for b := range c.send {
		if err := c.conn.WriteMessage(t, b); err != nil {
			break
		}
	}
}

type server struct {
	mu          sync.Mutex
	connections map[*connection]struct{}
	register    chan *connection
	unregister  chan *connection
	output      chan []byte
}

func newServer() *server {
	return &server{
		connections: make(map[*connection]struct{}),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		output:      make(chan []byte),
	}
}

func (s *server) run() {
	go s.receiveAndWrite()
	go s.readAndSend()
}

func (s *server) readAndSend() {
	if *isBinary {
		readBinary(os.Stdin, func(b []byte) error {
			os.Stdout.Write(b)
			s.mu.Lock()
			s.send(b)
			s.mu.Unlock()
			return nil
		})
	} else {
		readText(os.Stdin, func(t string) error {
			fmt.Println(t)
			s.mu.Lock()
			s.send(stringToBytes(t))
			s.mu.Unlock()
			return nil
		})
	}
}

func (s *server) receiveAndWrite() {
	for {
		select {
		case c := <-s.register:
			s.mu.Lock()
			s.add(c)
			s.mu.Unlock()
		case c := <-s.unregister:
			s.mu.Lock()
			s.delete(c)
			s.mu.Unlock()
		case b := <-s.output:
			if *isBinary {
				os.Stdout.Write(b)
			} else {
				fmt.Println(bytesToString(b))
			}
		}
	}
}

func (s *server) add(c *connection) {
	if s.has(c) {
		return
	}
	s.connections[c] = struct{}{}
}

func (s *server) delete(c *connection) {
	if !s.has(c) {
		return
	}
	delete(s.connections, c)
	close(c.send)
}

func (s *server) has(c *connection) bool {
	_, ok := s.connections[c]
	return ok
}

func (s *server) send(b []byte) {
	for c := range s.connections {
		select {
		case c.send <- b:
		default:
			s.delete(c)
		}
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err)
		return
	}
	c := newConnection(conn, s)
	c.open()
}

type client struct {
	mu      sync.Mutex
	closing bool
	conn    *websocket.Conn
	done    chan struct{}
}

func newClient(c *websocket.Conn) *client {
	return &client{
		conn: c,
		done: make(chan struct{}),
	}
}

func (c *client) run() {
	go c.receiveAndWrite()
	go c.readAndSend()
}

func (c *client) readAndSend() {
	defer c.close()
	if *isBinary {
		err := readBinary(os.Stdin, func(b []byte) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			os.Stdout.Write(b)
			return c.conn.WriteMessage(websocket.BinaryMessage, b)
		})
		if err != nil {
			log.Print(err)
		}
	} else {
		err := readText(os.Stdin, func(t string) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			fmt.Println(t)
			return c.conn.WriteMessage(websocket.TextMessage, stringToBytes(t))
		})
		if err != nil {
			log.Print(err)
		}
	}
}

func (c *client) receiveAndWrite() {
	defer close(c.done)
	for {
		_, b, err := c.conn.ReadMessage()
		if err != nil {
			log.Print(err)
			return
		}
		if *isBinary {
			os.Stdout.Write(b)
		} else {
			fmt.Println(bytesToString(b))
		}
	}
}

func (c *client) close() {
	c.mu.Lock()
	defer func() {
		c.mu.Unlock()
		select {
		case <-c.done:
		case <-time.After(time.Second):
		}
	}()

	if c.closing {
		return
	}
	c.closing = true
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Print(err)
	}
}

func handleServer() {
	s := newServer()
	s.run()
	http.Handle("/_streamer", s)
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.ListenAndServe(":"+strconv.Itoa(*port), nil)
}

func handleClient() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:" + strconv.Itoa(*port), Path: "/_streamer"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Print(err)
		return
	}
	defer conn.Close()

	c := newClient(conn)
	c.run()

	for {
		select {
		case <-c.done:
			return
		case <-interrupt:
			c.close()
		}
	}
}

func main() {
	flag.Parse()

	if *isClient {
		handleClient()
	} else {
		handleServer()
	}
}
