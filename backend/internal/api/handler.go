package api

import (
	"context"
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
	"agent-hub/internal/opencode"
	"agent-hub/internal/types"
)

type opencodeSessionState struct {
	serverMgr  *opencode.ServerManager
	ocClient   *opencode.Client
	ocSessionID string
	cancelSSE  context.CancelFunc
}

type Handler struct {
	store      *db.Store
	cfg        *types.Config
	wsHub      *WSHub
	agents     map[string]*agent.Manager
	opencodeSessions map[string]*opencodeSessionState
	usedPorts  map[int]bool
	mu         sync.Mutex
	fileServer http.Handler
}

func New(store *db.Store, cfg *types.Config) *Handler {
	return &Handler{
		store:            store,
		cfg:              cfg,
		wsHub:            NewWSHub(),
		agents:           make(map[string]*agent.Manager),
		opencodeSessions: make(map[string]*opencodeSessionState),
		usedPorts:        make(map[int]bool),
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
	search := r.URL.Query().Get("q")
	sessions, err := h.store.ListSessions(search)
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

	if req.AgentType == types.AgentOpenCode {
		h.startOpenCodeSession(session, worktreePath, req.TaskDescription)
	} else {
		// Start agent process via subprocess (Claude Code path)
		mgr := agent.NewManager(agentCmd)
		mgr.SetDir(worktreePath)
		if err := mgr.Start(); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("start agent: %v", err))
			return
		}
		log.Printf("[session %s] agent started (cmd=%s, worktree=%s)", sessionID, agentCmd, worktreePath)

		h.mu.Lock()
		h.agents[sessionID] = mgr
		h.mu.Unlock()

		go h.relayAgentEvents(sessionID, mgr)

		if err := mgr.SendMessage(req.TaskDescription); err != nil {
			log.Printf("[session %s] send initial message: %v", sessionID, err)
		} else {
			log.Printf("[session %s] initial message sent: %q", sessionID, req.TaskDescription)
		}
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
		_, ocOK := h.opencodeSessions[id]
		h.mu.Unlock()

		if ocOK {
			h.cleanupOpenCodeSession(id)
		}
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
	mgr, mgrOK := h.agents[id]
	ocState, ocOK := h.opencodeSessions[id]
	h.mu.Unlock()

	if mgrOK {
		if err := mgr.SendMessage(req.Content); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("send to agent: %v", err))
			return
		}
	} else if ocOK {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ocState.ocClient.SendMessage(ctx, ocState.ocSessionID, req.Content); err != nil {
			log.Printf("[session %s] send to opencode: %v", id, err)
		}
	} else {
		// No active agent — try reconnecting to an orphaned OpenCode session
		h.sendOrphanedOpenCodeMessage(id, req.Content)
	}

	respondJSON(w, http.StatusCreated, msg)
}

func (h *Handler) sendOrphanedOpenCodeMessage(sessionID, content string) {
	session, err := h.store.GetSession(sessionID)
	if err != nil {
		log.Printf("[orphan %s] get session: %v", sessionID, err)
		return
	}
	if session.AgentType != types.AgentOpenCode || session.ExternalSessionID == "" {
		log.Printf("[orphan %s] not an opencode session or no external id", sessionID)
		return
	}
	if session.Status == types.StatusDone {
		log.Printf("[orphan %s] session already done", sessionID)
		return
	}

	serverURL := h.opencodeServerURL()
	if serverURL == "" {
		log.Printf("[orphan %s] no opencode server URL configured", sessionID)
		return
	}

	client := opencode.NewClient(serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use sync API to send message and get response
	resp, err := client.SendMessageSync(ctx, session.ExternalSessionID, content)
	if err != nil {
		log.Printf("[orphan %s] send message: %v", sessionID, err)
		return
	}

	log.Printf("[orphan %s] got response for message %s", sessionID, resp.Info.ID)

	// Extract and store text parts
	for _, part := range resp.Parts {
		if part.Type == "text" && part.Text != "" {
			h.store.AddMessage(&types.Message{
				ID:        uuid.New().String(),
				SessionID: sessionID,
				Role:      "agent",
				Content:   part.Text,
				CreatedAt: time.Now(),
			})
			h.wsHub.Broadcast(sessionID, WSMessage{
				Type: "output",
				Data: part.Text,
			})
		}
	}

	h.store.UpdateSessionStatus(sessionID, types.StatusNeedsAttention)
	h.wsHub.Broadcast(sessionID, WSMessage{
		Type:   "status",
		Status: string(agent.StateNeedsAttention),
	})
	h.SnapshotChanges(sessionID)
}

func (h *Handler) opencodeServerURL() string {
	urlMap := ""
	h.mu.Lock()
	if h.cfg.OpenCodeServer != nil && h.cfg.OpenCodeServer.URL != "" {
		urlMap = h.cfg.OpenCodeServer.URL
	}
	h.mu.Unlock()
	return urlMap
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
	_, ocOK := h.opencodeSessions[id]
	h.mu.Unlock()

	if ocOK {
		h.cleanupOpenCodeSession(id)
	}

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
		log.Printf("[snapshot %s] get session: %v", sessionID, err)
		return
	}
	diff, err := git.GetDiff(session.WorktreePath, session.BaseBranch)
	if err != nil {
		log.Printf("[snapshot %s] get diff: %v", sessionID, err)
		return
	}
	if diff == "" {
		log.Printf("[snapshot %s] empty diff (worktree=%s, base=%s)", sessionID, session.WorktreePath, session.BaseBranch)
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
	log.Printf("[snapshot %s] captured %d bytes", sessionID, len(diff))
}

func (h *Handler) relayAgentEvents(sessionID string, mgr *agent.Manager) {
	log.Printf("[relay %s] started", sessionID)
	eventCount := 0
	for event := range mgr.Events() {
		eventCount++
		switch event.Type {
		case "output":
			clean := agent.StripANSI(event.Data)
			if eventCount <= 5 || eventCount%50 == 0 {
				log.Printf("[relay %s] output #%d: %q", sessionID, eventCount, truncate(clean, 100))
			}
			h.wsHub.Broadcast(sessionID, WSMessage{
				Type: "output",
				Data: clean,
			})
			if err := h.store.AddMessage(&types.Message{
				ID:        uuid.New().String(),
				SessionID: sessionID,
				Role:      "agent",
				Content:   clean,
				CreatedAt: time.Now(),
			}); err != nil {
				log.Printf("[relay %s] store message error: %v", sessionID, err)
			}
			if strings.Contains(clean, "git commit") || strings.Contains(clean, "committed") {
				log.Printf("[relay %s] triggering snapshot", sessionID)
				h.SnapshotChanges(sessionID)
			}

		case "status":
			log.Printf("[relay %s] status: %s", sessionID, event.Status)
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
				h.SnapshotChanges(sessionID)
			}

		case "error":
			log.Printf("[relay %s] error: %s", sessionID, event.Message)
			h.wsHub.Broadcast(sessionID, WSMessage{
				Type:    "error",
				Message: event.Message,
			})
		}
	}
	log.Printf("[relay %s] finished (%d events)", sessionID, eventCount)
}

func (h *Handler) startOpenCodeSession(session *types.Session, worktreePath, task string) {
	portRange := h.cfg.OpenCodeServer
	var basePort int
	if portRange != nil && portRange.PortRange[0] > 0 {
		basePort = portRange.PortRange[0]
	} else {
		basePort = 14100
	}

	sid := session.ID
	port := opencode.FindAvailablePort(basePort)
	serverMgr := opencode.NewServerManager(worktreePath, port)
	ctx, cancelSSE := context.WithCancel(context.Background())

	if err := serverMgr.Start(ctx); err != nil {
		log.Printf("[session %s] opencode server start failed: %v", sid, err)
		h.wsHub.Broadcast(sid, WSMessage{Type: "error", Message: fmt.Sprintf("opencode server: %v", err)})
		cancelSSE()
		h.store.UpdateSessionStatus(sid, types.StatusDone)
		return
	}

	h.mu.Lock()
	h.usedPorts[port] = true
	h.mu.Unlock()

	ocClient := serverMgr.Client()

	ocSessionID, err := ocClient.CreateSession(ctx, session.TaskDescription)
	if err != nil {
		log.Printf("[session %s] create opencode session: %v", sid, err)
		h.wsHub.Broadcast(sid, WSMessage{Type: "error", Message: fmt.Sprintf("create opencode session: %v", err)})
		serverMgr.Stop()
		cancelSSE()
		return
	}
	log.Printf("[session %s] opencode session created: %s", sid, ocSessionID)

	// Persist the external session ID so we can reconnect after restart
	if err := h.store.SetExternalSessionID(sid, ocSessionID); err != nil {
		log.Printf("[session %s] save external session id: %v", sid, err)
	}

	state := &opencodeSessionState{
		serverMgr:   serverMgr,
		ocClient:    ocClient,
		ocSessionID: ocSessionID,
		cancelSSE:   cancelSSE,
	}

	h.mu.Lock()
	h.opencodeSessions[sid] = state
	h.mu.Unlock()

	// Relay SSE events from opencode to our WS hub
	go h.relayOpenCodeEvents(ctx, sid, ocClient, ocSessionID)

	// Send initial task asynchronously
	if err := ocClient.SendMessage(ctx, ocSessionID, task); err != nil {
		log.Printf("[session %s] send initial message to opencode: %v", sid, err)
		h.wsHub.Broadcast(sid, WSMessage{Type: "error", Message: fmt.Sprintf("send message: %v", err)})
		h.cleanupOpenCodeSession(sid)
		return
	}
	log.Printf("[session %s] initial task sent to opencode session %s", sid, ocSessionID)
}

func (h *Handler) relayOpenCodeEvents(ctx context.Context, sessionID string, ocClient *opencode.Client, ocSessionID string) {
	log.Printf("[opencode-relay %s] started", sessionID)
	defer log.Printf("[opencode-relay %s] finished", sessionID)

	var pendingText strings.Builder
	pendingMsgID := ""

	storePending := func() {
		if pendingText.Len() == 0 {
			return
		}
		content := pendingText.String()
		h.store.AddMessage(&types.Message{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			Role:      "agent",
			Content:   content,
			CreatedAt: time.Now(),
		})
		h.wsHub.Broadcast(sessionID, WSMessage{
			Type: "output",
			Data: content,
		})
		pendingText.Reset()
		pendingMsgID = ""
	}

	for {
		err := ocClient.SubscribeSSE(ctx, func(event opencode.SSEEvent) {
			payload := event.Payload
			props := payload.Properties

			switch payload.Type {
			case "message.updated":
				info, _ := props["info"].(map[string]interface{})
				if info == nil {
					return
				}
				sid, _ := info["sessionID"].(string)
				if sid != ocSessionID {
					return
				}
				role, _ := info["role"].(string)
				if role == "user" && pendingMsgID != "" {
					storePending()
				}
				if role == "assistant" {
					msgID, _ := info["id"].(string)
					if msgID != "" && msgID != pendingMsgID {
						storePending()
						pendingMsgID = msgID
					}
				}

			case "message.part.updated":
				// sessionID is at properties level, not inside part
				sid, _ := props["sessionID"].(string)
				if sid != ocSessionID {
					return
				}
				part, _ := props["part"].(map[string]interface{})
				if part == nil {
					return
				}
				pType, _ := part["type"].(string)

				switch pType {
				case "text":
					text, _ := part["text"].(string)
					if text != "" {
						pendingText.WriteString(text)
						h.wsHub.Broadcast(sessionID, WSMessage{
							Type: "output",
							Data: text,
						})
					}
			case "reasoning":
				text, _ := part["text"].(string)
				if text != "" {
					h.wsHub.Broadcast(sessionID, WSMessage{
						Type: "reasoning",
						Data: text,
					})
				}
				case "step-finish":
					log.Printf("[opencode-relay %s] step-finish, snapshotting", sessionID)
					storePending()
					h.SnapshotChanges(sessionID)
				}

			case "session.status":
				sid, _ := props["sessionID"].(string)
				if sid != ocSessionID {
					return
				}
				status, _ := props["status"].(map[string]interface{})
				if status == nil {
					return
				}
				statusType, _ := status["type"].(string)
				if statusType == "idle" {
					log.Printf("[opencode-relay %s] idle, snapshotting", sessionID)
					storePending()
					h.SnapshotChanges(sessionID)
					h.wsHub.Broadcast(sessionID, WSMessage{
						Type:   "status",
						Status: string(agent.StateNeedsAttention),
					})
					h.store.UpdateSessionStatus(sessionID, types.StatusNeedsAttention)
				}
			}
		})
		if err == context.Canceled || err == context.DeadlineExceeded {
			return
		}
		if err != nil {
			log.Printf("[opencode-relay %s] SSE error: %v (reconnecting...)", sessionID, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func (h *Handler) Shutdown() {
	h.mu.Lock()
	ids := make([]string, 0, len(h.opencodeSessions))
	for id := range h.opencodeSessions {
		ids = append(ids, id)
	}
	h.mu.Unlock()

	for _, id := range ids {
		log.Printf("[handler] cleaning up opencode session %s", id)
		h.cleanupOpenCodeSession(id)
	}

	h.mu.Lock()
	for _, mgr := range h.agents {
		mgr.Stop()
	}
	h.agents = make(map[string]*agent.Manager)
	h.mu.Unlock()

	log.Printf("[handler] shutdown complete")
}

func (h *Handler) cleanupOpenCodeSession(sessionID string) {
	h.mu.Lock()
	state, ok := h.opencodeSessions[sessionID]
	if ok {
		delete(h.opencodeSessions, sessionID)
		if state.serverMgr != nil {
			port := state.serverMgr.Port()
			delete(h.usedPorts, port)
		}
	}
	h.mu.Unlock()

	if !ok {
		return
	}

	if state.cancelSSE != nil {
		state.cancelSSE()
	}
	if state.ocClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		state.ocClient.AbortSession(ctx, state.ocSessionID)
		cancel()
	}
	if state.serverMgr != nil {
		state.serverMgr.Stop()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
