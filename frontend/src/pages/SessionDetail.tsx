import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Card, CardContent } from '../components/ui/card'
import { StatusBadge } from '../components/StatusBadge'

export function SessionDetail() {
  const { id } = useParams<{ id: string }>()

  const { data: session, isLoading } = useQuery({
    queryKey: ['session', id],
    queryFn: () => api.getSession(id!),
    enabled: !!id,
  })

  const { data: messages } = useQuery({
    queryKey: ['messages', id],
    queryFn: () => api.getMessages(id!),
    enabled: !!id,
    refetchInterval: 3000,
  })

  const { data: changes } = useQuery({
    queryKey: ['changes', id],
    queryFn: () => api.getChanges(id!),
    enabled: !!id,
    refetchInterval: 10000,
  })

  if (isLoading) {
    return <div className="text-center text-muted-foreground py-12">Loading session...</div>
  }

  if (!session) {
    return <div className="text-center text-muted-foreground py-12">Session not found</div>
  }

  return (
    <div className="animate-in">
      <Link to="/" className="text-sm text-muted-foreground hover:text-foreground mb-4 inline-block">
        ← Back to Dashboard
      </Link>

      <Card className="mb-6">
        <CardContent className="p-6">
          <div className="flex items-start justify-between mb-4">
            <div>
              <h1 className="text-xl font-bold">{session.project_name}</h1>
              <p className="text-sm text-muted-foreground mt-1">{session.task_description}</p>
            </div>
            <StatusBadge status={session.status} />
          </div>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground font-mono">
            <span>Agent: {session.agent_type}</span>
            <span>Branch: {session.feature_branch}</span>
            <span>Base: {session.base_branch}</span>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardContent className="p-4">
            <h2 className="text-sm font-semibold text-muted-foreground mb-3 uppercase tracking-wider">
              Chat History
            </h2>
            <div className="space-y-3 max-h-[60vh] overflow-y-auto">
              {messages?.length === 0 && (
                <p className="text-sm text-muted-foreground">No messages yet.</p>
              )}
              {messages?.map((msg) => (
                <div
                  key={msg.id}
                  className={`rounded-lg p-3 text-sm ${
                    msg.role === 'user'
                      ? 'bg-primary/10 border border-primary/20'
                      : 'bg-secondary border border-border'
                  }`}
                >
                  <div className="text-xs text-muted-foreground mb-1 font-medium">
                    {msg.role === 'user' ? 'You' : 'Agent'}
                  </div>
                  <pre className="whitespace-pre-wrap font-sans text-sm">
                    {msg.content}
                  </pre>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <h2 className="text-sm font-semibold text-muted-foreground mb-3 uppercase tracking-wider">
              Code Changes
            </h2>
            <div className="space-y-3 max-h-[60vh] overflow-y-auto">
              {changes?.length === 0 && (
                <p className="text-sm text-muted-foreground">No code changes yet.</p>
              )}
              {changes?.map((snap) => (
                <div key={snap.id} className="rounded-lg bg-secondary p-3 text-sm">
                  {snap.commit_hash && (
                    <div className="text-xs text-muted-foreground mb-2 font-mono">
                      {snap.commit_hash} — {snap.summary}
                    </div>
                  )}
                  <pre className="text-xs font-mono whitespace-pre-wrap overflow-x-auto">
                    {snap.diff}
                  </pre>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
