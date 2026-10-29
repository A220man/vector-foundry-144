package model

import (
	"net"
	"sync"
	"time"

	"vector-foundry-144/internal/pty"
)

type Session struct {
	Mu         sync.RWMutex
	ID         string
	Name       string
	Space      string
	CreatedAt  time.Time
	Workspaces map[int]*Workspace
	ActiveWS   int
	Attached   map[string]*Client
}

type Workspace struct {
	Mu      sync.RWMutex
	ID      int
	Root    *PaneNode
	Windows map[string]*Window
	Focused *Window
}

type PaneNode struct {
	Mu     sync.RWMutex
	Split  *SplitNode
	Leaf   *Window
	Parent *PaneNode
}

type SplitNode struct {
	Horizontal bool
	Ratio      float64
	Children   [2]*PaneNode
}

type Window struct {
	Mu          sync.RWMutex
	ID          string
	SessionID   string
	WorkspaceID int
	Title       string
	PTY         *pty.PTY
	Scrollback  *Scrollback
	Cursor      CursorPos
	Grid        *Grid
	AgentState  AgentState
	Quiet       bool
	CreatedAt   time.Time
}

type Scrollback struct {
	Mu    sync.Mutex
	Lines []Line
	Max   int
}

type Line struct {
	Cells   []Cell
	Wrapped bool
}

type Cell struct {
	Rune  rune
	Style Style
}

type Style struct {
	FG        Color
	BG        Color
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	Reverse   bool
	Blink     bool
}

type Color struct {
	R, G, B     uint8
	IsDefault   bool
	IsIndexed   bool
	Index       uint8
}

type CursorPos struct {
	Row, Col int
	Hidden   bool
	Shape    CursorShape
}

type CursorShape int

const (
	CursorBlock CursorShape = iota
	CursorUnderline
	CursorBar
)

type Grid struct {
	Mu     sync.Mutex
	Rows   int
	Cols   int
	Cells  [][]Cell
	Cursor CursorPos
}

type AgentState string

const (
	AgentStateUnknown     AgentState = "unknown"
	AgentStateWorking     AgentState = "working"
	AgentStateDone        AgentState = "done"
	AgentStateIdle        AgentState = "idle"
	AgentStateNeedsInput  AgentState = "needs_input"
)

type Client struct {
	ID        string
	SessionID string
	WindowID  string
	Conn      net.Conn
	Send      chan []byte
}
