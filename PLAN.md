# Product development plan

Each round completes one feature or repair. Future items are planned, not implemented claims.

## Current round

minimal-skeleton

## Roadmap

### minimal-skeleton — Minimal Runnable Skeleton

Status: in_progress

Acceptance:
- go build ./cmd/tuios succeeds
- tuios new creates a session with one pane running $SHELL
- tuios ls lists sessions
- tuios attach connects to daemon and renders pane
- Ctrl+B d detaches cleanly
- Daemon survives client disconnect

Implementation: `go.mod`, `cmd/tuios/main.go`, `internal/daemon/daemon.go`, `internal/client/client.go`, `internal/model/session.go`, `internal/ipc/ipc.go`, `internal/pty/pty.go`, `internal/ui/renderer.go`

Tests: `scripts/smoke_test.sh`

### tiling-layout — Tiling Layout Engine

Status: planned

Acceptance:
- Split horizontally/vertically with ratio
- Resize panes by dragging or keys
- Focus management across panes
- Workspace 1-9 per session
- Pane tree serialization for persistence

### session-persistence — Session Persistence & Resurrection

Status: planned

Acceptance:
- Sessions survive daemon restart
- Window state (cwd, env, grid, scrollback) restored
- ResurrectionVersion for compatibility
- Auto-save on changes, load on start

### agent-inbox — Agent Inbox & State Tracking

Status: planned

Acceptance:
- Attention items for finished/question/errored/approval
- Agent state machine (working/done/idle/needs_input)
- Rail glyphs in sidebar
- agent-state events on subscribe
- Dismiss/approve actions from Inbox

### privacy-mode — Privacy Mode Renderer Pass

Status: planned

Acceptance:
- Toggle masks sensitive text in all panes
- Built-in detectors: secret, email, phone, card, iban, ip
- Custom regex patterns in config.toml
- Dock indicator shows privacy active
- Scrollback also masked

### quiet-mode — Quiet Mode per Pane

Status: planned

Acceptance:
- set-agent-quiet verb and CLI
- Suppresses finished/question/errored attention items
- Agent state, rail glyph, events unchanged
- Static config.toml list option
- Approvals still reach Inbox

### tasks-kanban — Tasks/Kanban Board

Status: planned

Acceptance:
- Task model: id, title, prompt, stage, holder, blocked, history
- Stages: todo, plan, doing, verify, review, shipped, done
- Verbs: add, ls, claim --next, done, verify, stage, block, rm
- Board tab in agents view grouped by stage
- Events: task-claimed, task-done, task-stage

### spaces — Spaces: Group Sessions + Cross-Session Move

Status: planned

Acceptance:
- Session.Space label, grouped in sidebar
- set-session-space, list-spaces, rename-space verbs
- move-window with to_session parameter
- Crash-safe move journal
- Ctrl+B {/} cycle spaces, </> cycle sessions in space

### macos-app-bundle — macOS App Bundle Launcher

Status: planned

Acceptance:
- Small LSUIElement app bundle hosts daemon
- Stable code signature for Local Network grant
- Grant propagates to all pane child processes
- Documented workaround for manual setup

