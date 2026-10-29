# Changelog

## 1.0.0 — work-in-progress checkpoint

Partial code published for continued development. Known issues: STATUS.md.

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-10-27

### Added
- Initial project structure with Go module
- Daemon/client architecture over Unix domain sockets
- Session management (create, list, kill)
- Window/pane management with PTY spawning
- Basic IPC protocol with JSON messages
- Minimal terminal renderer with ANSI parsing
- Keyboard input handling with Ctrl+B prefix keys
- Smoke test script
- **Daemon Grid synchronization from PTY output** via gridParser
- **Scrollback capture** on terminal scroll
- **GridDiff computation** for incremental updates (dirty cells, cursor position, scroll delta)
- **GridDiff sent with MsgWindowOutput** for efficient client rendering
- **Grid state included in attach response** for immediate render on re-attach

### Known Issues
- Daemon grid not synced from PTY output (FIXED)
- Scrollback not captured (FIXED)
- ANSI parser incomplete
- No tiling layout engine
- No session persistence
- No agent inbox/state tracking
- Client doesn't apply GridDiff (still re-parses raw data)
- Client doesn't apply grid state on attach
