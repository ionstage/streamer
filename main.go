package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
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

type connection struct {
	mu      sync.Mutex
	closing bool
	conn    *websocket.Conn
	server  *server
	send    chan []byte
}

func newConnection(c *websocket.Conn, s *server) *connection {
	return &connection{conn: c, server: s, send: make(chan []byte, upgrader.WriteBufferSize)}
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
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.server.output <- msg
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
	for msg := range c.send {
		if err := c.conn.WriteMessage(t, msg); err != nil {
			break
		}
	}
}

type server struct {
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
			s.send(b)
			return nil
		})
	} else {
		readText(os.Stdin, func(t string) error {
			fmt.Println(t)
			b := unsafe.Slice(unsafe.StringData(t), len(t))
			s.send(b)
			return nil
		})
	}
}

func (s *server) receiveAndWrite() {
	for {
		select {
		case c := <-s.register:
			s.connections[c] = struct{}{}
		case c := <-s.unregister:
			delete(s.connections, c)
			close(c.send)
		case msg := <-s.output:
			if *isBinary {
				os.Stdout.Write(msg)
			} else {
				fmt.Fprintln(os.Stdout, string(msg))
			}
		}
	}
}

func (s *server) send(b []byte) {
	for c := range s.connections {
		select {
		case c.send <- b:
		default:
			delete(s.connections, c)
			close(c.send)
		}
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	c := newConnection(conn, s)
	c.open()
}

func handleServer() {
	handler := newServer()
	handler.run()
	http.Handle("/_streamer", handler)
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.ListenAndServe(":"+strconv.Itoa(*port), nil)
}

type client struct {
	mu      sync.Mutex
	closing bool
	conn    *websocket.Conn
	done    chan struct{}
}

func newClient(c *websocket.Conn) *client {
	return &client{conn: c, done: make(chan struct{})}
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
			fmt.Fprintln(os.Stderr, err)
		}
	} else {
		err := readText(os.Stdin, func(t string) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			fmt.Println(t)
			b := unsafe.Slice(unsafe.StringData(t), len(t))
			return c.conn.WriteMessage(websocket.TextMessage, b)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func (c *client) receiveAndWrite() {
	defer close(c.done)
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if *isBinary {
			os.Stdout.Write(msg)
		} else {
			fmt.Fprintln(os.Stdout, string(msg))
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
		fmt.Fprintln(os.Stderr, err)
	}
}

func handleClient() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:" + strconv.Itoa(*port), Path: "/_streamer"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

func readBinary(r io.Reader, callback func([]byte) error) error {
	in := bufio.NewReader(r)
	buf := make([]byte, upgrader.WriteBufferSize)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			err := callback(buf[0:n])
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}
}

func readText(r io.Reader, callback func(string) error) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		err := callback(scanner.Text())
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	flag.Parse()

	if *isClient {
		handleClient()
	} else {
		handleServer()
	}
}
