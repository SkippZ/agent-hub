package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ProcessState string

const (
	StateRunning        ProcessState = "running"
	StateNeedsAttention ProcessState = "needs_attention"
	StateExited         ProcessState = "exited"
)

type ProcessEvent struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type Manager struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	state    ProcessState
	stateMu  sync.RWMutex
	events   chan ProcessEvent
	done     chan struct{}
	cancel   context.CancelFunc
}

func NewManager(command string, args ...string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, command, args...)
	return &Manager{
		cmd:    cmd,
		cancel: cancel,
		events: make(chan ProcessEvent, 100),
		done:   make(chan struct{}),
	}
}

func (m *Manager) SetDir(dir string) {
	m.cmd.Dir = dir
}

func (m *Manager) Start() error {
	stdin, err := m.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	m.stdin = stdin

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	m.stdout = stdout

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	m.stderr = stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	m.state = StateRunning
	go m.readOutput()
	go m.readError()
	go m.wait()
	return nil
}

func (m *Manager) SendMessage(msg string) error {
	if m.state == StateExited {
		return fmt.Errorf("process already exited")
	}
	m.setState(StateRunning)
	_, err := io.WriteString(m.stdin, msg+"\n")
	return err
}

func (m *Manager) Stop() error {
	m.cancel()
	return nil
}

func (m *Manager) Events() <-chan ProcessEvent {
	return m.events
}

func (m *Manager) State() ProcessState {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

func (m *Manager) Done() <-chan struct{} {
	return m.done
}

func (m *Manager) readOutput() {
	reader := bufio.NewReader(m.stdout)
	var buffer strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				m.events <- ProcessEvent{Type: "error", Message: err.Error()}
			}
			break
		}
		m.events <- ProcessEvent{Type: "output", Data: line}
		buffer.WriteString(line)
	}
	// After output stream ends, check if process is waiting
	final := buffer.String()
	if m.state != StateExited && isWaitingForInput(final) {
		m.setState(StateNeedsAttention)
		m.events <- ProcessEvent{Type: "status", Status: string(StateNeedsAttention)}
	}
}

func (m *Manager) readError() {
	reader := bufio.NewReader(m.stderr)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				m.events <- ProcessEvent{Type: "error", Message: err.Error()}
			}
			break
		}
		m.events <- ProcessEvent{Type: "output", Data: line}
	}
}

func (m *Manager) wait() {
	err := m.cmd.Wait()
	m.setState(StateExited)
	m.events <- ProcessEvent{Type: "status", Status: string(StateExited)}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			m.events <- ProcessEvent{Type: "error", Message: fmt.Sprintf("exited with code %d", exitErr.ExitCode())}
		}
	}
	close(m.events)
	close(m.done)
}

func (m *Manager) setState(s ProcessState) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.state = s
}

// isWaitingForInput detects when the agent has finished a response and is
// waiting for user input. Looks for trailing prompt patterns.
func isWaitingForInput(output string) bool {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return false
	}
	// Common prompt patterns from CLI agents
	promptPatterns := []string{
		"\n>",
		"\n> ",
	}
	lastNewline := strings.LastIndex(trimmed, "\n")
	if lastNewline >= 0 {
		after := trimmed[lastNewline:]
		for _, p := range promptPatterns {
			// The prompt character might be at the very end
			if strings.Contains(after, p) || strings.HasSuffix(trimmed, p) {
				return true
			}
		}
	}
	// If no output for a while after a complete block, might be waiting
	return false
}

// StripANSI removes ANSI escape sequences from a string.
func StripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip until 'm' (end of ANSI sequence)
			i++
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// DefaultPromptTimeout is how long to wait after last output before
// considering the agent as waiting for input (fallback detection).
const DefaultPromptTimeout = 5 * time.Second
