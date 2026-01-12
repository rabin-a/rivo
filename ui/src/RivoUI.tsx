import { useState, useMemo } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  LayoutDashboard,
  ListTodo,
  GitBranch,
  Clock,
  Activity,
  Layers
} from 'lucide-react'
import Dashboard from './pages/Dashboard'
import Jobs from './pages/Jobs'
import Queues from './pages/Queues'
import Workflows from './pages/Workflows'
import WorkflowRuns from './pages/WorkflowRuns'
import WorkflowRunDetail from './pages/WorkflowRunDetail'
import Schedules from './pages/Schedules'
import { api } from './api/client'

export type RivoPage =
  | 'dashboard'
  | 'jobs'
  | 'queues'
  | 'workflows'
  | 'workflow-runs'
  | 'schedules'

export interface RivoUIProps {
  /** Show or hide the sidebar. Default: true */
  sidebar?: boolean
  /** Initial page to display. Default: 'dashboard' */
  page?: RivoPage
  /** Specific workflow name for workflow-runs page */
  workflowName?: string
  /** Specific workflow run ID for detail page */
  workflowRunId?: number
  /** API base URL. Default: '/api/v1' */
  apiBaseUrl?: string
  /** Callback when page changes */
  onPageChange?: (page: RivoPage, params?: { workflowName?: string; workflowRunId?: number }) => void
  /** Custom className for the container */
  className?: string
  /** Dark mode. Default: follows system preference */
  darkMode?: boolean
}

/**
 * RivoUI - Embeddable Rivo Dashboard Component
 *
 * Usage:
 * ```tsx
 * // Full UI with sidebar
 * <RivoUI />
 *
 * // Specific page without sidebar
 * <RivoUI sidebar={false} page="jobs" />
 *
 * // With custom API URL
 * <RivoUI apiBaseUrl="http://localhost:8080/api/v1" />
 * ```
 */
export function RivoUI({
  sidebar = true,
  page: initialPage = 'dashboard',
  workflowName: initialWorkflowName,
  workflowRunId: initialWorkflowRunId,
  apiBaseUrl,
  onPageChange,
  className = '',
  darkMode,
}: RivoUIProps) {
  const [currentPage, setCurrentPage] = useState<RivoPage>(initialPage)
  const [workflowName, setWorkflowName] = useState<string | undefined>(initialWorkflowName)
  const [workflowRunId, setWorkflowRunId] = useState<number | undefined>(initialWorkflowRunId)

  // Create a dedicated QueryClient for this instance
  const queryClient = useMemo(() => new QueryClient({
    defaultOptions: {
      queries: {
        refetchInterval: 5000,
        staleTime: 1000,
      },
    },
  }), [])

  // Update API base URL if provided
  if (apiBaseUrl) {
    api.setBaseUrl(apiBaseUrl)
  }

  const handlePageChange = (newPage: RivoPage, params?: { workflowName?: string; workflowRunId?: number }) => {
    setCurrentPage(newPage)
    setWorkflowName(params?.workflowName)
    setWorkflowRunId(params?.workflowRunId)
    onPageChange?.(newPage, params)
  }

  const navigateToWorkflowRuns = (name: string) => {
    handlePageChange('workflow-runs', { workflowName: name })
  }

  const navigateToWorkflowRunDetail = (id: number) => {
    handlePageChange('workflow-runs', { workflowRunId: id })
  }

  const renderPage = () => {
    if (workflowRunId) {
      return <WorkflowRunDetail embeddedRunId={workflowRunId} />
    }

    switch (currentPage) {
      case 'dashboard':
        return <Dashboard />
      case 'jobs':
        return <Jobs />
      case 'queues':
        return <Queues />
      case 'workflows':
        return <Workflows onNavigateToRuns={navigateToWorkflowRuns} />
      case 'workflow-runs':
        return <WorkflowRuns embeddedWorkflowName={workflowName} onNavigateToDetail={navigateToWorkflowRunDetail} />
      case 'schedules':
        return <Schedules />
      default:
        return <Dashboard />
    }
  }

  const darkModeClass = darkMode === true ? 'dark' : darkMode === false ? '' : ''

  const content = (
    <div className={`min-h-full bg-slate-50 dark:bg-slate-900 ${darkModeClass} ${className}`}>
      {sidebar ? (
        <div className="flex">
          {/* Sidebar */}
          <aside className="w-64 min-h-screen bg-white dark:bg-slate-800 border-r border-slate-200 dark:border-slate-700 flex-shrink-0">
            <div className="flex items-center gap-2 px-6 py-4 border-b border-slate-200 dark:border-slate-700">
              <Activity className="w-8 h-8 text-primary-600" />
              <span className="text-xl font-bold text-slate-900 dark:text-white">Rivo</span>
            </div>

            <nav className="p-4 space-y-1">
              <NavItem
                icon={<LayoutDashboard className="w-5 h-5" />}
                active={currentPage === 'dashboard' && !workflowRunId}
                onClick={() => handlePageChange('dashboard')}
              >
                Dashboard
              </NavItem>
              <NavItem
                icon={<ListTodo className="w-5 h-5" />}
                active={currentPage === 'jobs' && !workflowRunId}
                onClick={() => handlePageChange('jobs')}
              >
                Jobs
              </NavItem>
              <NavItem
                icon={<Layers className="w-5 h-5" />}
                active={currentPage === 'queues' && !workflowRunId}
                onClick={() => handlePageChange('queues')}
              >
                Queues
              </NavItem>
              <NavItem
                icon={<GitBranch className="w-5 h-5" />}
                active={(currentPage === 'workflows' || currentPage === 'workflow-runs') && !workflowRunId}
                onClick={() => handlePageChange('workflows')}
              >
                Workflows
              </NavItem>
              <NavItem
                icon={<Clock className="w-5 h-5" />}
                active={currentPage === 'schedules' && !workflowRunId}
                onClick={() => handlePageChange('schedules')}
              >
                Schedules
              </NavItem>
            </nav>
          </aside>

          {/* Main content */}
          <main className="flex-1 min-h-screen">
            {renderPage()}
          </main>
        </div>
      ) : (
        <div className="min-h-full">
          {renderPage()}
        </div>
      )}
    </div>
  )

  return (
    <QueryClientProvider client={queryClient}>
      {content}
    </QueryClientProvider>
  )
}

function NavItem({
  icon,
  children,
  active,
  onClick
}: {
  icon: React.ReactNode
  children: React.ReactNode
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${
        active
          ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/50 dark:text-primary-300'
          : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-700/50'
      }`}
    >
      {icon}
      <span className="font-medium">{children}</span>
    </button>
  )
}

export default RivoUI
