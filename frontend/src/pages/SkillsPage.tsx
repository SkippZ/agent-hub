import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Textarea } from '../components/ui/textarea'
import { Card, CardContent, CardHeader } from '../components/ui/card'
import type { Skill } from '../types'

export function SkillsPage() {
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [showInstall, setShowInstall] = useState(false)
  const [editSkill, setEditSkill] = useState<Skill | null>(null)

  const { data: skills, isLoading } = useQuery({
    queryKey: ['skills'],
    queryFn: api.listSkills,
  })

  const deleteSkill = useMutation({
    mutationFn: api.deleteSkill,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['skills'] }),
  })

  return (
    <div className="animate-in">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold mb-1">Skills</h1>
          <p className="text-muted-foreground text-sm">
            Create, edit, and manage project skills.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setShowInstall(true)}>
            Install from GitHub
          </Button>
          <Button onClick={() => setShowCreate(true)}>
            Create Skill
          </Button>
        </div>
      </div>

      {showCreate && (
        <SkillFormDialog
          onClose={() => setShowCreate(false)}
        />
      )}

      {showInstall && (
        <InstallSkillDialog
          onClose={() => setShowInstall(false)}
        />
      )}

      {editSkill && (
        <SkillFormDialog
          skill={editSkill}
          onClose={() => setEditSkill(null)}
        />
      )}

      {isLoading && (
        <div className="text-center text-muted-foreground py-12">Loading skills...</div>
      )}

      {!isLoading && (!skills || skills.length === 0) && (
        <div className="text-center text-muted-foreground py-12 border border-dashed border-border rounded-lg">
          <p>No skills yet. Create one or install from GitHub.</p>
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {skills?.map((skill) => (
          <Card key={skill.name}>
            <CardHeader>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <h3 className="font-semibold truncate">{skill.name}</h3>
                  {skill.description && (
                    <p className="text-sm text-muted-foreground mt-1 line-clamp-2">
                      {skill.description}
                    </p>
                  )}
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setEditSkill(skill)}
                >
                  Edit
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => {
                    if (confirm(`Delete skill "${skill.name}"?`)) {
                      deleteSkill.mutate(skill.name)
                    }
                  }}
                >
                  Delete
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}

function SkillFormDialog({ skill, onClose }: { skill?: Skill; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(skill?.name || '')
  const [description, setDescription] = useState(skill?.description || '')
  const [content, setContent] = useState(skill?.content || '')

  const createSkill = useMutation({
    mutationFn: () => api.createSkill({ name, description, content }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
      onClose()
    },
  })

  const updateSkill = useMutation({
    mutationFn: () => api.updateSkill(skill!.name, { name, description, content }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
      onClose()
    },
  })

  const isPending = createSkill.isPending || updateSkill.isPending
  const error = createSkill.error || updateSkill.error

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm animate-in">
      <Card className="w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto">
        <CardHeader>
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">
              {skill ? `Edit: ${skill.name}` : 'Create Skill'}
            </h2>
            <Button variant="ghost" size="icon" onClick={onClose}>✕</Button>
          </div>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (skill) {
                updateSkill.mutate()
              } else {
                createSkill.mutate()
              }
            }}
            className="space-y-4"
          >
            <div className="space-y-2">
              <label className="text-sm font-medium">Name</label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="skill-name"
                disabled={!!skill}
                required
              />
              {skill && (
                <p className="text-xs text-muted-foreground">Name cannot be changed after creation.</p>
              )}
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Description</label>
              <Input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Short description of this skill"
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Content (SKILL.md)</label>
              <Textarea
                value={content}
                onChange={(e) => setContent(e.target.value)}
                placeholder="# Skill Name&#10;&#10;Describe what this skill does..."
                rows={16}
                className="font-mono text-xs"
              />
            </div>

            {error && (
              <p className="text-sm text-destructive">{(error as Error).message}</p>
            )}

            <div className="flex justify-end gap-2 pt-2">
              <Button type="button" variant="outline" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" disabled={!name || isPending}>
                {isPending ? 'Saving...' : skill ? 'Save Changes' : 'Create Skill'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

function InstallSkillDialog({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [url, setUrl] = useState('')
  const [skillName, setSkillName] = useState('')

  const installSkill = useMutation({
    mutationFn: () => api.installSkill({ url, name: skillName || undefined }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
      onClose()
    },
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm animate-in">
      <Card className="w-full max-w-lg mx-4">
        <CardHeader>
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">Install Skill from GitHub</h2>
            <Button variant="ghost" size="icon" onClick={onClose}>✕</Button>
          </div>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              installSkill.mutate()
            }}
            className="space-y-4"
          >
            <div className="space-y-2">
              <label className="text-sm font-medium">GitHub Repository URL</label>
              <Input
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://github.com/user/repo"
                required
              />
              <p className="text-xs text-muted-foreground">
                The repository should contain skills in a <code>.opencode/skills/</code> directory.
              </p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Skill Name (optional)</label>
              <Input
                value={skillName}
                onChange={(e) => setSkillName(e.target.value)}
                placeholder="Leave empty to install all skills from the repo"
              />
            </div>

            {installSkill.error && (
              <p className="text-sm text-destructive">{(installSkill.error as Error).message}</p>
            )}

            {installSkill.isSuccess && (
              <p className="text-sm text-green-600">Skill(s) installed successfully!</p>
            )}

            <div className="flex justify-end gap-2 pt-2">
              <Button type="button" variant="outline" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" disabled={!url || installSkill.isPending}>
                {installSkill.isPending ? 'Installing...' : 'Install'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
