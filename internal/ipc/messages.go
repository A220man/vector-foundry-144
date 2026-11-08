package ipc

import "vector-foundry-144/internal/model"

// Message types
const (
	MsgNewSession     = "new_session"
	MsgListSessions   = "list_sessions"
	MsgAttach         = "attach"
	MsgDetach         = "detach"
	MsgKillSession    = "kill_session"
	MsgCreateWindow   = "create_window"
	MsgCloseWindow    = "close_window"
	MsgResizeWindow   = "resize_window"
	MsgInput          = "input"
	MsgSetAgentQuiet  = "set_agent_quiet"
	
	MsgSessionCreated = "session_created"
	MsgSessionList    = "session_list"
	MsgAttached       = "attached"
	MsgDetached       = "detached"
	MsgWindowOutput   = "window_output"
	MsgWindowResized  = "window_resized"
	MsgWindowClosed   = "window_closed"
	MsgAgentState     = "agent_state"
	MsgAttentionItem  = "attention_item"
	MsgError          = "error"
	MsgOK             = "ok"
	MsgSplitWindow    = "split_window"
	MsgResizePane     = "resize_pane"
	MsgFocusPane      = "focus_pane"
	MsgSwitchWorkspace = "switch_workspace"
	MsgListWorkspaces  = "list_workspaces"
	MsgPaneTree       = "pane_tree"
)

type NewSessionPayload struct {
	Name  string `json:"name,omitempty"`
	Space string `json:"space,omitempty"`
}

type SessionInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Space        string `json:"space"`
	ActiveWS     int    `json:"active_ws"`
	WindowCount  int    `json:"window_count"`
	Attached     int    `json:"attached"`
	CreatedAt    string `json:"created_at"`
}

type AttachPayload struct {
	SessionID string `json:"session_id"`
	WindowID  string `json:"window_id,omitempty"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type WindowOutputPayload struct {
	WindowID string `json:"window_id"`
	Data     []byte `json:"data"`
	GridDiff *GridDiff `json:"grid_diff,omitempty"`
}

type GridDiff struct {
	Cursor model.CursorPos `json:"cursor"`
	Cells  []CellUpdate    `json:"cells"`
	Scroll int             `json:"scroll,omitempty"`
}

type CellUpdate struct {
	Row, Col int        `json:"row,col"`
	Cell     model.Cell `json:"cell"`
}

type InputPayload struct {
	WindowID string `json:"window_id"`
	Key      string `json:"key"`
	Rune     rune   `json:"rune"`
	Mods     string `json:"mods"`
	Raw      []byte `json:"raw,omitempty"`
}

type ResizePayload struct {
	WindowID string `json:"window_id"`
	Cols     int    `json:"cols"`
	Rows     int    `json:"rows"`
}

type AgentStatePayload struct {
	WindowID  string        `json:"window_id"`
	State     model.AgentState `json:"state"`
	Seq       int           `json:"seq"`
	Timestamp string        `json:"timestamp"`
}

type AttentionItemPayload struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	WindowID  string `json:"window_id"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Actions   []AttentionAction `json:"actions,omitempty"`
}

type AttentionAction struct {
	Label string            `json:"label"`
	Verb  string            `json:"verb"`
	Args  map[string]string `json:"args,omitempty"`
}

type SplitWindowPayload struct {
	SessionID    string  `json:"session_id"`
	WindowID     string  `json:"window_id"`
	Direction    string  `json:"direction"`
	Ratio        float64 `json:"ratio"`
	Command      string  `json:"command,omitempty"`
}

type ResizePanePayload struct {
	SessionID string  `json:"session_id"`
	WindowID  string  `json:"window_id"`
	Delta     int     `json:"delta"`
	Direction string  `json:"direction"`
}

type FocusPanePayload struct {
	SessionID string `json:"session_id"`
	WindowID  string `json:"window_id"`
}

type SwitchWorkspacePayload struct {
	SessionID string `json:"session_id"`
	Workspace int    `json:"workspace"`
}

type ListWorkspacesPayload struct {
	SessionID string `json:"session_id"`
}

type WorkspaceInfo struct {
	ID        int      `json:"id"`
	Active    bool     `json:"active"`
	WindowIDs []string `json:"window_ids"`
}

type ListWorkspacesResponse struct {
	Workspaces []WorkspaceInfo `json:"workspaces"`
	Active     int             `json:"active"`
}

type PaneTreePayload struct {
	SessionID  string         `json:"session_id"`
	Workspace  int            `json:"workspace"`
	Root       *PaneTreeNode  `json:"root"`
}

type PaneTreeNode struct {
	Split      *SplitNode     `json:"split,omitempty"`
	WindowID   string         `json:"window_id,omitempty"`
	Focused    bool           `json:"focused,omitempty"`
}

type SplitNode struct {
	Horizontal bool           `json:"horizontal"`
	Ratio      float64        `json:"ratio"`
	Children   [2]*PaneTreeNode `json:"children"`
}

type SetAgentQuietPayload struct {
	WindowID string `json:"window_id"`
	Quiet    bool   `json:"quiet"`
}
