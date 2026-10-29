# vector-foundry-144

> Work in progress. This checkpoint may be incomplete or fail tests. See PLAN.md and STATUS.md before use.


A terminal window manager that knows what your agents are doing. Tiling panes, workspaces, sessions that survive restarts, and one Inbox for every coding agent.

Official only at tuios.dev and this repository. vector-foundry-144 has no X/Twitter account.

## Status

**Phase 1: Minimal Runnable Skeleton** - In Progress

This is an early development checkpoint. The basic daemon/client architecture is implemented but core rendering and synchronization features are incomplete.

### What Works
- `tuios new [name]` - Create a new session with a shell pane
- `tuios ls` - List all sessions
- `tuios attach [session]` - Attach to a session
- `tuios kill-server` - Stop the daemon
- `tuios version` - Show version
- Daemon survives client disconnect/reattach
- PTY-based pane spawning

### What's Incomplete
- Terminal output not synced to daemon model (grid, scrollback)
- ANSI parser missing SGR, alternate screen, cursor save/restore
- No tiling layout (splits created but not rendered)
- No session persistence across daemon restarts
- No agent inbox or state tracking

## Architecture

```
┌─────────────┐     Unix Socket      ┌─────────────┐
│   Client    │ ◄──────────────────► │   Daemon    │
│  (renderer) │   JSON messages      │  (sessions) │
└─────────────┘                      └─────────────┘
                                             │
                                      ┌──────┴──────┐
                                      ▼             ▼
                                 ┌─────────┐  ┌─────────┐
                                 │  PTY    │  │  PTY    │
                                 │ (shell) │  │ (shell) │
                                 └─────────┘  └─────────┘
```

## Building

```bash
go build -o tuios ./cmd/tuios
```

## Running

```bash
# Start a new session (auto-starts daemon)
./tuios new my-session

# List sessions
./tuios ls

# Attach to most recent session
./tuios attach

# Detach: Ctrl+B d
# New window: Ctrl+B c
# Kill daemon
./tuios kill-server
```

## Configuration

Configuration will be in `~/.config/tuios/config.toml` (not yet implemented).

## Development

See `PLAN.md` for implementation roadmap and `STATUS.md` for current checkpoint.

## License

MIT

Maintainer: Ryan Vo <ryandtvo@gmail.com>
