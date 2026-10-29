package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"vector-foundry-144/internal/ipc"
	"vector-foundry-144/internal/model"
	"vector-foundry-144/internal/pty"
)

type Daemon struct {
	mu           sync.RWMutex
	socketPath   string
	listener     net.Listener
	sessions     map[string]*model.Session
	nextSessionID int
	nextWindowID  int
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewDaemon(socketPath string) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())
	return &Daemon{
		socketPath:    socketPath,
		sessions:      make(map[string]*model.Session),
		nextSessionID: 1,
		nextWindowID:  1,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (d *Daemon) Start() error {
	os.Remove(d.socketPath)
	dir := filepath.Dir(d.socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return err
	}
	d.listener = listener

	pidPath := d.socketPath + ".pid"
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	d.wg.Add(1)
	go d.acceptLoop()
	return nil
}

func (d *Daemon) Stop() error {
	d.cancel()
	if d.listener != nil {
		d.listener.Close()
	}
	d.mu.Lock()
	for _, s := range d.sessions {
		d.closeSessionLocked(s)
	}
	d.mu.Unlock()
	d.wg.Wait()
	os.Remove(d.socketPath)
	os.Remove(d.socketPath + ".pid")
	return nil
}

func (d *Daemon) acceptLoop() {
	defer d.wg.Done()
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.ctx.Done():
				return
			default:
				continue
			}
		}
		d.wg.Add(1)
		go d.handleClient(conn)
	}
}

func (d *Daemon) handleClient(conn net.Conn) {
	defer d.wg.Done()
	defer conn.Close()

	ipcConn := ipc.NewConn(conn)
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())

	for {
		msg, err := ipcConn.Recv()
		if err != nil {
			return
		}
		if err := d.handleMessage(clientID, ipcConn, msg); err != nil {
			ipcConn.Send(ipc.Message{Type: ipc.MsgError, Error: err.Error(), ID: msg.ID})
		}
	}
}

func (d *Daemon) handleMessage(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	switch msg.Type {
	case ipc.MsgNewSession:
		return d.handleNewSession(clientID, conn, msg)
	case ipc.MsgListSessions:
		return d.handleListSessions(clientID, conn, msg)
	case ipc.MsgAttach:
		return d.handleAttach(clientID, conn, msg)
	case ipc.MsgDetach:
		return d.handleDetach(clientID, conn, msg)
	case ipc.MsgKillSession:
		return d.handleKillSession(clientID, conn, msg)
	case ipc.MsgCreateWindow:
		return d.handleCreateWindow(clientID, conn, msg)
	case ipc.MsgCloseWindow:
		return d.handleCloseWindow(clientID, conn, msg)
	case ipc.MsgResizeWindow:
		return d.handleResizeWindow(clientID, conn, msg)
	case ipc.MsgInput:
		return d.handleInput(clientID, conn, msg)
	case ipc.MsgSetAgentQuiet:
		return d.handleSetAgentQuiet(clientID, conn, msg)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

func (d *Daemon) handleNewSession(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload ipc.NewSessionPayload
	if len(msg.Payload) > 0 {
		json.Unmarshal(msg.Payload, &payload)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	sessionID := fmt.Sprintf("session-%d", d.nextSessionID)
	d.nextSessionID++

	name := payload.Name
	if name == "" {
		name = fmt.Sprintf("session-%d", d.nextSessionID-1)
	}

	session := &model.Session{
		ID:         sessionID,
		Name:       name,
		Space:      payload.Space,
		CreatedAt:  time.Now(),
		Workspaces: make(map[int]*model.Workspace),
		ActiveWS:   1,
		Attached:   make(map[string]*model.Client),
	}

	ws := &model.Workspace{
		ID:      1,
		Windows: make(map[string]*model.Window),
	}
	session.Workspaces[1] = ws

	windowID := fmt.Sprintf("window-%d", d.nextWindowID)
	d.nextWindowID++
	window := d.createWindow(sessionID, 1, windowID, os.Getenv("SHELL"))
	if window == nil {
		fmt.Fprintf(os.Stderr, "Failed to create initial window\n")
	} else {
		ws.Windows[windowID] = window
		ws.Root = &model.PaneNode{Leaf: window}
		window.SessionID = sessionID
		window.WorkspaceID = 1
		ws.Focused = window
		fmt.Fprintf(os.Stderr, "Created window %s for session %s\n", windowID, sessionID)
	}

	d.sessions[sessionID] = session

	resp := ipc.Message{
		Type: ipc.MsgSessionCreated,
		ID:   msg.ID,
		Payload: mustMarshal(ipc.SessionInfo{
			ID:          sessionID,
			Name:        name,
			Space:       payload.Space,
			ActiveWS:    1,
			WindowCount: len(ws.Windows),
			Attached:    0,
			CreatedAt:   session.CreatedAt.Format(time.RFC3339),
		}),
	}
	return conn.Send(resp)
}

func (d *Daemon) handleListSessions(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sessions []ipc.SessionInfo
	for _, s := range d.sessions {
		s.Mu.RLock()
		windowCount := 0
		for _, ws := range s.Workspaces {
			windowCount += len(ws.Windows)
		}
		attached := len(s.Attached)
		createdAt := s.CreatedAt.Format(time.RFC3339)
		s.Mu.RUnlock()

		sessions = append(sessions, ipc.SessionInfo{
			ID:           s.ID,
			Name:         s.Name,
			Space:        s.Space,
			ActiveWS:     s.ActiveWS,
			WindowCount:  windowCount,
			Attached:     attached,
			CreatedAt:    createdAt,
		})
	}

	resp := ipc.Message{
		Type:    ipc.MsgSessionList,
		ID:      msg.ID,
		Payload: mustMarshal(sessions),
	}
	return conn.Send(resp)
}

func (d *Daemon) handleAttach(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload ipc.AttachPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	d.mu.Lock()
	session, ok := d.sessions[payload.SessionID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("session not found: %s", payload.SessionID)
	}

	var window *model.Window
	if payload.WindowID != "" {
		for _, ws := range session.Workspaces {
			if w, ok := ws.Windows[payload.WindowID]; ok {
				window = w
				break
			}
		}
	} else {
		if ws := session.Workspaces[session.ActiveWS]; ws != nil {
			window = ws.Focused
		}
	}

	if window == nil {
		d.mu.Unlock()
		return fmt.Errorf("no window to attach to")
	}

	client := &model.Client{
		ID:        clientID,
		SessionID: session.ID,
		WindowID:  window.ID,
		Conn:      conn.NetConn(),
		Send:      make(chan []byte, 100),
	}
	session.Attached[clientID] = client
	window.PTY.Resize(payload.Cols, payload.Rows)
	d.mu.Unlock()

	resp := ipc.Message{
		Type: ipc.MsgAttached,
		ID:   msg.ID,
		Payload: mustMarshal(map[string]interface{}{
			"session_id":  session.ID,
			"window_id":   window.ID,
			"cols":        payload.Cols,
			"rows":        payload.Rows,
			"grid_rows":   window.Grid.Rows,
			"grid_cols":   window.Grid.Cols,
			"grid_cells":  window.Grid.Cells,
			"cursor_row":  window.Cursor.Row,
			"cursor_col":  window.Cursor.Col,
		}),
	}
	conn.Send(resp)

	go d.pumpOutput(window, client)
	return nil
}

func (d *Daemon) pumpOutput(window *model.Window, client *model.Client) {
	buf := make([]byte, 4096)
	// Create a parser for this window's grid
	parser := newGridParser(window)
	for {
		n, err := window.PTY.Read(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "PTY read error: %v\n", err)
			return
		}
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			
			// Parse and update daemon's grid model, get diff
			diff := parser.Parse(data)
			
			// Send both raw data and grid diff to client
			msg := ipc.Message{
				Type: ipc.MsgWindowOutput,
				Payload: mustMarshal(ipc.WindowOutputPayload{
					WindowID: window.ID,
					Data:     data,
					GridDiff: &diff,
				}),
			}
			select {
			case client.Send <- mustMarshal(msg):
			default:
			}
		}
	}
}

// gridParser parses ANSI output and updates a Window's Grid and Scrollback
type gridParser struct {
	window      *model.Window
	cursor      model.CursorPos
	dirtyCells  []ipc.CellUpdate
	scrollDelta int
}

func newGridParser(w *model.Window) *gridParser {
	return &gridParser{
		window:     w,
		cursor:     model.CursorPos{Row: 0, Col: 0},
		dirtyCells: make([]ipc.CellUpdate, 0, 64),
	}
}

func (p *gridParser) Parse(data []byte) ipc.GridDiff {
	p.dirtyCells = p.dirtyCells[:0]
	p.scrollDelta = 0
	
	p.window.Mu.Lock()
	defer p.window.Mu.Unlock()
	
	grid := p.window.Grid
	if grid == nil {
		return ipc.GridDiff{}
	}
	
	i := 0
	for i < len(data) {
		if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '[' {
			i += 2
			params := []int{0}
			for i < len(data) {
				if data[i] >= '0' && data[i] <= '9' {
					params[len(params)-1] = params[len(params)-1]*10 + int(data[i]-'0')
				} else if data[i] == ';' {
					params = append(params, 0)
				} else {
					break
				}
				i++
			}
			if i < len(data) {
				p.handleCSI(grid, params, data[i])
				i++
			}
		} else if data[i] == '\n' {
			p.cursor.Row++
			p.cursor.Col = 0
			if p.cursor.Row >= grid.Rows {
				p.scrollUp(grid, 1)
				p.cursor.Row = grid.Rows - 1
				p.scrollDelta++
			}
			i++
		} else if data[i] == '\r' {
			p.cursor.Col = 0
			i++
		} else if data[i] == '\t' {
			p.cursor.Col = (p.cursor.Col/8 + 1) * 8
			if p.cursor.Col >= grid.Cols {
				p.cursor.Col = grid.Cols - 1
			}
			i++
		} else if data[i] >= 0x20 {
			if p.cursor.Row < grid.Rows && p.cursor.Col < grid.Cols {
				cell := model.Cell{Rune: rune(data[i]), Style: model.Style{}}
				grid.Cells[p.cursor.Row][p.cursor.Col] = cell
				p.dirtyCells = append(p.dirtyCells, ipc.CellUpdate{
					Row: p.cursor.Row, Col: p.cursor.Col, Cell: cell,
				})
				p.cursor.Col++
				if p.cursor.Col >= grid.Cols {
					p.cursor.Col = 0
					p.cursor.Row++
					if p.cursor.Row >= grid.Rows {
						p.scrollUp(grid, 1)
						p.cursor.Row = grid.Rows - 1
						p.scrollDelta++
					}
				}
			}
			i++
		} else {
			i++
		}
	}
	// Update window's cursor position
	p.window.Cursor = p.cursor
	
	// Build and return diff
	return ipc.GridDiff{
		Cursor: p.cursor,
		Cells:  p.dirtyCells,
		Scroll: p.scrollDelta,
	}
}


func (p *gridParser) handleCSI(grid *model.Grid, params []int, final byte) {
	switch final {
	case 'H', 'f': // Cursor position
		row := 1
		col := 1
		if len(params) > 0 {
			row = params[0]
		}
		if len(params) > 1 {
			col = params[1]
		}
		p.cursor.Row = row - 1
		p.cursor.Col = col - 1
	case 'J': // Erase in display
		if len(params) > 0 && params[0] == 2 {
			for i := range grid.Cells {
				for j := range grid.Cells[i] {
					grid.Cells[i][j] = model.Cell{Rune: ' ', Style: model.Style{}}
				}
			}
			p.cursor.Row, p.cursor.Col = 0, 0
		}
	case 'K': // Erase in line
		if len(params) > 0 && params[0] == 2 && p.cursor.Row < grid.Rows {
			for j := range grid.Cells[p.cursor.Row] {
				grid.Cells[p.cursor.Row][j] = model.Cell{Rune: ' ', Style: model.Style{}}
			}
		}
	case 'A': // Cursor up
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		p.cursor.Row -= n
		if p.cursor.Row < 0 {
			p.cursor.Row = 0
		}
	case 'B': // Cursor down
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		p.cursor.Row += n
		if p.cursor.Row >= grid.Rows {
			p.cursor.Row = grid.Rows - 1
		}
	case 'C': // Cursor forward
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		p.cursor.Col += n
		if p.cursor.Col >= grid.Cols {
			p.cursor.Col = grid.Cols - 1
		}
	case 'D': // Cursor backward
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		p.cursor.Col -= n
		if p.cursor.Col < 0 {
			p.cursor.Col = 0
		}
	}
}

func (p *gridParser) scrollUp(grid *model.Grid, n int) {
	// Capture lines scrolling off top into scrollback
	if p.window.Scrollback != nil {
		p.window.Scrollback.Mu.Lock()
		for i := 0; i < n && i < grid.Rows; i++ {
			line := model.Line{Cells: make([]model.Cell, grid.Cols)}
			copy(line.Cells, grid.Cells[i])
			p.window.Scrollback.Lines = append(p.window.Scrollback.Lines, line)
			if len(p.window.Scrollback.Lines) > p.window.Scrollback.Max {
				p.window.Scrollback.Lines = p.window.Scrollback.Lines[len(p.window.Scrollback.Lines)-p.window.Scrollback.Max:]
			}
		}
		p.window.Scrollback.Mu.Unlock()
	}
	
	copy(grid.Cells, grid.Cells[n:])
	for i := grid.Rows - n; i < grid.Rows; i++ {
		for j := range grid.Cells[i] {
			grid.Cells[i][j] = model.Cell{Rune: ' ', Style: model.Style{}}
		}
	}
}

func (d *Daemon) handleDetach(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, s := range d.sessions {
		if _, ok := s.Attached[clientID]; ok {
			delete(s.Attached, clientID)
			resp := ipc.Message{Type: ipc.MsgDetached, ID: msg.ID}
			return conn.Send(resp)
		}
	}
	return fmt.Errorf("client not attached")
}

func (d *Daemon) handleKillSession(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(msg.Payload, &payload)

	d.mu.Lock()
	session, ok := d.sessions[payload.SessionID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("session not found")
	}
	d.closeSessionLocked(session)
	delete(d.sessions, payload.SessionID)
	d.mu.Unlock()

	resp := ipc.Message{Type: ipc.MsgOK, ID: msg.ID}
	return conn.Send(resp)
}

func (d *Daemon) closeSessionLocked(session *model.Session) {
	for _, ws := range session.Workspaces {
		for _, w := range ws.Windows {
			if w.PTY != nil {
				w.PTY.Close()
			}
		}
	}
}

func (d *Daemon) handleCreateWindow(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload struct {
		SessionID     string `json:"session_id"`
		Workspace     int    `json:"workspace"`
		Command       string `json:"command,omitempty"`
		SplitDir      string `json:"split_dir,omitempty"`
		TargetWindow  string `json:"target_window,omitempty"`
	}
	json.Unmarshal(msg.Payload, &payload)

	d.mu.Lock()
	defer d.mu.Unlock()

	session, ok := d.sessions[payload.SessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}

	wsID := payload.Workspace
	if wsID == 0 {
		wsID = session.ActiveWS
	}
	ws := session.Workspaces[wsID]
	if ws == nil {
		ws = &model.Workspace{ID: wsID, Windows: make(map[string]*model.Window)}
		session.Workspaces[wsID] = ws
	}

	windowID := fmt.Sprintf("window-%d", d.nextWindowID)
	d.nextWindowID++

	cmd := payload.Command
	if cmd == "" {
		cmd = os.Getenv("SHELL")
	}

	window := d.createWindow(payload.SessionID, wsID, windowID, cmd)
	if window == nil {
		return fmt.Errorf("failed to create window")
	}

	ws.Windows[windowID] = window
	window.SessionID = payload.SessionID
	window.WorkspaceID = wsID

	if ws.Root == nil {
		ws.Root = &model.PaneNode{Leaf: window}
	} else {
		ws.Root = &model.PaneNode{
			Split: &model.SplitNode{
				Horizontal: payload.SplitDir == "h",
				Ratio:      0.5,
				Children: [2]*model.PaneNode{
					ws.Root,
					{Leaf: window},
				},
			},
		}
	}

	resp := ipc.Message{
		Type: ipc.MsgOK,
		ID:   msg.ID,
		Payload: mustMarshal(map[string]string{"window_id": windowID}),
	}
	return conn.Send(resp)
}

func (d *Daemon) createWindow(sessionID string, wsID int, windowID, cmd string) *model.Window {
	shell := cmd
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/bash"
	}

	c := exec.Command(shell)
	p, err := pty.Open(80, 24, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "PTY open error: %v\n", err)
		return nil
	}

	grid := &model.Grid{Rows: 24, Cols: 80, Cells: make([][]model.Cell, 24)}
	for i := range grid.Cells {
		grid.Cells[i] = make([]model.Cell, 80)
	}

	scrollback := &model.Scrollback{Lines: make([]model.Line, 0), Max: 10000}

	return &model.Window{
		ID:          windowID,
		SessionID:   sessionID,
		WorkspaceID: wsID,
		Title:       shell,
		PTY:         p,
		Scrollback:  scrollback,
		Grid:        grid,
		AgentState:  model.AgentStateUnknown,
		CreatedAt:   time.Now(),
	}
}

func (d *Daemon) handleCloseWindow(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload struct {
		WindowID string `json:"window_id"`
	}
	json.Unmarshal(msg.Payload, &payload)

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, s := range d.sessions {
		for _, ws := range s.Workspaces {
			if w, ok := ws.Windows[payload.WindowID]; ok {
				if w.PTY != nil {
					w.PTY.Close()
				}
				delete(ws.Windows, payload.WindowID)
				resp := ipc.Message{
					Type: ipc.MsgWindowClosed,
					ID:   msg.ID,
					Payload: mustMarshal(map[string]string{"window_id": payload.WindowID}),
				}
				return conn.Send(resp)
			}
		}
	}
	return fmt.Errorf("window not found")
}

func (d *Daemon) handleResizeWindow(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload ipc.ResizePayload
	json.Unmarshal(msg.Payload, &payload)

	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.sessions {
		for _, ws := range s.Workspaces {
			if w, ok := ws.Windows[payload.WindowID]; ok {
				w.PTY.Resize(payload.Cols, payload.Rows)
				w.Grid.Rows = payload.Rows
				w.Grid.Cols = payload.Cols
				newCells := make([][]model.Cell, payload.Rows)
				for i := range newCells {
					newCells[i] = make([]model.Cell, payload.Cols)
				}
				w.Grid.Cells = newCells
				resp := ipc.Message{Type: ipc.MsgWindowResized, ID: msg.ID}
				return conn.Send(resp)
			}
		}
	}
	return fmt.Errorf("window not found")
}

func (d *Daemon) handleInput(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload ipc.InputPayload
	json.Unmarshal(msg.Payload, &payload)

	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.sessions {
		for _, ws := range s.Workspaces {
			if w, ok := ws.Windows[payload.WindowID]; ok {
				var data []byte
				if len(payload.Raw) > 0 {
					data = payload.Raw
				} else if payload.Rune != 0 {
					data = []byte(string(payload.Rune))
				} else {
					data = []byte(payload.Key)
				}
				w.PTY.Write(data)
				return nil
			}
		}
	}
	return fmt.Errorf("window not found")
}

func (d *Daemon) handleSetAgentQuiet(clientID string, conn *ipc.Conn, msg ipc.Message) error {
	var payload ipc.SetAgentQuietPayload
	json.Unmarshal(msg.Payload, &payload)

	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.sessions {
		for _, ws := range s.Workspaces {
			if w, ok := ws.Windows[payload.WindowID]; ok {
				w.Mu.Lock()
				w.Quiet = payload.Quiet
				w.Mu.Unlock()
				resp := ipc.Message{Type: ipc.MsgOK, ID: msg.ID}
				return conn.Send(resp)
			}
		}
	}
	return fmt.Errorf("window not found")
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
