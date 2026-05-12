package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	sseClient  *http.Client // no timeout for long-lived SSE stream
}

type Session struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Directory string `json:"directory"`
}

type MessagePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type MessageResponse struct {
	Info  MessageInfo   `json:"info"`
	Parts []MessagePart `json:"parts"`
}

type MessageInfo struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	SessionID string `json:"sessionID"`
}

type SSEEvent struct {
	Directory string      `json:"directory"`
	Project   string      `json:"project"`
	Payload   SSEPayload  `json:"payload"`
}

type SSEPayload struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

type MessageUpdatedProperties struct {
	SessionID string      `json:"sessionID"`
	Info      MessageInfo `json:"info"`
}

type MessagePartUpdatedProperties struct {
	SessionID string      `json:"sessionID"`
	Part      MessagePart `json:"part"`
	Time      int64       `json:"time"`
}

type SessionStatusProperties struct {
	SessionID string `json:"sessionID"`
	Status    struct {
		Type string `json:"type"`
	} `json:"status"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sseClient:  &http.Client{}, // no timeout for long-lived SSE stream
	}
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/global/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}
	return nil
}

type createSessionBody struct {
	Title string `json:"title"`
}

func (c *Client) CreateSession(ctx context.Context, title string) (string, error) {
	body := createSessionBody{Title: title}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/session", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session: %s - %s", resp.Status, strings.TrimSpace(string(respData)))
	}
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("decode session: %w", err)
	}
	return session.ID, nil
}

type messageBody struct {
	Parts []MessagePart `json:"parts"`
}

func (c *Client) SendMessage(ctx context.Context, sessionID string, content string) error {
	return c.sendPrompt(ctx, sessionID, content, "/prompt_async")
}

func (c *Client) SendMessageSync(ctx context.Context, sessionID string, content string) (*MessageResponse, error) {
	body := messageBody{Parts: []MessagePart{{Type: "text", Text: content}}}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/session/"+sessionID+"/message", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send message: %s - %s", resp.Status, strings.TrimSpace(string(respData)))
	}
	var msgResp MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &msgResp, nil
}

func (c *Client) sendPrompt(ctx context.Context, sessionID, content, endpoint string) error {
	body := messageBody{Parts: []MessagePart{{Type: "text", Text: content}}}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/session/"+sessionID+endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send message: %s - %s", resp.Status, strings.TrimSpace(string(respData)))
	}
	return nil
}

func (c *Client) AbortSession(ctx context.Context, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/session/"+sessionID+"/abort", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("abort session: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]MessageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/session/"+sessionID+"/message", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get messages: %s", resp.Status)
	}
	var msgs []MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	return msgs, nil
}

type SSEParser struct {
	decoder *json.Decoder
	reader  io.ReadCloser
}

func NewSSEParser(r io.ReadCloser) *SSEParser {
	return &SSEParser{
		decoder: json.NewDecoder(r),
		reader:  r,
	}
}

func (p *SSEParser) Next() (*SSEEvent, error) {
	for {
		line, err := p.readLine()
		if err != nil {
			return nil, err
		}
		if len(line) < 6 || string(line[:5]) != "data:" {
			continue
		}
		data := bytes.TrimSpace(line[5:])
		if len(data) == 0 {
			continue
		}
		var event SSEEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}
		return &event, nil
	}
}

func (p *SSEParser) readLine() ([]byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 1)
	for {
		_, err := p.reader.Read(tmp)
		if err != nil {
			return nil, err
		}
		if tmp[0] == '\n' {
			break
		}
		buf.WriteByte(tmp[0])
	}
	return buf.Bytes(), nil
}

func (p *SSEParser) Close() error {
	return p.reader.Close()
}

type EventHandler func(event SSEEvent)

func (c *Client) SubscribeSSE(ctx context.Context, handler EventHandler) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/global/event", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("sse connect: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("sse: %s", resp.Status)
	}

	parser := NewSSEParser(resp.Body)
	defer parser.Close()

	for {
		event, err := parser.Next()
		if err != nil {
			return err
		}
		handler(*event)
	}
}
