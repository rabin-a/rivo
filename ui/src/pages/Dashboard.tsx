import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api, WorkflowRun } from '../api/client'
import { formatDistanceToNow } from 'date-fns'
import {
  CheckCircle2,
  Clock,
  AlertCircle,
  PlayCircle,
  XCircle,
  GitBranch,
  Server,
  Code,
  Layers,
  TrendingUp,
  Activity,
  ChevronRight,
  Heart,
  HeartOff,
  Loader2
} from 'lucide-react'

export default function Dashboard() {
  const { data: stats, isLoading: statsLoading, error: statsError } = useQuery({
    queryKey: ['stats'],
    queryFn: () => api.getStats(),
    refetchInterval: 5000,
  })

  const { data: workers } = useQuery({
    queryKey: ['workers'],
    queryFn: () => api.getWorkers(),
    refetchInterval: 5000,
  })

  const { data: workerHealth } = useQuery({
    queryKey: ['workers-health'],
    queryFn: () => api.getWorkersHealth(),
    refetchInterval: 5000,
  })

  const { data: handlers } = useQuery({
    queryKey: ['handlers'],
    queryFn: () => api.getHandlers(),
  })

  const { data: workflows } = useQuery({
    queryKey: ['workflows'],
    queryFn: () => api.getWorkflows(),
  })

  const { data: workflowRuns } = useQuery({
    queryKey: ['workflow-runs'],
    queryFn: () => api.getWorkflowRuns({ limit: 5 }),
    refetchInterval: 5000,
  })

  if (statsLoading) {
    return (
      <div className="p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-slate-400" />
      </div>
    )
  }

  if (statsError) {
    return (
      <div className="p-8">
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 text-red-700 dark:text-red-300">
          Failed to load stats. Make sure the API server is running.
        </div>
      </div>
    )
  }

  const healthyWorkers = workerHealth?.filter(w => w.is_healthy).length ?? 0
  const totalWorkers = workerHealth?.length ?? 0

  const runningWorkflows = workflowRuns?.filter(r => r.state === 'running').length ?? 0
  const completedWorkflows = workflowRuns?.filter(r => r.state === 'completed').length ?? 0
  const failedWorkflows = workflowRuns?.filter(r => r.state === 'failed').length ?? 0

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
          Dashboard
        </h1>
        <div className="flex items-center gap-2 text-sm text-slate-500">
          <Activity className="w-4 h-4" />
          Auto-refreshing every 5s
        </div>
      </div>

      {/* Overview Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4 mb-8">
        <StatCard
          label="Total Jobs"
          value={stats?.total ?? 0}
          icon={<Layers className="w-5 h-5" />}
          color="bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
        />
        <StatCard
          label="Running"
          value={stats?.running ?? 0}
          icon={<PlayCircle className="w-5 h-5" />}
          color="bg-yellow-100 dark:bg-yellow-900/30 text-yellow-600"
        />
        <StatCard
          label="Completed"
          value={stats?.completed ?? 0}
          icon={<CheckCircle2 className="w-5 h-5" />}
          color="bg-green-100 dark:bg-green-900/30 text-green-600"
        />
        <StatCard
          label="Failed"
          value={stats?.failed ?? 0}
          icon={<XCircle className="w-5 h-5" />}
          color="bg-red-100 dark:bg-red-900/30 text-red-600"
        />
        <StatCard
          label="Workers"
          value={`${healthyWorkers}/${totalWorkers}`}
          icon={healthyWorkers > 0 ? <Heart className="w-5 h-5" /> : <HeartOff className="w-5 h-5" />}
          color={healthyWorkers > 0 ? "bg-green-100 dark:bg-green-900/30 text-green-600" : "bg-red-100 dark:bg-red-900/30 text-red-600"}
        />
        <StatCard
          label="Handlers"
          value={handlers?.length ?? 0}
          icon={<Code className="w-5 h-5" />}
          color="bg-purple-100 dark:bg-purple-900/30 text-purple-600"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* Queues Overview */}
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white flex items-center gap-2">
              <Layers className="w-5 h-5 text-blue-500" />
              Queue Overview
            </h2>
            <Link to="/queues" className="text-sm text-primary-600 hover:text-primary-700 flex items-center gap-1">
              View all <ChevronRight className="w-4 h-4" />
            </Link>
          </div>
          <div className="p-4">
            {stats?.queues && stats.queues.length > 0 ? (
              <div className="space-y-3">
                {stats.queues.map((queue) => (
                  <div key={queue.name} className="flex items-center gap-3">
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-medium text-slate-900 dark:text-white">{queue.name}</span>
                        <span className="text-sm text-slate-500">{queue.total} total</span>
                      </div>
                      <div className="h-2 bg-slate-100 dark:bg-slate-700 rounded-full overflow-hidden flex">
                        {queue.completed > 0 && (
                          <div
                            className="h-full bg-green-500"
                            style={{ width: `${(queue.completed / Math.max(queue.total, 1)) * 100}%` }}
                          />
                        )}
                        {queue.running > 0 && (
                          <div
                            className="h-full bg-yellow-500"
                            style={{ width: `${(queue.running / Math.max(queue.total, 1)) * 100}%` }}
                          />
                        )}
                        {queue.failed > 0 && (
                          <div
                            className="h-full bg-red-500"
                            style={{ width: `${(queue.failed / Math.max(queue.total, 1)) * 100}%` }}
                          />
                        )}
                        {queue.available > 0 && (
                          <div
                            className="h-full bg-blue-500"
                            style={{ width: `${(queue.available / Math.max(queue.total, 1)) * 100}%` }}
                          />
                        )}
                      </div>
                    </div>
                  </div>
                ))}
                <div className="flex items-center gap-4 text-xs text-slate-500 mt-2 pt-2 border-t border-slate-100 dark:border-slate-700">
                  <span className="flex items-center gap-1"><span className="w-2 h-2 bg-green-500 rounded-full" /> Completed</span>
                  <span className="flex items-center gap-1"><span className="w-2 h-2 bg-yellow-500 rounded-full" /> Running</span>
                  <span className="flex items-center gap-1"><span className="w-2 h-2 bg-red-500 rounded-full" /> Failed</span>
                  <span className="flex items-center gap-1"><span className="w-2 h-2 bg-blue-500 rounded-full" /> Available</span>
                </div>
              </div>
            ) : (
              <div className="text-center py-6 text-slate-500">
                <Layers className="w-10 h-10 mx-auto mb-2 text-slate-300" />
                <p>No queues yet</p>
              </div>
            )}
          </div>
        </div>

        {/* Workers Status */}
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white flex items-center gap-2">
              <Server className="w-5 h-5 text-green-500" />
              Workers
            </h2>
            <Link to="/queues" className="text-sm text-primary-600 hover:text-primary-700 flex items-center gap-1">
              Manage <ChevronRight className="w-4 h-4" />
            </Link>
          </div>
          <div className="p-4">
            {workers && workers.length > 0 ? (
              <div className="space-y-3">
                {workers.slice(0, 4).map((worker) => {
                  const health = workerHealth?.find(h => h.id === worker.id)
                  const isHealthy = health?.is_healthy ?? false
                  return (
                    <div key={worker.id} className="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-900 rounded-lg">
                      <div className="flex items-center gap-3">
                        <div className={`p-1.5 rounded-full ${isHealthy ? 'bg-green-100 dark:bg-green-900/30' : 'bg-red-100 dark:bg-red-900/30'}`}>
                          {isHealthy ? <Heart className="w-4 h-4 text-green-600" /> : <HeartOff className="w-4 h-4 text-red-600" />}
                        </div>
                        <div>
                          <p className="font-mono text-sm text-slate-900 dark:text-white">
                            {worker.id.slice(0, 8)}...
                          </p>
                          <p className="text-xs text-slate-500">{worker.queues.join(', ')}</p>
                        </div>
                      </div>
                      <div className="text-right text-sm">
                        <p className="text-green-600">{worker.jobs_processed} processed</p>
                        {worker.jobs_failed > 0 && <p className="text-red-600">{worker.jobs_failed} failed</p>}
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="text-center py-6 text-slate-500">
                <Server className="w-10 h-10 mx-auto mb-2 text-slate-300" />
                <p>No workers connected</p>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Workflows Overview */}
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white flex items-center gap-2">
              <GitBranch className="w-5 h-5 text-primary-500" />
              Workflows
            </h2>
            <Link to="/workflows" className="text-sm text-primary-600 hover:text-primary-700 flex items-center gap-1">
              View all <ChevronRight className="w-4 h-4" />
            </Link>
          </div>
          <div className="p-4">
            {/* Workflow Stats */}
            <div className="grid grid-cols-3 gap-3 mb-4">
              <div className="text-center p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
                <p className="text-2xl font-bold text-yellow-600">{runningWorkflows}</p>
                <p className="text-xs text-slate-500">Running</p>
              </div>
              <div className="text-center p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
                <p className="text-2xl font-bold text-green-600">{completedWorkflows}</p>
                <p className="text-xs text-slate-500">Completed</p>
              </div>
              <div className="text-center p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
                <p className="text-2xl font-bold text-red-600">{failedWorkflows}</p>
                <p className="text-xs text-slate-500">Failed</p>
              </div>
            </div>

            {/* Registered Workflows */}
            {workflows && workflows.length > 0 ? (
              <div className="space-y-2">
                <p className="text-xs font-medium text-slate-500 uppercase tracking-wider">Registered Workflows</p>
                {workflows.map((name) => (
                  <Link
                    key={name}
                    to={`/workflows/${encodeURIComponent(name)}/runs`}
                    className="flex items-center justify-between p-2 hover:bg-slate-50 dark:hover:bg-slate-700/50 rounded-lg transition-colors"
                  >
                    <span className="flex items-center gap-2 text-sm text-slate-900 dark:text-white">
                      <GitBranch className="w-4 h-4 text-primary-500" />
                      {name}
                    </span>
                    <ChevronRight className="w-4 h-4 text-slate-400" />
                  </Link>
                ))}
              </div>
            ) : (
              <div className="text-center py-4 text-slate-500">
                <p className="text-sm">No workflows registered</p>
              </div>
            )}
          </div>
        </div>

        {/* Recent Workflow Runs */}
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white flex items-center gap-2">
              <TrendingUp className="w-5 h-5 text-indigo-500" />
              Recent Activity
            </h2>
            <Link to="/workflow-runs" className="text-sm text-primary-600 hover:text-primary-700 flex items-center gap-1">
              View all <ChevronRight className="w-4 h-4" />
            </Link>
          </div>
          <div className="divide-y divide-slate-100 dark:divide-slate-700">
            {workflowRuns && workflowRuns.length > 0 ? (
              workflowRuns.map((run) => (
                <Link
                  key={run.id}
                  to={`/workflow-runs/${run.id}`}
                  className="flex items-center justify-between px-4 py-3 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <WorkflowStateIcon state={run.state} />
                    <div>
                      <p className="font-medium text-slate-900 dark:text-white">{run.workflow_name}</p>
                      <p className="text-xs text-slate-500">
                        {run.started_at ? formatDistanceToNow(new Date(run.started_at), { addSuffix: true }) : 'Pending'}
                      </p>
                    </div>
                  </div>
                  <WorkflowStateBadge state={run.state} />
                </Link>
              ))
            ) : (
              <div className="p-6 text-center text-slate-500">
                <Clock className="w-10 h-10 mx-auto mb-2 text-slate-300" />
                <p>No recent workflow runs</p>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Empty state when no data */}
      {(!stats || stats.total === 0) && (!workflowRuns || workflowRuns.length === 0) && (
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-8 text-center mt-6">
          <AlertCircle className="w-12 h-12 text-slate-400 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-slate-900 dark:text-white mb-2">
            Welcome to Rivo
          </h3>
          <p className="text-slate-500 dark:text-slate-400 max-w-md mx-auto">
            Start by registering handlers and enqueueing jobs, or define workflows for complex multi-step processes.
          </p>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, icon, color }: { label: string; value: number | string; icon: React.ReactNode; color: string }) {
  return (
    <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
      <div className={`inline-flex p-2 rounded-lg ${color} mb-2`}>
        {icon}
      </div>
      <div className="text-2xl font-bold text-slate-900 dark:text-white">
        {typeof value === 'number' ? value.toLocaleString() : value}
      </div>
      <div className="text-sm text-slate-500 dark:text-slate-400">
        {label}
      </div>
    </div>
  )
}

function WorkflowStateIcon({ state }: { state: WorkflowRun['state'] }) {
  switch (state) {
    case 'completed':
      return <CheckCircle2 className="w-5 h-5 text-green-500" />
    case 'failed':
      return <XCircle className="w-5 h-5 text-red-500" />
    case 'running':
      return <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />
    case 'cancelled':
      return <XCircle className="w-5 h-5 text-slate-400" />
    default:
      return <Clock className="w-5 h-5 text-yellow-500" />
  }
}

function WorkflowStateBadge({ state }: { state: WorkflowRun['state'] }) {
  const colors = {
    pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300',
    running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300',
    completed: 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300',
    failed: 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300',
    cancelled: 'bg-slate-100 text-slate-800 dark:bg-slate-700 dark:text-slate-300',
  }
  return (
    <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${colors[state]}`}>
      {state}
    </span>
  )
}
