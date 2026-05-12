package opencode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"time"
)

type ServerManager struct {
	worktreePath string
	port         int
	cmd          *exec.Cmd
	client       *Client
	done         chan struct{}
	stderr       io.ReadCloser
}

func NewServerManager(worktreePath string, port int) *ServerManager {
	return &ServerManager{
		worktreePath: worktreePath,
		port:         port,
		done:         make(chan struct{}),
	}
}

func FindAvailablePort(start int) int {
	for port := start; port < start+100; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port
		}
	}
	return start
}

func (m *ServerManager) Start(ctx context.Context) error {
	m.cmd = exec.Command("opencode", "serve", "--port", fmt.Sprintf("%d", m.port), "--hostname", "127.0.0.1")
	m.cmd.Dir = m.worktreePath

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	m.stderr = stderr

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start opencode serve: %w", err)
	}

	log.Printf("[opencode-server] starting on port %d in %s (pid=%d)", m.port, m.worktreePath, m.cmd.Process.Pid)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[opencode-server:%d] %s", m.port, line)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Printf("[opencode-server:%d] %s", m.port, scanner.Text())
		}
	}()

	go func() {
		err := m.cmd.Wait()
		log.Printf("[opencode-server:%d] exited: %v", m.port, err)
		close(m.done)
	}()

	// Wait for server to be ready (up to 30s for first-time startup)
	deadline := time.Now().Add(30 * time.Second)
	m.client = NewClient(fmt.Sprintf("http://127.0.0.1:%d", m.port))

	for time.Now().Before(deadline) {
		select {
		case <-m.done:
			return fmt.Errorf("opencode server exited prematurely on port %d", m.port)
		default:
		}
		healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := m.client.Health(healthCtx)
		cancel()
		if err == nil {
			log.Printf("[opencode-server:%d] ready", m.port)
			return nil
		}
		if m.port <= 14105 {
			log.Printf("[opencode-server:%d] waiting... (%v)", m.port, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Check if process is still alive
	if !m.IsRunning() {
		return fmt.Errorf("opencode server exited during startup on port %d", m.port)
	}
	return fmt.Errorf("timeout waiting for opencode server on port %d", m.port)
}

func (m *ServerManager) Stop() error {
	if m.cmd != nil && m.cmd.Process != nil {
		log.Printf("[opencode-server:%d] stopping", m.port)
		if err := m.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("kill opencode server: %w", err)
		}
	}
	select {
	case <-m.done:
	case <-time.After(5 * time.Second):
		log.Printf("[opencode-server:%d] force killed", m.port)
	}
	return nil
}

func (m *ServerManager) Client() *Client {
	return m.client
}

func (m *ServerManager) Port() int {
	return m.port
}

func (m *ServerManager) Done() <-chan struct{} {
	return m.done
}

func (m *ServerManager) WorktreePath() string {
	return m.worktreePath
}

func (m *ServerManager) IsRunning() bool {
	select {
	case <-m.done:
		return false
	default:
		return true
	}
}


