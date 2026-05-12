package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"agent-hub/internal/agent"
	"agent-hub/internal/config"
	"agent-hub/internal/db"
	"agent-hub/internal/git"
	"agent-hub/internal/types"
)

type Handler struct {
	store      *db.Store
	cfg        *types.Config
	wsHub      *WSHub
	agents     map[string]*agent.Manager
	mu         sync.Mutex
	fileServer http.Handler
}

func New(store *db.Store, cfg *types.Config) *Handler {
	return &Handler{
		store:  store,
		cfg:    cfg,
		wsHub:  NewWSHub(),
		agents: make(map[string]*agent.Manager),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", h.handleHealth)

	mux.HandleFunc("GET /api/sessions", h.handleListSessions)
	mux.HandleFunc("POST /api/sessions", h.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", h.handleGetSession)
	mux.HandleFunc("PATCH /api/sessions/{id}/status", h.handleUpdateStatus)
	mux.HandleFunc("POST /api/sessions/{id}/message", h.handleSendMessage)
	mux.HandleFunc("GET /api/sessions/{id}/messages", h.handleGetMessages)
	mux.HandleFunc("GET /api/sessions/{id}/changes", h.handleGetChanges)
	mux.HandleFunc("POST /api/sessions/{id}/stop", h.handleStopSession)

	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.HandleFunc("GET /api/projects/{name}/branches", h.handleListBranches)

	mux.HandleFunc("GET /api/config", h.handleGetConfig)
	mux.HandleFunc("PUT /api/config", h.handleUpdateConfig)

	mux.HandleFunc("GET /ws/sessions/{id}", h.handleWebSocket)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.store.ListSessions()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sessions == nil {
		sessions = []*types.Session{}
	}
	respondJSON(w, http.StatusOK, sessions)
}

func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req types.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AgentType == "" || req.ProjectName == "" || req.BaseBranch == "" || req.TaskDescription == "" {
		respondError(w, http.StatusBadRequest, "agent_type, project_name, base_branch, and task_description are required")
		return
	}

	projectPath := filepath.Join(h.cfg.ProjectsDir, req.ProjectName)
	if info, err := os.Stat(projectPath); err != nil || !info.IsDir() {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("project not found: %s", req.ProjectName))
		return
	}

	agentCmd, ok := h.cfg.Agents[string(req.AgentType)]
	if !ok {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown agent type: %s", req.AgentType))
		return
	}

	featureBranch := git.BranchNameFromDescription(req.TaskDescription)
	worktreePath, err := git.CreateWorktree(projectPath, req.BaseBranch, featureBranch)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("create worktree: %v", err))
		return
	}

	sessionID := uuid.New().String()
	now := time.Now()
	session := &types.Session{
		ID:              sessionID,
		AgentType:       req.AgentType,
		ProjectPath:     projectPath,
		ProjectName:     req.ProjectName,
		BaseBranch:      req.BaseBranch,
		FeatureBranch:   featureBranch,
		WorktreePath:    worktreePath,
		TaskDescription: req.TaskDescription,
		Status:          types.StatusRunning,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.store.CreateSession(session); err != nil {
		git.RemoveWorktree(projectPath, worktreePath)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("create session: %v", err))
		return
	}

	// Store initial user message
	initialMsg := &types.Message{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "user",
		Content:   req.TaskDescription,
		CreatedAt: now,
	}
	h.store.AddMessage(initialMsg)

	// Start agent process
	mgr := agent.NewManager(agentCmd)
	mgr.SetDir(worktreePath)
	if err := mgr.Start(); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("start agent: %v", err))
		return
	}

	h.mu.Lock()
	h.agents[sessionID] = mgr
	h.mu.Unlock()

	// Relay agent events to WebSocket
	go h.relayAgentEvents(sessionID, mgr)

	// Send initial task to agent
	if err := mgr.SendMessage(req.TaskDescription); err != nil {
		log.Printf("send initial message: %v", err)
	}

	respondJSON(w, http.StatusCreated, session)
}

func (h *Handler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, err := h.store.GetSession(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (h *Handler) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req types.UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status == types.StatusDone {
		h.mu.Lock()
		if mgr, ok := h.agents[id]; ok {
			mgr.Stop()
			delete(h.agents, id)
		}
		h.mu.Unlock()
		h.store.SetSessionExited(id)
	} else {
		h.store.UpdateSessionStatus(id, req.Status)
	}

	session, _ := h.store.GetSession(id)
	respondJSON(w, http.StatusOK, session)
}

func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req types.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg := &types.Message{
		ID:        uuid.New().String(),
		SessionID: id,
		Role:      "user",
		Content:   req.Content,
		CreatedAt: time.Now(),
	}
	h.store.AddMessage(msg)
	h.store.UpdateSessionStatus(id, types.StatusRunning)

	h.mu.Lock()
	mgr, ok := h.agents[id]
	h.mu.Unlock()

	if ok {
		if err := mgr.SendMessage(req.Content); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("send to agent: %v", err))
			return
		}
	}

	respondJSON(w, http.StatusCreated, msg)
}

func (h *Handler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	messages, err := h.store.GetMessages(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []*types.Message{}
	}
	respondJSON(w, http.StatusOK, messages)
}

func (h *Handler) handleGetChanges(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snaps, err := h.store.GetCodeSnapshots(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if snaps == nil {
		snaps = []*types.CodeSnapshot{}
	}
	respondJSON(w, http.StatusOK, snaps)
}

func (h *Handler) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	h.mu.Lock()
	mgr, ok := h.agents[id]
	if ok {
		mgr.Stop()
		delete(h.agents, id)
	}
	h.mu.Unlock()

	h.store.SetSessionExited(id)
	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *Handler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := git.ListProjects(h.cfg.ProjectsDir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []*types.Project{}
	}
	respondJSON(w, http.StatusOK, projects)
}

func (h *Handler) handleListBranches(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projectPath := filepath.Join(h.cfg.ProjectsDir, name)
	branches, err := git.ListBranches(projectPath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if branches == nil {
		branches = []*types.Branch{}
	}
	respondJSON(w, http.StatusOK, branches)
}

func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.cfg)
}

func (h *Handler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg types.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := config.Save(config.ConfigPath(), &cfg); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.cfg = &cfg
	respondJSON(w, http.StatusOK, &cfg)
}

func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Verify session exists
	_, err := h.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	h.wsHub.HandleWS(w, r, id)
}

// Snapshot code changes for a session
func (h *Handler) SnapshotChanges(sessionID string) {
	session, err := h.store.GetSession(sessionID)
	if err != nil {
		return
	}
	diff, err := git.GetDiff(session.WorktreePath, session.BaseBranch)
	if err != nil || diff == "" {
		return
	}

	// Get last commit message as summary
	commits, _ := git.GetRecentCommits(session.WorktreePath, session.BaseBranch, 1)
	summary := strings.TrimSpace(commits)

	snapshot := &types.CodeSnapshot{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Diff:      diff,
		Summary:   summary,
		CreatedAt: time.Now(),
	}
	h.store.AddCodeSnapshot(snapshot)
}

func (h *Handler) relayAgentEvents(sessionID string, mgr *agent.Manager) {
	for event := range mgr.Events() {
		switch event.Type {
		case "output":
			clean := agent.StripANSI(event.Data)
			h.wsHub.Broadcast(sessionID, WSMessage{
				Type: "output",
				Data: clean,
			})
			// Store agent message in chunks (simple approach: buffer per output line)
			h.store.AddMessage(&types.Message{
				ID:        uuid.New().String(),
				SessionID: sessionID,
				Role:      "agent",
				Content:   clean,
				CreatedAt: time.Now(),
			})
			// Snapshot changes periodically
			if strings.Contains(clean, "git commit") || strings.Contains(clean, "committed") {
				h.SnapshotChanges(sessionID)
			}

		case "status":
			h.wsHub.Broadcast(sessionID, WSMessage{
				Type:   "status",
				Status: event.Status,
			})
			if event.Status == string(agent.StateNeedsAttention) {
				h.store.UpdateSessionStatus(sessionID, types.StatusNeedsAttention)
			} else if event.Status == string(agent.StateExited) {
				h.store.SetSessionExited(sessionID)
				h.mu.Lock()
				delete(h.agents, sessionID)
				h.mu.Unlock()
				// Final snapshot
				h.SnapshotChanges(sessionID)
			}

		case "error":
			h.wsHub.Broadcast(sessionID, WSMessage{
				Type:    "error",
				Message: event.Message,
			})
		}
	}
}

func (h *Handler) RegisterFileServer(fs http.Handler) {
	h.fileServer = fs
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
