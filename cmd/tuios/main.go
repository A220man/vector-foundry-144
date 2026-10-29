package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"vector-foundry-144/internal/client"
	"vector-foundry-144/internal/daemon"
)

const (
	defaultSocketDir = "/tmp/tuios"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	uid := os.Getuid()
	socketDir := filepath.Join(defaultSocketDir, fmt.Sprintf("%d", uid))
	os.MkdirAll(socketDir, 0700)
	socketPath := filepath.Join(socketDir, "tuios.sock")

	switch command {
	case "daemon":
		runDaemon(socketPath)
	case "new":
		runNew(args, socketPath)
	case "ls", "list":
		runList(socketPath)
	case "attach":
		runAttach(args, socketPath)
	case "kill-server":
		runKillServer(socketPath)
	case "version":
		fmt.Println("vector-foundry-144 0.1.0")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: tuios <command> [args...]

Commands:
  daemon         Start the daemon (usually auto-started)
  new [name]     Create a new session
  ls, list       List all sessions
  attach [id]    Attach to a session (default: most recent)
  kill-server    Stop the daemon
  version        Show version`)
}

func ensureDaemonRunning(socketPath string) error {
	// Check if daemon is already running
	c := client.NewClient(socketPath)
	if err := c.Connect(); err == nil {
		c.Close()
		return nil
	}

	// Start daemon as background process
	cmd := exec.Command(os.Args[0], "daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "TUIOS_SOCKET="+socketPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Don't wait - let it run in background
	go func() {
		cmd.Wait()
	}()

	// Wait for daemon to be ready
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		c := client.NewClient(socketPath)
		if err := c.Connect(); err == nil {
			c.Close()
			return nil
		}
	}
	return fmt.Errorf("daemon failed to start")
}

func runDaemon(socketPath string) {
	d := daemon.NewDaemon(socketPath)
	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Daemon started on %s\n", socketPath)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Fprintf(os.Stderr, "\nShutting down...\n")
	d.Stop()
}

func runNew(args []string, socketPath string) {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	name := fs.String("name", "", "Session name")
	space := fs.String("space", "", "Space label")
	fs.Parse(args)

	if err := ensureDaemonRunning(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	c := client.NewClient(socketPath)
	if err := c.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	sessionID, err := c.NewSession(*name, *space)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(sessionID)
}

func runList(socketPath string) {
	if err := ensureDaemonRunning(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "No daemon running\n")
		os.Exit(1)
	}

	c := client.NewClient(socketPath)
	if err := c.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	sessions, err := c.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions")
		return
	}

	for _, s := range sessions {
		spaceInfo := ""
		if s.Space != "" {
			spaceInfo = fmt.Sprintf(" [space: %s]", s.Space)
		}
		fmt.Printf("%s  %s%s  (ws:%d, windows:%d, attached:%d)\n",
			s.ID, s.Name, spaceInfo, s.ActiveWS, s.WindowCount, s.Attached)
	}
}

func runAttach(args []string, socketPath string) {
	fs := flag.NewFlagSet("attach", flag.ExitOnError)
	sessionID := fs.String("s", "", "Session ID")
	windowID := fs.String("w", "", "Window ID")
	fs.Parse(args)

	if err := ensureDaemonRunning(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "No daemon running. Start with 'tuios new' first.\n")
		os.Exit(1)
	}

	c := client.NewClient(socketPath)
	if err := c.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	targetSession := *sessionID
	if targetSession == "" {
		sessions, err := c.ListSessions()
		if err != nil || len(sessions) == 0 {
			fmt.Fprintf(os.Stderr, "No sessions available\n")
			os.Exit(1)
		}
		targetSession = sessions[0].ID
	}

	ws, err := getWinsize()
	if err != nil {
		ws.Cols, ws.Rows = 80, 24
	}

	if err := c.Attach(targetSession, *windowID, int(ws.Cols), int(ws.Rows)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to attach: %v\n", err)
		os.Exit(1)
	}
}

func runKillServer(socketPath string) {
	c := client.NewClient(socketPath)
	if err := c.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "No daemon running\n")
		return
	}
	defer c.Close()

	// Send a signal to the daemon process to stop
	pidPath := socketPath + ".pid"
	if data, err := os.ReadFile(pidPath); err == nil {
		var pid int
		fmt.Sscanf(string(data), "%d", &pid)
		if pid > 0 {
			proc, err := os.FindProcess(pid)
			if err == nil {
				proc.Signal(syscall.SIGTERM)
			}
		}
	}
	
	// Also clean up socket files
	os.Remove(socketPath)
	os.Remove(socketPath + ".pid")
	fmt.Println("Daemon stopped")
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
