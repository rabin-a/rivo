import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import {
  GitBranch,
  Play,
  RefreshCw,
  ChevronRight,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  AlertCircle
} from 'lucide-react'
import { api, WorkflowRun } from '../api/client'

interface WorkflowsProps {
  /** For embedded mode: callback when navigating to runs */
  onNavigateToRuns?: (workflowName: string) => void
}

export default function Workflows({ onNavigateToRuns }: WorkflowsProps = {}) {
  const [workflows, setWorkflows] = useState<string[]>([])
  const [runs, setRuns] = useState<WorkflowRun[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [startingWorkflow, setStartingWorkflow] = useState<string | null>(null)
  const [showStartModal, setShowStartModal] = useState(false)
  const [selectedWorkflow, setSelectedWorkflow] = useState<string | null>(null)
  const [inputJson, setInputJson] = useState('{}')

  const fetchData = async () => {
    try {
      const [workflowsData, runsData] = await Promise.all([
        api.getWorkflows(),
        api.getWorkflowRuns({ limit: 20 })
      ])
      setWorkflows(workflowsData)
      setRuns(runsData)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleStartWorkflow = async () => {
    if (!selectedWorkflow) return

    try {
      setStartingWorkflow(selectedWorkflow)
      const input = JSON.parse(inputJson)
      await api.startWorkflow(selectedWorkflow, input)
      setShowStartModal(false)
      setInputJson('{}')
      fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to start workflow')
    } finally {
      setStartingWorkflow(null)
    }
  }

  const openStartModal = (workflowName: string) => {
    setSelectedWorkflow(workflowName)
    // Set default input based on workflow name
    if (workflowName === 'order-processing') {
      setInputJson(JSON.stringify({
        order_id: `ORD-${Date.now()}`,
        customer: "John Doe",
        amount: 99.99
      }, null, 2))
    } else if (workflowName === 'data-pipeline') {
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

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-slate-400" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="bg-red-50 dark:bg-red-900/20 rounded-lg p-4 text-red-600 dark:text-red-400">
          {error}
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
          Workflows
        </h1>
        <button
          onClick={fetchData}
          className="flex items-center gap-2 px-3 py-2 text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
        >
          <RefreshCw className="w-4 h-4" />
          Refresh
        </button>
      </div>

      {/* Registered Workflows */}
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4">
          Registered Workflows
        </h2>

        {workflows.length === 0 ? (
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-8 text-center">
            <AlertCircle className="w-12 h-12 text-slate-400 mx-auto mb-4" />
            <p className="text-slate-500 dark:text-slate-400">
              No workflows registered. Register a workflow using client.RegisterWorkflow().
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
            <table className="w-full">
              <thead className="bg-slate-50 dark:bg-slate-900/50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Workflow
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Recent Runs
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
                {workflows.map((name) => {
                  const workflowRuns = runs.filter(r => r.workflow_name === name)
                  const completedCount = workflowRuns.filter(r => r.state === 'completed').length
                  const failedCount = workflowRuns.filter(r => r.state === 'failed').length
                  const runningCount = workflowRuns.filter(r => r.state === 'running').length

                  return (
                    <tr
                      key={name}
                      className="hover:bg-slate-50 dark:hover:bg-slate-700/50 cursor-pointer"
                      onClick={() => {
                        if (onNavigateToRuns) {
                          onNavigateToRuns(name)
                        } else {
                          window.location.href = `/workflows/${encodeURIComponent(name)}/runs`
                        }
                      }}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <GitBranch className="w-5 h-5 text-primary-600" />
                          <span className="font-medium text-slate-900 dark:text-white">{name}</span>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-3 text-sm">
                          {runningCount > 0 && (
                            <span className="flex items-center gap-1 text-blue-600">
                              <Loader2 className="w-3 h-3 animate-spin" />
                              {runningCount} running
                            </span>
                          )}
                          {completedCount > 0 && (
                            <span className="flex items-center gap-1 text-green-600">
                              <CheckCircle2 className="w-3 h-3" />
                              {completedCount} completed
                            </span>
                          )}
                          {failedCount > 0 && (
                            <span className="flex items-center gap-1 text-red-600">
                              <XCircle className="w-3 h-3" />
                              {failedCount} failed
                            </span>
                          )}
                          {workflowRuns.length === 0 && (
                            <span className="text-slate-400">No runs yet</span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            openStartModal(name)
                          }}
                          disabled={startingWorkflow === name}
                          className="inline-flex items-center gap-1 px-3 py-1.5 text-sm bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50"
                        >
                          {startingWorkflow === name ? (
                            <Loader2 className="w-4 h-4 animate-spin" />
                          ) : (
                            <Play className="w-4 h-4" />
                          )}
                          Start
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Recent Runs */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
            Recent Executions
          </h2>
          <Link
            to="/workflow-runs"
            className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400"
          >
            View All
          </Link>
        </div>

        {runs.length === 0 ? (
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-8 text-center">
            <Clock className="w-12 h-12 text-slate-400 mx-auto mb-4" />
            <p className="text-slate-500 dark:text-slate-400">
              No workflow executions yet. Start a workflow to see runs here.
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
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                    Workflow
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase">
                    State
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
                    onClick={() => window.location.href = `/workflow-runs/${run.id}`}
                  >
                    <td className="px-4 py-3 text-sm font-mono text-slate-900 dark:text-white">
                      #{run.id}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <GitBranch className="w-4 h-4 text-slate-400" />
                        <span className="text-sm text-slate-900 dark:text-white">
                          {run.workflow_name}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        {getStateIcon(run.state)}
                        {getStateBadge(run.state)}
                      </div>
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
      </div>

      {/* Start Workflow Modal */}
      {showStartModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
            <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-white">
                Start Workflow: {selectedWorkflow}
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
                disabled={startingWorkflow !== null}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50"
              >
                {startingWorkflow ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Play className="w-4 h-4" />
                )}
                Start Workflow
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
