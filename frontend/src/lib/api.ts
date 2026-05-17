import type { Session, Message, CodeSnapshot, CreateSessionRequest, SessionStatus, Project, Branch, Config } from '../types'

const BASE = ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const method = options?.method ?? 'GET'
  const body = options?.body as string | undefined
  const tag = body ? `${method} ${path} ${JSON.stringify(body).slice(0, 80)}` : `${method} ${path}`
  console.log(`[API] → ${tag}`)
  const start = performance.now()
  try {
    const res = await fetch(`${BASE}${path}`, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(err.error || res.statusText)
    }
    const data = await res.json()
    const elapsed = (performance.now() - start).toFixed(0)
    const preview = Array.isArray(data) ? `[${data.length} items]` : JSON.stringify(data).slice(0, 60)
    console.log(`[API] ← ${method} ${path} (${elapsed}ms) ${preview}`)
    return data
  } catch (err) {
    const elapsed = (performance.now() - start).toFixed(0)
    console.error(`[API] ✗ ${method} ${path} (${elapsed}ms) ${err}`)
    throw err
  }
}

export const api = {
  health: () => request<{ status: string }>('/api/health'),

  listSessions: (q?: string, project?: string) => {
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    if (project) params.set('project', project)
    const qs = params.toString()
    return request<Session[]>(qs ? `/api/sessions?${qs}` : '/api/sessions')
  },

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
