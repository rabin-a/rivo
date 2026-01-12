import { Routes, Route, NavLink } from 'react-router-dom'
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

function App() {
  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-900">
      {/* Sidebar */}
      <aside className="fixed inset-y-0 left-0 w-64 bg-white dark:bg-slate-800 border-r border-slate-200 dark:border-slate-700">
        <div className="flex items-center gap-2 px-6 py-4 border-b border-slate-200 dark:border-slate-700">
          <Activity className="w-8 h-8 text-primary-600" />
          <span className="text-xl font-bold text-slate-900 dark:text-white">Rivo</span>
        </div>

        <nav className="p-4 space-y-1">
          <NavItem to="/" icon={<LayoutDashboard className="w-5 h-5" />}>
            Dashboard
          </NavItem>
          <NavItem to="/jobs" icon={<ListTodo className="w-5 h-5" />}>
            Jobs
          </NavItem>
          <NavItem to="/queues" icon={<Layers className="w-5 h-5" />}>
            Queues
          </NavItem>
          <NavItem to="/workflows" icon={<GitBranch className="w-5 h-5" />}>
            Workflows
          </NavItem>
          <NavItem to="/schedules" icon={<Clock className="w-5 h-5" />}>
            Schedules
          </NavItem>
        </nav>
      </aside>

      {/* Main content */}
      <main className="ml-64 min-h-screen">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/jobs" element={<Jobs />} />
          <Route path="/queues" element={<Queues />} />
          <Route path="/workflows" element={<Workflows />} />
          <Route path="/workflows/:name/runs" element={<WorkflowRuns />} />
          <Route path="/workflow-runs" element={<WorkflowRuns />} />
          <Route path="/workflow-runs/:id" element={<WorkflowRunDetail />} />
          <Route path="/schedules" element={<Schedules />} />
        </Routes>
      </main>
    </div>
  )
}

function NavItem({ to, icon, children }: { to: string; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${
          isActive
            ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/50 dark:text-primary-300'
            : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-700/50'
        }`
      }
    >
      {icon}
      <span className="font-medium">{children}</span>
    </NavLink>
  )
}

export default App
