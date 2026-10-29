package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"unsafe"

	"vector-foundry-144/internal/ipc"
	"vector-foundry-144/internal/ui"
)

type Client struct {
	socketPath string
	conn       net.Conn
	ipcConn    *ipc.Conn
	renderer   *ui.Renderer
	sessionID  string
	windowID   string
	cols       int
	rows       int
	done       chan struct{}
}

func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		done:       make(chan struct{}),
	}
}

func (c *Client) Connect() error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return err
	}
	c.conn = conn
	c.ipcConn = ipc.NewConn(conn)
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) NewSession(name, space string) (string, error) {
	msg := ipc.Message{
		Type: ipc.MsgNewSession,
		Payload: mustMarshal(ipc.NewSessionPayload{
			Name:  name,
			Space: space,
		}),
	}
	if err := c.ipcConn.Send(msg); err != nil {
		return "", err
	}

	resp, err := c.ipcConn.Recv()
	if err != nil {
		return "", err
	}
	if resp.Type == ipc.MsgError {
		return "", fmt.Errorf("%s", resp.Error)
	}

	var info ipc.SessionInfo
	json.Unmarshal(resp.Payload, &info)
	return info.ID, nil
}

func (c *Client) ListSessions() ([]ipc.SessionInfo, error) {
	msg := ipc.Message{Type: ipc.MsgListSessions}
	if err := c.ipcConn.Send(msg); err != nil {
		return nil, err
	}

	resp, err := c.ipcConn.Recv()
	if err != nil {
		return nil, err
	}
	if resp.Type == ipc.MsgError {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	var sessions []ipc.SessionInfo
	json.Unmarshal(resp.Payload, &sessions)
	return sessions, nil
}

func (c *Client) Attach(sessionID, windowID string, cols, rows int) error {
	c.sessionID = sessionID
	c.windowID = windowID
	c.cols = cols
	c.rows = rows

	msg := ipc.Message{
		Type: ipc.MsgAttach,
		Payload: mustMarshal(ipc.AttachPayload{
			SessionID: sessionID,
			WindowID:  windowID,
			Cols:      cols,
			Rows:      rows,
		}),
	}
	if err := c.ipcConn.Send(msg); err != nil {
		return err
	}

	resp, err := c.ipcConn.Recv()
	if err != nil {
		return err
	}
	if resp.Type == ipc.MsgError {
		return fmt.Errorf("%s", resp.Error)
	}

	c.renderer = ui.NewRenderer(cols, rows)
	c.handleResize()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			c.handleResize()
		}
	}()

	return c.eventLoop()
}

func (c *Client) handleResize() {
	ws, err := getWinsize()
	if err == nil {
		c.cols = int(ws.Cols)
		c.rows = int(ws.Rows)
		c.renderer.Resize(c.cols, c.rows)
		c.ipcConn.Send(ipc.Message{
			Type: ipc.MsgResizeWindow,
			Payload: mustMarshal(ipc.ResizePayload{
				WindowID: c.windowID,
				Cols:     c.cols,
				Rows:     c.rows,
			}),
		})
	}
}

func (c *Client) eventLoop() error {
	oldState, err := ui.EnableRawMode()
	if err != nil {
		return err
	}
	defer ui.DisableRawMode(oldState)

	go c.readDaemonLoop()

	buf := make([]byte, 1024)
	for {
		select {
		case <-c.done:
			return nil
		default:
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return err
			}
			if n > 0 {
				c.handleInput(buf[:n])
			}
		}
	}
}

func (c *Client) readDaemonLoop() {
	for {
		msg, err := c.ipcConn.Recv()
		if err != nil {
			close(c.done)
			return
		}
		c.handleDaemonMessage(msg)
	}
}

func (c *Client) handleDaemonMessage(msg ipc.Message) {
	switch msg.Type {
	case ipc.MsgWindowOutput:
		var payload ipc.WindowOutputPayload
		json.Unmarshal(msg.Payload, &payload)
		if c.renderer != nil {
			c.renderer.Write(payload.Data)
			c.renderer.Render()
		}
	case ipc.MsgWindowResized:
	case ipc.MsgWindowClosed:
	case ipc.MsgDetached:
		close(c.done)
	case ipc.MsgError:
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg.Error)
	}
}

func (c *Client) handleInput(data []byte) {
	if len(data) == 1 && data[0] == 0x02 {
		buf := make([]byte, 1)
		os.Stdin.Read(buf)
		c.handlePrefixKey(buf[0])
		return
	}

	c.ipcConn.Send(ipc.Message{
		Type: ipc.MsgInput,
		Payload: mustMarshal(ipc.InputPayload{
			WindowID: c.windowID,
			Raw:      data,
		}),
	})
}

func (c *Client) handlePrefixKey(key byte) {
	switch key {
	case 'd':
		c.ipcConn.Send(ipc.Message{Type: ipc.MsgDetach})
		close(c.done)
	case 'c':
		c.ipcConn.Send(ipc.Message{
			Type: ipc.MsgCreateWindow,
			Payload: mustMarshal(map[string]string{
				"session_id": c.sessionID,
				"workspace":  "1",
			}),
		})
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

type winsize struct {
	Rows    uint16
	Cols    uint16
	Xpixels uint16
	Ypixels uint16
}

func getWinsize() (winsize, error) {
	var ws winsize
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(os.Stdin.Fd()),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
		0, 0, 0,
	)
	if errno != 0 {
		return ws, errno
	}
	return ws, nil
}
