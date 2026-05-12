import { useState, useRef, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Card, CardContent } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { Textarea } from '../components/ui/textarea'
import { StatusBadge } from '../components/StatusBadge'
import { Markdown } from '../components/Markdown'
import { useWebSocket } from '../hooks/useWebSocket'
import type { Message, SessionStatus } from '../types'

export function SessionDetail() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const [liveOutput, setLiveOutput] = useState<string[]>([])
  const [input, setInput] = useState('')
  const outputEndRef = useRef<HTMLDivElement>(null)

  const { data: session, isLoading: sessionLoading } = useQuery({
    queryKey: ['session', id],
    queryFn: () => api.getSession(id!),
    enabled: !!id,
  })

  const { data: messages, isLoading: messagesLoading } = useQuery({
    queryKey: ['messages', id],
    queryFn: () => api.getMessages(id!),
    enabled: !!id,
  })

  const { data: changes } = useQuery({
    queryKey: ['changes', id],
    queryFn: () => api.getChanges(id!),
    enabled: !!id,
    refetchInterval: 10000,
  })

  const changeStatus = useMutation({
    mutationFn: (status: SessionStatus) => api.updateSessionStatus(id!, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['session', id] }),
  })

  const sendMessage = useMutation({
    mutationFn: (content: string) => api.sendMessage(id!, content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['messages', id] })
      setInput('')
    },
  })

  useWebSocket(id, (msg) => {
    if (msg.type === 'output') {
      setLiveOutput((prev) => [...prev, msg.data!])
    } else if (msg.type === 'status') {
      queryClient.invalidateQueries({ queryKey: ['session', id] })
    }
  })

  useEffect(() => {
    outputEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [liveOutput, messages])

  // Combine stored messages with live output for display
  const allMessages: { role: string; content: string; key: string }[] = []
  const storedMessages = messages || []
  const hasLiveOutput = liveOutput.length > 0
  const lastStored = storedMessages[storedMessages.length - 1]
  const lastLiveIdx = liveOutput.length - 1

  storedMessages.forEach((m, i) => {
    // Don't duplicate if live output already shows recent agent message
    if (hasLiveOutput && m.role === 'agent' && i >= storedMessages.length - 3) {
      return
    }
    allMessages.push({ role: m.role, content: m.content, key: m.id })
  })

  if (hasLiveOutput) {
    allMessages.push({
      role: 'agent',
      content: liveOutput.join(''),
      key: 'live',
    })
  }

  if (sessionLoading || messagesLoading) {
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
              <div className="flex items-center gap-2 mb-1">
                <span className="text-xs font-mono text-muted-foreground uppercase">
                  {session.agent_type}
                </span>
                <span className="text-muted-foreground">/</span>
                <h1 className="text-xl font-bold">{session.project_name}</h1>
              </div>
              <p className="text-sm text-muted-foreground">{session.task_description}</p>
            </div>
            <div className="flex items-center gap-2">
              <StatusBadge status={session.status} />
              {session.status !== 'done' && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => changeStatus.mutate('done')}
                >
                  Mark Done
                </Button>
              )}
            </div>
          </div>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground font-mono">
            <span>Branch: {session.feature_branch}</span>
            <span>Base: {session.base_branch}</span>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Chat + Live Output */}
        <Card>
          <CardContent className="p-4">
            <h2 className="text-sm font-semibold text-muted-foreground mb-3 uppercase tracking-wider">
              Chat
            </h2>
            <div className="space-y-3 max-h-[50vh] overflow-y-auto mb-4">
              {allMessages.map((msg) => (
                <div
                  key={msg.key}
                  className={`rounded-lg p-3 text-sm ${
                    msg.role === 'user'
                      ? 'bg-primary/10 border border-primary/20'
                      : 'bg-secondary border border-border'
                  }`}
                >
                  <div className="text-xs text-muted-foreground mb-1 font-medium">
                    {msg.role === 'user' ? 'You' : 'Agent'}
                    {msg.key === 'live' && (
                      <span className="ml-2 inline-block h-2 w-2 rounded-full bg-blue-400 animate-pulse" />
                    )}
                  </div>
                  <div className="text-sm">
                    <Markdown content={msg.content} />
                  </div>
                </div>
              ))}
              <div ref={outputEndRef} />
            </div>

            {session.status !== 'done' && (
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  if (input.trim()) sendMessage.mutate(input)
                }}
                className="flex gap-2"
              >
                <Textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="Type a message..."
                  rows={2}
                  className="flex-1"
                />
                <Button type="submit" disabled={!input.trim() || sendMessage.isPending}>
                  Send
                </Button>
              </form>
            )}
          </CardContent>
        </Card>

        {/* Code Changes */}
        <Card>
          <CardContent className="p-4">
            <h2 className="text-sm font-semibold text-muted-foreground mb-3 uppercase tracking-wider">
              Code Changes
            </h2>
            <div className="space-y-3 max-h-[60vh] overflow-y-auto">
              {(!changes || changes.length === 0) && (
                <p className="text-sm text-muted-foreground">No code changes yet.</p>
              )}
              {changes?.map((snap) => (
                <div key={snap.id} className="rounded-lg bg-secondary p-3 text-sm">
                  {snap.commit_hash && (
                    <div className="text-xs text-muted-foreground mb-2 font-mono">
                      {snap.commit_hash}
                      {snap.summary && <> — {snap.summary}</>}
                    </div>
                  )}
                  <pre className="text-xs font-mono whitespace-pre-wrap overflow-x-auto max-h-48 overflow-y-auto">
                    {snap.diff.length > 2000
                      ? snap.diff.slice(0, 2000) + '\n... (truncated)'
                      : snap.diff}
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
