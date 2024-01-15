package main

import (
	"bufio"
	"flag"
	"fmt"
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
var listen = flag.Bool("l", false, "listen mode")
var binary = flag.Bool("b", false, "binary data transfer")

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func server(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	go func() {
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				break
			}
			if *binary {
				os.Stdout.Write(msg)
			} else {
				fmt.Fprintln(os.Stdout, string(msg))
			}
		}
	}()
}

func handleListener() {
	http.HandleFunc("/ws", server)
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.ListenAndServe(":"+strconv.Itoa(*port), nil)
}

type Closer struct {
	mu      sync.Mutex
	closing bool
	conn    *websocket.Conn
}

func (c *Closer) Close() {
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

func handleSender() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:" + strconv.Itoa(*port), Path: "/ws"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer c.Close()

	done := make(chan struct{})

	closer := Closer{conn: c}

	go func() {
		defer close(done)
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				return
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
		if *binary {
			in := bufio.NewReader(os.Stdin)
			buf := make([]byte, upgrader.WriteBufferSize)
			for {
				n, err := in.Read(buf)
				if n > 0 {
					b := buf[0:n]
					err := c.WriteMessage(websocket.BinaryMessage, b)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						break
					}
					os.Stdout.Write(b)
				}
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					break
				}
			}
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				s := scanner.Text()
				b := unsafe.Slice(unsafe.StringData(s), len(s))
				err := c.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					break
				}
				fmt.Println(s)
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

func main() {
	flag.Parse()

	if *listen {
		handleListener()
	} else {
		handleSender()
	}
}
