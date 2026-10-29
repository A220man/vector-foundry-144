package pty

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type PTY struct {
	File *os.File
	PID  int
	Cols int
	Rows int
}

func Open(cols, rows int, cmd *exec.Cmd) (*PTY, error) {
	// Use creack/pty to start the command with a PTY
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	// Set window size
	ws := pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	if err := pty.Setsize(f, &ws); err != nil {
		f.Close()
		return nil, err
	}

	return &PTY{
		File: f,
		PID:  cmd.Process.Pid,
		Cols: cols,
		Rows: rows,
	}, nil
}

func (p *PTY) Resize(cols, rows int) error {
	ws := pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	if err := pty.Setsize(p.File, &ws); err != nil {
		return err
	}
	p.Cols = cols
	p.Rows = rows
	return nil
}

func (p *PTY) Read(b []byte) (int, error) {
	return p.File.Read(b)
}

func (p *PTY) Write(b []byte) (int, error) {
	return p.File.Write(b)
}

func (p *PTY) Close() error {
	return p.File.Close()
}
