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
	conn    *websocket.Conn
	handler *streamHandler
	send    chan []byte
}

func (c *connection) start() {
	c.handler.register <- c
	go c.write()
	go c.read()
}

func (c *connection) stop() {
	c.handler.unregister <- c
	c.conn.Close()
}

func (c *connection) read() {
	defer c.stop()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.handler.output <- msg
	}
}

func (c *connection) write() {
	defer c.conn.Close()
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

type streamHandler struct {
	connections map[*connection]struct{}
	register    chan *connection
	unregister  chan *connection
	output      chan []byte
}

func newStreamHandler() *streamHandler {
	return &streamHandler{
		connections: make(map[*connection]struct{}),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		output:      make(chan []byte),
	}
}

func (h *streamHandler) run() {
	go func() {
		if *isBinary {
			readBinary(os.Stdin, func(b []byte) error {
				os.Stdout.Write(b)
				for c := range h.connections {
					select {
					case c.send <- b:
					default:
						delete(h.connections, c)
						close(c.send)
					}
				}
				return nil
			})
		} else {
			readText(os.Stdin, func(s string) error {
				fmt.Println(s)
				b := unsafe.Slice(unsafe.StringData(s), len(s))
				for c := range h.connections {
					select {
					case c.send <- b:
					default:
						delete(h.connections, c)
						close(c.send)
					}
				}
				return nil
			})
		}
	}()

	for {
		select {
		case c := <-h.register:
			h.connections[c] = struct{}{}
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
		case msg := <-h.output:
			if *isBinary {
				os.Stdout.Write(msg)
			} else {
				fmt.Fprintln(os.Stdout, string(msg))
			}
		}
	}
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	c := &connection{conn: conn, handler: h, send: make(chan []byte, upgrader.WriteBufferSize)}
	c.start()
}

func handleServer() {
	handler := newStreamHandler()
	go handler.run()
	http.Handle("/_streamer", handler)
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.ListenAndServe(":"+strconv.Itoa(*port), nil)
}

type connectionCloser struct {
	mu      sync.Mutex
	closing bool
	conn    *websocket.Conn
}

func (c *connectionCloser) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer c.Close()

	done := make(chan struct{})

	closer := connectionCloser{conn: c}

	go func() {
		defer close(done)
		for {
			_, msg, err := c.ReadMessage()
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
	}()

	go func() {
		defer func() {
			closer.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}()

		if *isBinary {
			err := readBinary(os.Stdin, func(b []byte) error {
				closer.mu.Lock()
				defer closer.mu.Unlock()
				os.Stdout.Write(b)
				return c.WriteMessage(websocket.BinaryMessage, b)
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			err := readText(os.Stdin, func(s string) error {
				closer.mu.Lock()
				defer closer.mu.Unlock()
				fmt.Println(s)
				b := unsafe.Slice(unsafe.StringData(s), len(s))
				return c.WriteMessage(websocket.TextMessage, b)
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			closer.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
			}
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