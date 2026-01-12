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
  ChevronDown,
  ChevronRight,
  Ban,
  RotateCcw
} from 'lucide-react'
import { api, WorkflowRun, StepRunState, WorkflowStepLog } from '../api/client'

interface WorkflowRunDetailProps {
  /** For embedded mode: workflow run ID */
  embeddedRunId?: number
}

export default function WorkflowRunDetail({ embeddedRunId }: WorkflowRunDetailProps = {}) {
  const params = useParams<{ id: string }>()
  const id = embeddedRunId?.toString() ?? params.id
  const [run, setRun] = useState<WorkflowRun | null>(null)
  const [stepLogs, setStepLogs] = useState<WorkflowStepLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expandedSteps, setExpandedSteps] = useState<Set<string>>(new Set())
  const [cancelling, setCancelling] = useState(false)
  const [rerunningStep, setRerunningStep] = useState<string | null>(null)

  const fetchRun = async () => {
    if (!id) return
    try {
      const [runData, logsData] = await Promise.all([
        api.getWorkflowRun(parseInt(id)),
        api.getWorkflowStepLogs(parseInt(id)).catch(() => [])
      ])
      setRun(runData)
      setStepLogs(logsData)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch workflow run')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchRun()
    const interval = setInterval(fetchRun, 2000)
    return () => clearInterval(interval)
  }, [id])

  const handleRerunStep = async (stepId: string) => {
    if (!run || rerunningStep) return
    if (!confirm(`Rerun step "${stepId}" and all subsequent steps?`)) return

    try {
      setRerunningStep(stepId)
      await api.rerunWorkflowStep(run.id, stepId)
      fetchRun()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to rerun step')
    } finally {
      setRerunningStep(null)
    }
  }

  // Get the latest log for a step
  const getStepLog = (stepId: string): WorkflowStepLog | undefined => {
    return stepLogs.find(log => log.step_id === stepId)
  }

  const handleCancel = async () => {
    if (!run || cancelling) return
    if (!confirm('Are you sure you want to cancel this workflow run?')) return

    try {
      setCancelling(true)
      await api.cancelWorkflowRun(run.id)
      fetchRun()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to cancel workflow run')
    } finally {
      setCancelling(false)
    }
  }

  const toggleStep = (stepId: string) => {
    setExpandedSteps(prev => {
      const next = new Set(prev)
      if (next.has(stepId)) {
        next.delete(stepId)
      } else {
        next.add(stepId)
      }
      return next
    })
  }

  const getStateIcon = (state: StepRunState['state'] | WorkflowRun['state']) => {
    switch (state) {
      case 'completed':
        return <CheckCircle2 className="w-5 h-5 text-green-500" />
      case 'failed':
        return <XCircle className="w-5 h-5 text-red-500" />
      case 'running':
        return <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />
      case 'skipped':
        return <Ban className="w-5 h-5 text-slate-400" />
      case 'cancelled':
        return <XCircle className="w-5 h-5 text-slate-400" />
      default:
        return <Clock className="w-5 h-5 text-yellow-500" />
    }
  }

  const getStateBadge = (state: StepRunState['state'] | WorkflowRun['state']) => {
    const colors: Record<string, string> = {
      pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300',
      running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300',
      completed: 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300',
      failed: 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300',
      skipped: 'bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-400',
      cancelled: 'bg-slate-100 text-slate-800 dark:bg-slate-700 dark:text-slate-300',
    }
    return (
      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${colors[state] || colors.pending}`}>
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

  if (error || !run) {
    return (
      <div className="p-8">
        <div className="bg-red-50 dark:bg-red-900/20 rounded-lg p-4 text-red-600 dark:text-red-400">
          {error || 'Workflow run not found'}
        </div>
      </div>
    )
  }

  const stepEntries = Object.entries(run.step_states || {})

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
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <GitBranch className="w-6 h-6 text-primary-600" />
              <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
                {run.workflow_name}
              </h1>
            </div>
            <span className="text-slate-500 dark:text-slate-400">
              Run #{run.id}
            </span>
            {getStateBadge(run.state)}
          </div>

          <div className="flex items-center gap-2">
            {(run.state === 'running' || run.state === 'pending') && (
              <button
                onClick={handleCancel}
                disabled={cancelling}
                className="flex items-center gap-2 px-3 py-2 text-sm text-red-600 hover:text-red-700 border border-red-200 dark:border-red-800 rounded-md hover:bg-red-50 dark:hover:bg-red-900/30"
              >
                {cancelling ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Ban className="w-4 h-4" />
                )}
                Cancel
              </button>
            )}
            <button
              onClick={fetchRun}
              className="flex items-center gap-2 px-3 py-2 text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            >
              <RefreshCw className="w-4 h-4" />
              Refresh
            </button>
          </div>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-4 gap-4 mb-8">
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
          <div className="text-sm text-slate-500 dark:text-slate-400 mb-1">Status</div>
          <div className="flex items-center gap-2">
            {getStateIcon(run.state)}
            <span className="font-semibold text-slate-900 dark:text-white capitalize">
              {run.state}
            </span>
          </div>
        </div>
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
          <div className="text-sm text-slate-500 dark:text-slate-400 mb-1">Duration</div>
          <div className="font-semibold text-slate-900 dark:text-white">
            {formatDuration(run.started_at, run.completed_at)}
          </div>
        </div>
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
          <div className="text-sm text-slate-500 dark:text-slate-400 mb-1">Started</div>
          <div className="font-semibold text-slate-900 dark:text-white text-sm">
            {formatTime(run.started_at)}
          </div>
        </div>
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
          <div className="text-sm text-slate-500 dark:text-slate-400 mb-1">Completed</div>
          <div className="font-semibold text-slate-900 dark:text-white text-sm">
            {formatTime(run.completed_at)}
          </div>
        </div>
      </div>

      {/* Error */}
      {run.error && (
        <div className="mb-8 bg-red-50 dark:bg-red-900/20 rounded-lg border border-red-200 dark:border-red-800 p-4">
          <h3 className="text-sm font-medium text-red-800 dark:text-red-300 mb-2">Error</h3>
          <pre className="text-sm text-red-700 dark:text-red-400 whitespace-pre-wrap font-mono">
            {run.error}
          </pre>
        </div>
      )}

      {/* Steps */}
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4">Steps</h2>
        <div className="space-y-2">
          {stepEntries.length === 0 ? (
            <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-8 text-center">
              <p className="text-slate-500 dark:text-slate-400">No step data available</p>
            </div>
          ) : (
            stepEntries.map(([stepId, stepState]) => {
              const stepLog = getStepLog(stepId)
              const canRerun = (run.state === 'failed' || run.state === 'cancelled') &&
                               (stepState.state === 'failed' || stepState.state === 'pending')

              return (
                <div
                  key={stepId}
                  className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden"
                >
                  <div className="w-full px-4 py-3 flex items-center justify-between hover:bg-slate-50 dark:hover:bg-slate-700/30">
                    <button
                      onClick={() => toggleStep(stepId)}
                      className="flex items-center gap-3 flex-1 text-left"
                    >
                      {getStateIcon(stepState.state)}
                      <span className="font-medium text-slate-900 dark:text-white">
                        {stepId}
                      </span>
                      {getStateBadge(stepState.state)}
                    </button>
                    <div className="flex items-center gap-4">
                      {stepLog?.duration_ms && (
                        <span className="text-sm text-slate-500 dark:text-slate-400">
                          {stepLog.duration_ms}ms
                        </span>
                      )}
                      {!stepLog?.duration_ms && stepState.started_at && (
                        <span className="text-sm text-slate-500 dark:text-slate-400">
                          {formatDuration(stepState.started_at, stepState.completed_at)}
                        </span>
                      )}
                      {canRerun && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            handleRerunStep(stepId)
                          }}
                          disabled={rerunningStep !== null}
                          className="flex items-center gap-1 px-2 py-1 text-xs text-blue-600 hover:text-blue-700 border border-blue-200 dark:border-blue-800 rounded hover:bg-blue-50 dark:hover:bg-blue-900/30"
                        >
                          {rerunningStep === stepId ? (
                            <Loader2 className="w-3 h-3 animate-spin" />
                          ) : (
                            <RotateCcw className="w-3 h-3" />
                          )}
                          Rerun
                        </button>
                      )}
                      <button onClick={() => toggleStep(stepId)}>
                        {expandedSteps.has(stepId) ? (
                          <ChevronDown className="w-5 h-5 text-slate-400" />
                        ) : (
                          <ChevronRight className="w-5 h-5 text-slate-400" />
                        )}
                      </button>
                    </div>
                  </div>

                  {expandedSteps.has(stepId) && (
                    <div className="px-4 py-3 border-t border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-700/30">
                      <div className="grid grid-cols-2 gap-4 text-sm mb-4">
                        <div>
                          <span className="text-slate-500 dark:text-slate-400">Started:</span>{' '}
                          <span className="text-slate-900 dark:text-white">
                            {formatTime(stepLog?.started_at || stepState.started_at)}
                          </span>
                        </div>
                        <div>
                          <span className="text-slate-500 dark:text-slate-400">Completed:</span>{' '}
                          <span className="text-slate-900 dark:text-white">
                            {formatTime(stepLog?.completed_at || stepState.completed_at)}
                          </span>
                        </div>
                        <div>
                          <span className="text-slate-500 dark:text-slate-400">Attempt:</span>{' '}
                          <span className="text-slate-900 dark:text-white">{stepLog?.attempt || stepState.attempt || 1}</span>
                        </div>
                        {stepLog?.duration_ms && (
                          <div>
                            <span className="text-slate-500 dark:text-slate-400">Duration:</span>{' '}
                            <span className="text-slate-900 dark:text-white">{stepLog.duration_ms}ms</span>
                          </div>
                        )}
                      </div>

                      {(stepState.error || stepLog?.error) && (
                        <div className="mb-4">
                          <h4 className="text-sm font-medium text-red-800 dark:text-red-300 mb-1">Error</h4>
                          <pre className="text-sm text-red-700 dark:text-red-400 bg-red-50 dark:bg-red-900/30 rounded p-2 whitespace-pre-wrap font-mono">
                            {stepLog?.error || stepState.error}
                          </pre>
                        </div>
                      )}

                      {stepLog?.input && Object.keys(stepLog.input).length > 0 && (
                        <div className="mb-4">
                          <h4 className="text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Input</h4>
                          <pre className="text-sm text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded p-2 overflow-x-auto font-mono max-h-40 overflow-y-auto">
                            {JSON.stringify(stepLog.input, null, 2)}
                          </pre>
                        </div>
                      )}

                      {(stepState.output || stepLog?.output) && Object.keys(stepState.output || stepLog?.output || {}).length > 0 && (
                        <div className="mb-4">
                          <h4 className="text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Output</h4>
                          <pre className="text-sm text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded p-2 overflow-x-auto font-mono max-h-40 overflow-y-auto">
                            {JSON.stringify(stepLog?.output || stepState.output, null, 2)}
                          </pre>
                        </div>
                      )}

                      {stepLog?.logs && stepLog.logs.length > 0 && (
                        <div>
                          <h4 className="text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Logs</h4>
                          <div className="bg-slate-900 rounded p-2 overflow-x-auto font-mono text-xs max-h-60 overflow-y-auto">
                            {stepLog.logs.map((log, i) => (
                              <div key={i} className="flex gap-2">
                                <span className="text-slate-500 shrink-0">
                                  {new Date(log.timestamp).toLocaleTimeString()}
                                </span>
                                <span className={`shrink-0 ${
                                  log.level === 'error' ? 'text-red-400' :
                                  log.level === 'warn' ? 'text-yellow-400' :
                                  log.level === 'debug' ? 'text-slate-500' :
                                  'text-green-400'
                                }`}>
                                  [{log.level.toUpperCase()}]
                                </span>
                                <span className="text-slate-300">{log.message}</span>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* Input/Output */}
      <div className="grid grid-cols-2 gap-6">
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4">Input</h2>
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
            <pre className="text-sm text-slate-600 dark:text-slate-400 overflow-x-auto font-mono">
              {JSON.stringify(run.input, null, 2)}
            </pre>
          </div>
        </div>
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4">Output</h2>
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
            <pre className="text-sm text-slate-600 dark:text-slate-400 overflow-x-auto font-mono">
              {run.output ? JSON.stringify(run.output, null, 2) : 'No output yet'}
            </pre>
          </div>
        </div>
      </div>
    </div>
  )
}
