import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { ProjectPage } from './pages/ProjectPage'
import { SessionDetail } from './pages/SessionDetail'
import { SkillsPage } from './pages/SkillsPage'
import { ProjectProvider } from './context/ProjectContext'

const queryClient = new QueryClient()

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ProjectProvider>
        <BrowserRouter>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/project/:name" element={<ProjectPage />} />
              <Route path="/session/:id" element={<SessionDetail />} />
              <Route path="/skills" element={<SkillsPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </ProjectProvider>
    </QueryClientProvider>
  )
}
