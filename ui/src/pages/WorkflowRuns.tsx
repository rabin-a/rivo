import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  GitBranch,
  ArrowLeft,
  RefreshCw,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  ChevronRight,
  Play
} from 'lucide-react'
import { api, WorkflowRun } from '../api/client'

interface WorkflowRunsProps {
  /** For embedded mode: workflow name to filter by */
  embeddedWorkflowName?: string
  /** For embedded mode: callback when navigating to detail */
  onNavigateToDetail?: (id: number) => void
}

export default function WorkflowRuns({ embeddedWorkflowName, onNavigateToDetail }: WorkflowRunsProps = {}) {
  const params = useParams<{ name?: string }>()
  const name = embeddedWorkflowName ?? params.name
  const [runs, setRuns] = useState<WorkflowRun[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)
  const [showStartModal, setShowStartModal] = useState(false)
  const [inputJson, setInputJson] = useState('{}')

  const fetchRuns = async () => {
    try {
      const data = await api.getWorkflowRuns({ limit: 100 })
      // Filter by workflow name if provided
      const filtered = name
        ? data.filter(r => r.workflow_name === name)
        : data
      setRuns(filtered)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch runs')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchRuns()
    const interval = setInterval(fetchRuns, 3000)
    return () => clearInterval(interval)
  }, [name])

  const handleStartWorkflow = async () => {
    if (!name) return
    try {
      setStarting(true)
      const input = JSON.parse(inputJson)
      await api.startWorkflow(name, input)
      setShowStartModal(false)
      fetchRuns()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to start workflow')
    } finally {
      setStarting(false)
    }
  }

  const openStartModal = () => {
    if (name === 'order-processing') {
      setInputJson(JSON.stringify({
        order_id: `ORD-${Date.now()}`,
        customer: "John Doe",
        amount: 99.99
      }, null, 2))
    } else if (name === 'data-pipeline') {
      setInputJson(JSON.stringify({
        source: "database",
        destination: "warehouse"
      }, null, 2))
    } else {
      setInputJson('{}')
    }
    setShowStartModal(true)
  }

  const getStateIcon = (state: WorkflowRun['state']) => {
    switch (state) {
      case 'completed':
        return <CheckCircle2 className="w-4 h-4 text-green-500" />
      case 'failed':
        return <XCircle className="w-4 h-4 text-red-500" />
      case 'running':
        return <Loader2 className="w-4 h-4 text-blue-500 animate-spin" />
      case 'cancelled':
        return <XCircle className="w-4 h-4 text-slate-400" />
      default:
        return <Clock className="w-4 h-4 text-yellow-500" />
    }
  }

  const getStateBadge = (state: WorkflowRun['state']) => {
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

  const formatTime = (time?: string) => {
    if (!time) return '-'
    return new Date(time).toLocaleString()
  }

  const formatDuration = (start?: string, end?: string) => {
    if (!start) return '-'
    const startTime = new Date(start).getTime()
    const endTime = end ? new Date(end).getTime() : Date.now()
    const diff = endTime - startTime
    if (diff < 1000) return `${diff}ms`
    if (diff < 60000) return `${(diff / 1000).toFixed(1)}s`
    return `${Math.floor(diff / 60000)}m ${Math.floor((diff % 60000) / 1000)}s`
  }

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-slate-400" />
      </div>
    )
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="mb-6">
        <Link
          to="/workflows"
          className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 mb-4"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Workflows
        </Link>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <GitBranch className="w-6 h-6 text-primary-600" />
            <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
              {name ? `${name} Runs` : 'All Workflow Runs'}
            </h1>
          </div>

          <div className="flex items-center gap-2">
            {name && (
              <button
                onClick={openStartModal}
                className="flex items-center gap-2 px-3 py-2 text-sm bg-primary-600 text-white rounded-md hover:bg-primary-700"
              >
                <Play className="w-4 h-4" />
                Start New Run
              </button>
            )}
            <button
              onClick={fetchRuns}
              className="flex items-center gap-2 px-3 py-2 text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            >
              <RefreshCw className="w-4 h-4" />
              Refresh
            </button>
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-4 bg-red-50 dark:bg-red-900/20 rounded-lg p-4 text-red-600 dark:text-red-400">
          {error}
        </div>
      )}

      {runs.length === 0 ? (
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-8 text-center">
          <Clock className="w-12 h-12 text-slate-400 mx-auto mb-4" />
          <p className="text-slate-500 dark:text-slate-400">
            No workflow runs found. {name && 'Click "Start New Run" to execute this workflow.'}
          </p>
        </div>
      ) : (
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full">
            <thead className="bg-slate-50 dark:bg-slate-700/50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                  ID
                </th>
                {!name && (
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                    Workflow
                  </th>
                )}
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                  State
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                  Duration
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                  Started
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                  Completed
                </th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {runs.map((run) => (
                <tr
                  key={run.id}
                  className="hover:bg-slate-50 dark:hover:bg-slate-700/30 cursor-pointer"
                  onClick={() => {
                    if (onNavigateToDetail) {
                      onNavigateToDetail(run.id)
                    } else {
                      window.location.href = `/workflow-runs/${run.id}`
                    }
                  }}
                >
                  <td className="px-4 py-3 text-sm font-mono text-slate-900 dark:text-white">
                    #{run.id}
                  </td>
                  {!name && (
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2 text-sm text-primary-600">
                        <GitBranch className="w-4 h-4" />
                        {run.workflow_name}
                      </div>
                    </td>
                  )}
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      {getStateIcon(run.state)}
                      {getStateBadge(run.state)}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-sm text-slate-500 dark:text-slate-400 font-mono">
                    {formatDuration(run.started_at, run.completed_at)}
                  </td>
                  <td className="px-4 py-3 text-sm text-slate-500 dark:text-slate-400">
                    {formatTime(run.started_at)}
                  </td>
                  <td className="px-4 py-3 text-sm text-slate-500 dark:text-slate-400">
                    {formatTime(run.completed_at)}
                  </td>
                  <td className="px-4 py-3">
                    <ChevronRight className="w-5 h-5 text-slate-400" />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Start Workflow Modal */}
      {showStartModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
            <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-white">
                Start Workflow: {name}
              </h3>
            </div>
            <div className="p-6">
              <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">
                Input (JSON)
              </label>
              <textarea
                value={inputJson}
                onChange={(e) => setInputJson(e.target.value)}
                rows={8}
                className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white font-mono text-sm"
              />
            </div>
            <div className="px-6 py-4 border-t border-slate-200 dark:border-slate-700 flex justify-end gap-3">
              <button
                onClick={() => setShowStartModal(false)}
                className="px-4 py-2 text-sm text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white"
              >
                Cancel
              </button>
              <button
                onClick={handleStartWorkflow}
                disabled={starting}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50"
              >
                {starting ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Play className="w-4 h-4" />
                )}
                Start
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
