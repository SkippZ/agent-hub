import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Card, CardContent, CardHeader } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { StatusBadge } from '../components/StatusBadge'
import { NewSessionDialog } from '../components/NewSessionDialog'
import { useProject } from '../context/ProjectContext'
import type { Session } from '../types'

export function ProjectPage() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const { setSelectedProject } = useProject()
  const [showNewSession, setShowNewSession] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(searchQuery), 300)
    return () => clearTimeout(timer)
  }, [searchQuery])

  const { data: sessions, isLoading } = useQuery({
    queryKey: ['sessions', debouncedQuery, name],
    queryFn: () => api.listSessions(debouncedQuery || undefined, name),
    refetchInterval: 5000,
  })

  const groups = groupBy(sessions || [], (s) => s.status)

  if (!name) {
    navigate('/', { replace: true })
    return null
  }

  return (
    <div className="animate-in">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Link to="/" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
            ← All Projects
          </Link>
          <h1 className="text-2xl font-bold">{name}</h1>
        </div>
        <Button onClick={() => setShowNewSession(true)}>+ New Session</Button>
      </div>

      <div className="mb-6">
        <Input
          type="search"
          placeholder="Search sessions by task description..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
      </div>

      {isLoading && (
        <div className="text-center text-muted-foreground py-12">Loading sessions...</div>
      )}

      {!isLoading && (!sessions || sessions.length === 0) && (
        <div className="text-center text-muted-foreground py-12 border border-dashed border-border rounded-lg">
          <p className="mb-2">No sessions for this project yet</p>
          <Button variant="outline" onClick={() => setShowNewSession(true)}>
            Start your first agent session
          </Button>
        </div>
      )}

      <SessionGroup title="Running" sessions={getGroup(groups, 'running')} />
      <SessionGroup title="Needs Attention" sessions={getGroup(groups, 'needs_attention')} />
      <SessionGroup title="Done" sessions={getGroup(groups, 'done')} />

      <NewSessionDialog
        open={showNewSession}
        onClose={() => setShowNewSession(false)}
        initialProject={name}
      />
    </div>
  )
}

function getGroup(groups: Record<string, Session[]>, key: string): Session[] {
  return groups[key] || []
}

function SessionGroup({ title, sessions }: { title: string; sessions: Session[] }) {
  if (sessions.length === 0) return null

  return (
    <section className="mb-8">
      <h2 className="text-lg font-semibold text-muted-foreground mb-3">{title}</h2>
      <div className="grid gap-3">
        {sessions.map((session) => (
          <SessionCard key={session.id} session={session} />
        ))}
      </div>
    </section>
  )
}

function SessionCard({ session }: { session: Session }) {
  return (
    <a href={`/session/${session.id}`} className="block">
      <Card className="hover:border-primary/30 transition-colors cursor-pointer">
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono text-muted-foreground uppercase">
                {session.agent_type}
              </span>
              <span className="text-sm text-muted-foreground">/</span>
              <span className="font-medium">{session.project_name}</span>
            </div>
            <StatusBadge status={session.status} />
          </div>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground line-clamp-2">
            {session.task_description}
          </p>
          <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
            <span className="font-mono">{session.feature_branch}</span>
            <span>·</span>
            <span>{new Date(session.created_at).toLocaleString()}</span>
          </div>
        </CardContent>
      </Card>
    </a>
  )
}

function groupBy<T>(items: T[], fn: (item: T) => string): Record<string, T[]> {
  const groups: Record<string, T[]> = {}
  for (const item of items) {
    const key = fn(item)
    if (!groups[key]) groups[key] = []
    groups[key].push(item)
  }
  return groups
}
