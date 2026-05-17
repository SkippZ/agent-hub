import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Card, CardContent } from '../components/ui/card'
import { useProject } from '../context/ProjectContext'

export function Dashboard() {
  const navigate = useNavigate()
  const { selectedProject, setSelectedProject, projects } = useProject()

  const { data: projectsList, isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: api.listProjects,
  })

  if (selectedProject) {
    navigate(`/project/${encodeURIComponent(selectedProject.name)}`, { replace: true })
    return null
  }

  const projectsToShow = projectsList || projects

  return (
    <div className="animate-in">
      <div className="mb-8">
        <h1 className="text-2xl font-bold mb-2">Select a Project</h1>
        <p className="text-muted-foreground">
          Choose a project to view its agent sessions and start new ones.
        </p>
      </div>

      {isLoading && (
        <div className="text-center text-muted-foreground py-12">Loading projects...</div>
      )}

      {!isLoading && (!projectsToShow || projectsToShow.length === 0) && (
        <div className="text-center text-muted-foreground py-12 border border-dashed border-border rounded-lg">
          <p>No projects found. Configure your projects directory.</p>
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {projectsToShow?.map((project) => (
          <Card
            key={project.name}
            className="hover:border-primary/30 transition-colors cursor-pointer"
            onClick={() => {
              setSelectedProject(project)
              navigate(`/project/${encodeURIComponent(project.name)}`)
            }}
          >
            <CardContent className="p-6">
              <h2 className="font-semibold text-lg mb-1">{project.name}</h2>
              <p className="text-sm text-muted-foreground truncate">{project.path}</p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
