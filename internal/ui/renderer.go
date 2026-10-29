package ui

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Renderer handles terminal rendering
type Renderer struct {
	cols, rows int
	grid       [][]Cell
	cursor     CursorPos
	altScreen  bool
}

// Cell represents a terminal cell
type Cell struct {
	Rune  rune
	Style Style
}

// Style holds cell formatting
type Style struct {
	FG, BG   Color
	Bold     bool
	Italic   bool
	Underline bool
}

// Color represents a color
type Color struct {
	R, G, B uint8
	Default bool
}

// CursorPos is cursor position
type CursorPos struct {
	Row, Col int
	Hidden   bool
}

// NewRenderer creates a new renderer
func NewRenderer(cols, rows int) *Renderer {
	grid := make([][]Cell, rows)
	for i := range grid {
		grid[i] = make([]Cell, cols)
		for j := range grid[i] {
			grid[i][j] = Cell{Rune: ' ', Style: Style{}}
		}
	}
	return &Renderer{
		cols:  cols,
		rows:  rows,
		grid:  grid,
		cursor: CursorPos{Row: 0, Col: 0},
	}
}

// Resize resizes the renderer
func (r *Renderer) Resize(cols, rows int) {
	newGrid := make([][]Cell, rows)
	for i := range newGrid {
		newGrid[i] = make([]Cell, cols)
		for j := range newGrid[i] {
			if i < r.rows && j < r.cols {
				newGrid[i][j] = r.grid[i][j]
			} else {
				newGrid[i][j] = Cell{Rune: ' ', Style: Style{}}
			}
		}
	}
	r.grid = newGrid
	r.cols = cols
	r.rows = rows
}

// Write writes raw terminal output to the renderer
func (r *Renderer) Write(data []byte) {
	r.parseANSI(data)
}

// parseANSI parses ANSI escape sequences (simplified)
func (r *Renderer) parseANSI(data []byte) {
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
				r.handleCSI(params, data[i])
				i++
			}
		} else if data[i] == '\n' {
			r.cursor.Row++
			r.cursor.Col = 0
			if r.cursor.Row >= r.rows {
				r.scrollUp(1)
				r.cursor.Row = r.rows - 1
			}
			i++
		} else if data[i] == '\r' {
			r.cursor.Col = 0
			i++
		} else if data[i] == '\t' {
			r.cursor.Col = (r.cursor.Col/8 + 1) * 8
			if r.cursor.Col >= r.cols {
				r.cursor.Col = r.cols - 1
			}
			i++
		} else if data[i] >= 0x20 {
			if r.cursor.Row < r.rows && r.cursor.Col < r.cols {
				r.grid[r.cursor.Row][r.cursor.Col] = Cell{Rune: rune(data[i]), Style: Style{}}
				r.cursor.Col++
				if r.cursor.Col >= r.cols {
					r.cursor.Col = 0
					r.cursor.Row++
					if r.cursor.Row >= r.rows {
						r.scrollUp(1)
						r.cursor.Row = r.rows - 1
					}
				}
			}
			i++
		} else {
			i++
		}
	}
}

func (r *Renderer) handleCSI(params []int, final byte) {
	switch final {
	case 'H', 'f':
		row := 1
		col := 1
		if len(params) > 0 {
			row = params[0]
		}
		if len(params) > 1 {
			col = params[1]
		}
		r.cursor.Row = row - 1
		r.cursor.Col = col - 1
	case 'J':
		if len(params) > 0 && params[0] == 2 {
			for i := range r.grid {
				for j := range r.grid[i] {
					r.grid[i][j] = Cell{Rune: ' ', Style: Style{}}
				}
			}
			r.cursor.Row, r.cursor.Col = 0, 0
		}
	case 'K':
		if len(params) > 0 && params[0] == 2 {
			for j := range r.grid[r.cursor.Row] {
				r.grid[r.cursor.Row][j] = Cell{Rune: ' ', Style: Style{}}
			}
		}
	case 'm':
		if len(params) == 1 && params[0] == 0 {
		}
	case 'A':
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		r.cursor.Row -= n
		if r.cursor.Row < 0 {
			r.cursor.Row = 0
		}
	case 'B':
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		r.cursor.Row += n
		if r.cursor.Row >= r.rows {
			r.cursor.Row = r.rows - 1
		}
	case 'C':
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		r.cursor.Col += n
		if r.cursor.Col >= r.cols {
			r.cursor.Col = r.cols - 1
		}
	case 'D':
		n := 1
		if len(params) > 0 {
			n = params[0]
		}
		r.cursor.Col -= n
		if r.cursor.Col < 0 {
			r.cursor.Col = 0
		}
	case '?':
	}
}

func (r *Renderer) scrollUp(n int) {
	copy(r.grid, r.grid[n:])
	for i := r.rows - n; i < r.rows; i++ {
		for j := range r.grid[i] {
			r.grid[i][j] = Cell{Rune: ' ', Style: Style{}}
		}
	}
}

func (r *Renderer) Render() {
	var buf []byte
	buf = append(buf, "\x1b[?25l"...)
	buf = append(buf, "\x1b[H"...)
	
	for row := 0; row < r.rows; row++ {
		line := ""
		for col := 0; col < r.cols; col++ {
			cell := r.grid[row][col]
			if cell.Rune == 0 {
				line += " "
			} else {
				line += string(cell.Rune)
			}
		}
		buf = append(buf, line...)
		if row < r.rows-1 {
			buf = append(buf, "\r\n"...)
		}
	}
	
	buf = append(buf, fmt.Sprintf("\x1b[%d;%dH", r.cursor.Row+1, r.cursor.Col+1)...)
	buf = append(buf, "\x1b[?25h"...)
	os.Stdout.Write(buf)
}

func EnableRawMode() (*termios, error) {
	var old termios
	fd := os.Stdin.Fd()
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0); errno != 0 {
		return nil, errno
	}
	
	newState := old
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0
	
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&newState)), 0, 0, 0); errno != 0 {
		return nil, errno
	}
	
	return &old, nil
}

func DisableRawMode(old *termios) error {
	fd := os.Stdin.Fd()
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(old)), 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

type termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Cc     [32]uint8
	Ispeed uint32
	Ospeed uint32
}
