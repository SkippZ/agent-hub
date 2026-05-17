import { Outlet, Link } from 'react-router-dom'
import { useProject } from '../context/ProjectContext'

export function Layout() {
  const { selectedProject } = useProject()

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b border-border">
        <div className="mx-auto max-w-6xl px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Link to="/" className="flex items-center gap-2 font-semibold text-lg">
              <span className="text-primary">◆</span>
              Agent Hub
            </Link>
            {selectedProject && (
              <>
                <span className="text-muted-foreground mx-1">/</span>
                <Link
                  to={`/project/${encodeURIComponent(selectedProject.name)}`}
                  className="font-medium text-sm hover:text-foreground transition-colors"
                >
                  {selectedProject.name}
                </Link>
              </>
            )}
          </div>
          <nav className="flex items-center gap-4 text-sm text-muted-foreground">
            <Link to="/" className="hover:text-foreground transition-colors">
              Projects
            </Link>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
