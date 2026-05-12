import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Card, CardContent, CardHeader } from '../components/ui/card'
import { StatusBadge } from '../components/StatusBadge'
import type { Session } from '../types'

export function Dashboard() {
  const { data: sessions, isLoading } = useQuery({
    queryKey: ['sessions'],
    queryFn: api.listSessions,
    refetchInterval: 5000,
  })

  const groups = groupBy(sessions || [], (s) => s.status)

  return (
    <div className="animate-in">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Dashboard</h1>
      </div>

      {isLoading && (
        <div className="text-center text-muted-foreground py-12">
          Loading sessions...
        </div>
      )}

      <SessionGroup title="Running" status="running" sessions={groups.running || []} />
      <SessionGroup title="Needs Attention" status="needs_attention" sessions={groups.needs_attention || []} />
      <SessionGroup title="Done" status="done" sessions={groups.done || []} />
    </div>
  )
}

function SessionGroup({ title, sessions }: { title: string; status: string; sessions: Session[] }) {
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
            <span>{session.feature_branch}</span>
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
