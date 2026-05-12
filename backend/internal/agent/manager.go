package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
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

	log.Printf("[agent %s] started (pid=%d)", m.cmd.Path, m.cmd.Process.Pid)
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
	n, err := io.WriteString(m.stdin, msg+"\n")
	log.Printf("[agent] sent %d bytes to stdin", n)
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
	lineCount := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[agent stdout] read error: %v", err)
				m.events <- ProcessEvent{Type: "error", Message: err.Error()}
			}
			break
		}
		lineCount++
		if lineCount <= 3 || lineCount%100 == 0 {
			log.Printf("[agent stdout] line %d: %q", lineCount, truncate(line, 80))
		}
		m.events <- ProcessEvent{Type: "output", Data: line}
		buffer.WriteString(line)
	}
	log.Printf("[agent stdout] closed after %d lines (%d bytes)", lineCount, buffer.Len())
	final := buffer.String()
	if m.state != StateExited && isWaitingForInput(final) {
		m.setState(StateNeedsAttention)
		m.events <- ProcessEvent{Type: "status", Status: string(StateNeedsAttention)}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (m *Manager) readError() {
	reader := bufio.NewReader(m.stderr)
	lineCount := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[agent stderr] read error: %v", err)
				m.events <- ProcessEvent{Type: "error", Message: err.Error()}
			}
			break
		}
		lineCount++
		if lineCount <= 3 || lineCount%100 == 0 {
			log.Printf("[agent stderr] line %d: %q", lineCount, truncate(line, 80))
		}
		m.events <- ProcessEvent{Type: "output", Data: line}
	}
	log.Printf("[agent stderr] closed after %d lines", lineCount)
}

func (m *Manager) wait() {
	err := m.cmd.Wait()
	log.Printf("[agent] process exited: %v", err)
	m.setState(StateExited)
	m.events <- ProcessEvent{Type: "status", Status: string(StateExited)}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("[agent] exit code: %d", exitErr.ExitCode())
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
			i++
			if i >= len(s) {
				break
			}
			// ESC [ (CSI): parameter bytes 0x30-0x3F, intermediate 0x20-0x2F, final 0x40-0x7E
			if s[i] == '[' {
				i++
				for i < len(s) && (s[i] >= 0x30 && s[i] <= 0x3F || s[i] >= 0x20 && s[i] <= 0x2F) {
					i++
				}
				if i < len(s) {
					i++ // skip final byte
				}
			} else if s[i] == ']' {
				// OSC sequence: terminated by BEL or ESC \
				i++
				for i < len(s) && s[i] != '\x07' && !(s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\') {
					i++
				}
				if i < len(s) && s[i] == '\x1b' {
					i += 2 // skip ESC + backslash
				} else if i < len(s) {
					i++ // skip BEL
				}
			} else {
				// Two-character escape sequence (e.g. ESC c, ESC (, ESC ))
				if i < len(s) {
					i++
				}
			}
		} else if s[i] == '\r' {
			// Replace carriage returns with newlines
			result.WriteByte('\n')
			i++
		} else if s[i] == '\x07' {
			// Strip BEL characters
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
