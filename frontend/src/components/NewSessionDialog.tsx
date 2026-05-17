import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Button } from './ui/button'
import { Card, CardContent, CardHeader } from './ui/card'
import { Input } from './ui/input'
import { Textarea } from './ui/textarea'
import { Select } from './ui/select'
import type { AgentType } from '../types'

interface NewSessionDialogProps {
  open: boolean
  onClose: () => void
  initialProject?: string
}

export function NewSessionDialog({ open, onClose, initialProject }: NewSessionDialogProps) {
  const queryClient = useQueryClient()

  const { data: projects } = useQuery({
    queryKey: ['projects'],
    queryFn: api.listProjects,
  })

  const [projectName, setProjectName] = useState(initialProject || '')
  const [agentType, setAgentType] = useState<AgentType>('opencode')
  const [baseBranch, setBaseBranch] = useState('')
  const [taskDescription, setTaskDescription] = useState('')

  const { data: branches } = useQuery({
    queryKey: ['branches', projectName],
    queryFn: () => api.listBranches(projectName),
    enabled: !!projectName,
  })

  const createSession = useMutation({
    mutationFn: () =>
      api.createSession({
        agent_type: agentType,
        project_name: projectName,
        base_branch: baseBranch,
        task_description: taskDescription,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
      onClose()
      reset()
    },
  })

  function reset() {
    setProjectName('')
    setAgentType('opencode')
    setBaseBranch('')
    setTaskDescription('')
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm animate-in">
      <Card className="w-full max-w-lg mx-4">
        <CardHeader>
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">New Agent Session</h2>
            <Button variant="ghost" size="icon" onClick={onClose}>
              ✕
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              createSession.mutate()
            }}
            className="space-y-4"
          >
            <div className="space-y-2">
              <label className="text-sm font-medium">Project</label>
              <Select
                value={projectName}
                onChange={(e) => {
                  setProjectName(e.target.value)
                  setBaseBranch('')
                }}
                placeholder="Select project..."
                disabled={!!initialProject}
                options={(projects || []).map((p) => ({ value: p.name, label: p.name }))}
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Agent</label>
              <Select
                value={agentType}
                onChange={(e) => setAgentType(e.target.value as AgentType)}
                options={[
                  { value: 'opencode', label: 'OpenCode' },
                  { value: 'claude-code', label: 'Claude Code' },
                ]}
              />
            </div>

            {branches && branches.length > 0 && (
              <div className="space-y-2">
                <label className="text-sm font-medium">Base Branch</label>
                <Select
                  value={baseBranch}
                  onChange={(e) => setBaseBranch(e.target.value)}
                  placeholder="Select base branch..."
                  options={branches.map((b) => ({ value: b.name, label: b.name }))}
                />
              </div>
            )}

            <div className="space-y-2">
              <label className="text-sm font-medium">Task Description</label>
              <Textarea
                value={taskDescription}
                onChange={(e) => setTaskDescription(e.target.value)}
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    e.preventDefault()
                    if (projectName && baseBranch && taskDescription && !createSession.isPending) {
                      createSession.mutate()
                    }
                  }
                }}
                placeholder="Describe what the agent should do..."
                rows={3}
              />
            </div>

            {createSession.error && (
              <p className="text-sm text-destructive">
                {(createSession.error as Error).message}
              </p>
            )}

            <div className="flex justify-end gap-2 pt-2">
              <Button type="button" variant="outline" onClick={onClose}>
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!projectName || !baseBranch || !taskDescription || createSession.isPending}
              >
                {createSession.isPending ? 'Starting...' : 'Start Session'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
