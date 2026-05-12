package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"agent-hub/internal/types"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id            TEXT PRIMARY KEY,
		agent_type    TEXT NOT NULL,
		project_path  TEXT NOT NULL,
		project_name  TEXT NOT NULL,
		base_branch   TEXT NOT NULL,
		feature_branch TEXT NOT NULL,
		worktree_path TEXT NOT NULL,
		task_description TEXT NOT NULL,
		status        TEXT NOT NULL DEFAULT 'running',
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		exited_at     DATETIME
	);

	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id),
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS code_snapshots (
		id         TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id),
		commit_hash TEXT,
		diff       TEXT NOT NULL,
		summary    TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_snapshots_session ON code_snapshots(session_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Add external_session_id column if not present (migration for existing DBs)
	_, _ = s.db.Exec(`ALTER TABLE sessions ADD COLUMN external_session_id TEXT`)

	return nil
}

func (s *Store) CreateSession(session *types.Session) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, agent_type, project_path, project_name, base_branch, feature_branch, worktree_path, task_description, status, created_at, updated_at, external_session_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.AgentType, session.ProjectPath, session.ProjectName,
		session.BaseBranch, session.FeatureBranch, session.WorktreePath,
		session.TaskDescription, session.Status, session.CreatedAt, session.UpdatedAt,
		session.ExternalSessionID,
	)
	return err
}

func (s *Store) SetExternalSessionID(id, externalID string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET external_session_id = ? WHERE id = ?`,
		externalID, id,
	)
	return err
}

func (s *Store) ListSessions() ([]*types.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, agent_type, project_path, project_name, base_branch, feature_branch,
		        worktree_path, task_description, status, created_at, updated_at, exited_at,
		        external_session_id
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*types.Session
	for rows.Next() {
		s := &types.Session{}
		var exitedAt sql.NullTime
		var extID sql.NullString
		if err := rows.Scan(
			&s.ID, &s.AgentType, &s.ProjectPath, &s.ProjectName,
			&s.BaseBranch, &s.FeatureBranch, &s.WorktreePath,
			&s.TaskDescription, &s.Status, &s.CreatedAt, &s.UpdatedAt, &exitedAt,
			&extID,
		); err != nil {
			return nil, err
		}
		if exitedAt.Valid {
			s.ExitedAt = &exitedAt.Time
		}
		if extID.Valid {
			s.ExternalSessionID = extID.String
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (s *Store) GetSession(id string) (*types.Session, error) {
	sess := &types.Session{}
	var exitedAt sql.NullTime
	var extID sql.NullString
	err := s.db.QueryRow(
		`SELECT id, agent_type, project_path, project_name, base_branch, feature_branch,
		        worktree_path, task_description, status, created_at, updated_at, exited_at,
		        external_session_id
		 FROM sessions WHERE id = ?`, id,
	).Scan(
		&sess.ID, &sess.AgentType, &sess.ProjectPath, &sess.ProjectName,
		&sess.BaseBranch, &sess.FeatureBranch, &sess.WorktreePath,
		&sess.TaskDescription, &sess.Status, &sess.CreatedAt, &sess.UpdatedAt, &exitedAt,
		&extID,
	)
	if err != nil {
		return nil, err
	}
	if exitedAt.Valid {
		sess.ExitedAt = &exitedAt.Time
	}
	if extID.Valid {
		sess.ExternalSessionID = extID.String
	}
	return sess, nil
}

func (s *Store) UpdateSessionStatus(id string, status types.SessionStatus) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (s *Store) SetSessionExited(id string) error {
	now := time.Now()
	_, err := s.db.Exec(
		`UPDATE sessions SET status = ?, updated_at = ?, exited_at = ? WHERE id = ?`,
		types.StatusDone, now, now, id,
	)
	return err
}

func (s *Store) AddMessage(msg *types.Message) error {
	_, err := s.db.Exec(
		`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt,
	)
	return err
}

func (s *Store) GetMessages(sessionID string) ([]*types.Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*types.Message
	for rows.Next() {
		m := &types.Message{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (s *Store) AddCodeSnapshot(snapshot *types.CodeSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO code_snapshots (id, session_id, commit_hash, diff, summary, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.SessionID, snapshot.CommitHash, snapshot.Diff, snapshot.Summary, snapshot.CreatedAt,
	)
	return err
}

func (s *Store) GetCodeSnapshots(sessionID string) ([]*types.CodeSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, commit_hash, diff, summary, created_at FROM code_snapshots WHERE session_id = ? ORDER BY created_at DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*types.CodeSnapshot
	for rows.Next() {
		cs := &types.CodeSnapshot{}
		var commitHash, summary sql.NullString
		if err := rows.Scan(&cs.ID, &cs.SessionID, &commitHash, &cs.Diff, &summary, &cs.CreatedAt); err != nil {
			return nil, err
		}
		if commitHash.Valid {
			cs.CommitHash = commitHash.String
		}
		if summary.Valid {
			cs.Summary = summary.String
		}
		snaps = append(snaps, cs)
	}
	return snaps, rows.Err()
}
