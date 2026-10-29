package ipc

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	ID      string          `json:"id,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type Conn struct {
	mu       sync.Mutex
	conn     net.Conn
	encoder  *json.Encoder
	decoder  *json.Decoder
	reader   *bufio.Reader
	writer   *bufio.Writer
	closeOnce sync.Once
}

func NewConn(conn net.Conn) *Conn {
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	return &Conn{
		conn:    conn,
		encoder: json.NewEncoder(w),
		decoder: json.NewDecoder(r),
		reader:  r,
		writer:  w,
	}
}

func (c *Conn) Send(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.encoder.Encode(msg); err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *Conn) Recv() (Message, error) {
	var msg Message
	err := c.decoder.Decode(&msg)
	return msg, err
}

func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.conn.Close()
	})
	return err
}

// NetConn returns the underlying net.Conn
func (c *Conn) NetConn() net.Conn {
	return c.conn
}
