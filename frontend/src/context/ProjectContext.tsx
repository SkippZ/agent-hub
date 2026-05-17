import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import type { Project } from '../types'

const STORAGE_KEY = 'selected_project'

interface ProjectContextValue {
  selectedProject: Project | null
  setSelectedProject: (project: Project | null) => void
  projects: Project[]
}

const ProjectContext = createContext<ProjectContextValue | null>(null)

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [selectedProject, setSelectedProjectState] = useState<Project | null>(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      try { return JSON.parse(stored) } catch { return null }
    }
    return null
  })

  const { data: projects = [] } = useQuery({
    queryKey: ['projects'],
    queryFn: api.listProjects,
  })

  const setSelectedProject = (project: Project | null) => {
    setSelectedProjectState(project)
    if (project) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(project))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  }

  useEffect(() => {
    if (selectedProject && projects.length > 0) {
      const stillExists = projects.find(p => p.name === selectedProject.name)
      if (!stillExists) {
        setSelectedProject(null)
      }
    }
  }, [projects, selectedProject])

  return (
    <ProjectContext.Provider value={{ selectedProject, setSelectedProject, projects }}>
      {children}
    </ProjectContext.Provider>
  )
}

export function useProject() {
  const ctx = useContext(ProjectContext)
  if (!ctx) throw new Error('useProject must be used within ProjectProvider')
  return ctx
}
