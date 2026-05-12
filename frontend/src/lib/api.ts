import type { Session, Message, CodeSnapshot, CreateSessionRequest, SessionStatus, Project, Branch, Config } from '../types'

const BASE = ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export const api = {
  health: () => request<{ status: string }>('/api/health'),

  listSessions: () => request<Session[]>('/api/sessions'),

  getSession: (id: string) => request<Session>(`/api/sessions/${id}`),

  createSession: (data: CreateSessionRequest) =>
    request<Session>('/api/sessions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateSessionStatus: (id: string, status: SessionStatus) =>
    request<Session>(`/api/sessions/${id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    }),

  stopSession: (id: string) =>
    request<void>(`/api/sessions/${id}/stop`, { method: 'POST' }),

  sendMessage: (id: string, content: string) =>
    request<Message>(`/api/sessions/${id}/message`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    }),

  getMessages: (id: string) =>
    request<Message[]>(`/api/sessions/${id}/messages`),

  getChanges: (id: string) =>
    request<CodeSnapshot[]>(`/api/sessions/${id}/changes`),

  listProjects: () => request<Project[]>('/api/projects'),

  listBranches: (name: string) =>
    request<Branch[]>(`/api/projects/${encodeURIComponent(name)}/branches`),

  getConfig: () => request<Config>('/api/config'),

  updateConfig: (cfg: Config) =>
    request<Config>('/api/config', {
      method: 'PUT',
      body: JSON.stringify(cfg),
    }),
}
