package types

import "time"

type SessionStatus string

const (
	StatusRunning       SessionStatus = "running"
	StatusNeedsAttention SessionStatus = "needs_attention"
	StatusDone          SessionStatus = "done"
)

type AgentType string

const (
	AgentOpenCode   AgentType = "opencode"
	AgentClaudeCode AgentType = "claude-code"
)

type Session struct {
	ID              string        `json:"id"`
	AgentType       AgentType     `json:"agent_type"`
	ProjectPath     string        `json:"project_path"`
	ProjectName     string        `json:"project_name"`
	BaseBranch      string        `json:"base_branch"`
	FeatureBranch   string        `json:"feature_branch"`
	WorktreePath    string        `json:"worktree_path"`
	TaskDescription string        `json:"task_description"`
	Status          SessionStatus `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	ExitedAt        *time.Time    `json:"exited_at,omitempty"`
}

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type CodeSnapshot struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	CommitHash string   `json:"commit_hash,omitempty"`
	Diff      string    `json:"diff"`
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateSessionRequest struct {
	AgentType       AgentType `json:"agent_type"`
	ProjectName     string    `json:"project_name"`
	BaseBranch      string    `json:"base_branch"`
	TaskDescription string    `json:"task_description"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
}

type UpdateStatusRequest struct {
	Status SessionStatus `json:"status"`
}

type Config struct {
	ProjectsDir string           `json:"projects_dir"`
	Agents      map[string]string `json:"agents"`
}

type Project struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Branch struct {
	Name string `json:"name"`
}
