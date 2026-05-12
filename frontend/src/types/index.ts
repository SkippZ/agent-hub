export type SessionStatus = 'running' | 'needs_attention' | 'done'
export type AgentType = 'opencode' | 'claude-code'

export interface Session {
  id: string
  agent_type: AgentType
  project_path: string
  project_name: string
  base_branch: string
  feature_branch: string
  worktree_path: string
  task_description: string
  status: SessionStatus
  created_at: string
  updated_at: string
  exited_at?: string
}

export interface Message {
  id: string
  session_id: string
  role: 'user' | 'agent'
  content: string
  created_at: string
}

export interface CodeSnapshot {
  id: string
  session_id: string
  commit_hash?: string
  diff: string
  summary?: string
  created_at: string
}

export interface CreateSessionRequest {
  agent_type: AgentType
  project_name: string
  base_branch: string
  task_description: string
}

export interface Project {
  name: string
  path: string
}

export interface Branch {
  name: string
}

export interface Config {
  projects_dir: string
  agents: Record<string, string>
}
