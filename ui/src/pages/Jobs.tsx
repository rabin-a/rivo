import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, Job, JobLog } from '../api/client'
import { formatDistanceToNow, format } from 'date-fns'
import {
  RefreshCw,
  XCircle,
  ChevronDown,
  Plus,
  Eye,
  X,
  Clock,
  CheckCircle,
  AlertCircle,
  Loader2
} from 'lucide-react'

const STATE_COLORS: Record<string, string> = {
  available: 'bg-blue-100 text-blue-700',
  scheduled: 'bg-purple-100 text-purple-700',
  running: 'bg-yellow-100 text-yellow-700',
  retryable: 'bg-orange-100 text-orange-700',
  completed: 'bg-green-100 text-green-700',
  cancelled: 'bg-slate-100 text-slate-700',
  discarded: 'bg-red-100 text-red-700',
}

const LOG_STATUS_ICONS: Record<string, React.ReactNode> = {
  running: <Loader2 className="w-4 h-4 text-yellow-500 animate-spin" />,
  completed: <CheckCircle className="w-4 h-4 text-green-500" />,
  failed: <AlertCircle className="w-4 h-4 text-red-500" />,
}

export default function Jobs() {
  const [stateFilter, setStateFilter] = useState<string>('')
  const [showEnqueueModal, setShowEnqueueModal] = useState(false)
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const queryClient = useQueryClient()

  const { data: jobs, isLoading } = useQuery({
    queryKey: ['jobs', stateFilter],
    queryFn: () => api.getJobs({ state: stateFilter || undefined, limit: 100 }),
  })

  const cancelMutation = useMutation({
    mutationFn: (id: number) => api.cancelJob(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })

  const retryMutation = useMutation({
    mutationFn: (id: number) => api.retryJob(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
          Jobs
        </h1>
        <div className="flex items-center gap-3">
          {/* State Filter */}
          <div className="relative">
            <select
              value={stateFilter}
              onChange={(e) => setStateFilter(e.target.value)}
              className="appearance-none bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg pl-3 pr-10 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">All States</option>
              <option value="available">Available</option>
              <option value="scheduled">Scheduled</option>
              <option value="running">Running</option>
              <option value="retryable">Retryable</option>
              <option value="completed">Completed</option>
              <option value="cancelled">Cancelled</option>
              <option value="discarded">Discarded</option>
            </select>
            <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400 pointer-events-none" />
          </div>

          {/* Enqueue Button */}
          <button
            onClick={() => setShowEnqueueModal(true)}
            className="flex items-center gap-2 bg-primary-600 hover:bg-primary-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors"
          >
            <Plus className="w-4 h-4" />
            Enqueue Job
          </button>
        </div>
      </div>

      {/* Jobs Table */}
      <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-50 dark:bg-slate-900/50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                ID
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                Kind
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                Queue
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                State
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                Attempt
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                Created
              </th>
              <th className="px-4 py-3 text-right text-xs font-medium text-slate-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
            {isLoading ? (
              [...Array(5)].map((_, i) => (
                <tr key={i}>
                  <td colSpan={7} className="px-4 py-3">
                    <div className="h-4 bg-slate-200 rounded animate-pulse"></div>
                  </td>
                </tr>
              ))
            ) : jobs && jobs.length > 0 ? (
              jobs.map((job) => (
                <JobRow
                  key={job.id}
                  job={job}
                  onCancel={() => cancelMutation.mutate(job.id)}
                  onRetry={() => retryMutation.mutate(job.id)}
                  onView={() => setSelectedJob(job)}
                />
              ))
            ) : (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-slate-500">
                  No jobs found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Enqueue Modal */}
      {showEnqueueModal && (
        <EnqueueModal onClose={() => setShowEnqueueModal(false)} />
      )}

      {/* Job Detail Modal */}
      {selectedJob && (
        <JobDetailModal job={selectedJob} onClose={() => setSelectedJob(null)} />
      )}
    </div>
  )
}

function JobRow({
  job,
  onCancel,
  onRetry,
  onView,
}: {
  job: Job
  onCancel: () => void
  onRetry: () => void
  onView: () => void
}) {
  const canCancel = ['available', 'scheduled', 'running', 'retryable'].includes(job.state)
  const canRetry = ['cancelled', 'discarded'].includes(job.state)

  return (
    <tr className="hover:bg-slate-50 dark:hover:bg-slate-700/50 cursor-pointer" onClick={onView}>
      <td className="px-4 py-3 text-sm font-mono text-slate-600 dark:text-slate-300">
        {job.id}
      </td>
      <td className="px-4 py-3 text-sm font-medium text-slate-900 dark:text-white">
        {job.kind}
      </td>
      <td className="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
        {job.queue}
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${STATE_COLORS[job.state]}`}>
          {job.state}
        </span>
      </td>
      <td className="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
        {job.attempt}/{job.max_attempts}
      </td>
      <td className="px-4 py-3 text-sm text-slate-500">
        {formatDistanceToNow(new Date(job.created_at), { addSuffix: true })}
      </td>
      <td className="px-4 py-3 text-right">
        <div className="flex items-center justify-end gap-2" onClick={(e) => e.stopPropagation()}>
          <button
            onClick={onView}
            className="p-1 text-slate-400 hover:text-primary-600 transition-colors"
            title="View Details"
          >
            <Eye className="w-4 h-4" />
          </button>
          {canRetry && (
            <button
              onClick={onRetry}
              className="p-1 text-slate-400 hover:text-primary-600 transition-colors"
              title="Retry"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
          )}
          {canCancel && (
            <button
              onClick={onCancel}
              className="p-1 text-slate-400 hover:text-red-600 transition-colors"
              title="Cancel"
            >
              <XCircle className="w-4 h-4" />
            </button>
          )}
        </div>
      </td>
    </tr>
  )
}

function JobDetailModal({ job, onClose }: { job: Job; onClose: () => void }) {
  const { data: logs, isLoading } = useQuery({
    queryKey: ['job-logs', job.id],
    queryFn: () => api.getJobLogs(job.id),
    refetchInterval: job.state === 'running' ? 1000 : false,
  })

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
              Job #{job.id}
            </h2>
            <p className="text-sm text-slate-500">{job.kind}</p>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {/* Job Info */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">State</label>
              <span className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${STATE_COLORS[job.state]}`}>
                {job.state}
              </span>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Queue</label>
              <p className="text-sm text-slate-900 dark:text-white">{job.queue}</p>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Attempts</label>
              <p className="text-sm text-slate-900 dark:text-white">{job.attempt}/{job.max_attempts}</p>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Priority</label>
              <p className="text-sm text-slate-900 dark:text-white">{job.priority}</p>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Created</label>
              <p className="text-sm text-slate-900 dark:text-white">
                {format(new Date(job.created_at), 'PPpp')}
              </p>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Scheduled</label>
              <p className="text-sm text-slate-900 dark:text-white">
                {format(new Date(job.scheduled_at), 'PPpp')}
              </p>
            </div>
            {job.attempted_at && (
              <div>
                <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Last Attempt</label>
                <p className="text-sm text-slate-900 dark:text-white">
                  {format(new Date(job.attempted_at), 'PPpp')}
                </p>
              </div>
            )}
            {job.completed_at && (
              <div>
                <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Completed</label>
                <p className="text-sm text-slate-900 dark:text-white">
                  {format(new Date(job.completed_at), 'PPpp')}
                </p>
              </div>
            )}
          </div>

          {/* Payload */}
          <div>
            <label className="block text-xs font-medium text-slate-500 uppercase mb-2">Payload</label>
            <pre className="bg-slate-100 dark:bg-slate-900 rounded-lg p-4 text-sm font-mono overflow-x-auto text-slate-800 dark:text-slate-200">
              {JSON.stringify(job.payload, null, 2)}
            </pre>
          </div>

          {/* Execution Logs */}
          <div>
            <label className="block text-xs font-medium text-slate-500 uppercase mb-2">
              Execution History
            </label>
            {isLoading ? (
              <div className="flex items-center justify-center py-8 text-slate-500">
                <Loader2 className="w-5 h-5 animate-spin mr-2" />
                Loading logs...
              </div>
            ) : logs && logs.length > 0 ? (
              <div className="space-y-3">
                {logs.map((log) => (
                  <JobLogCard key={log.id} log={log} />
                ))}
              </div>
            ) : (
              <p className="text-sm text-slate-500 py-4 text-center bg-slate-50 dark:bg-slate-900 rounded-lg">
                No execution logs yet
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function JobLogCard({ log }: { log: JobLog }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="bg-slate-50 dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
      >
        <div className="flex items-center gap-3">
          {LOG_STATUS_ICONS[log.status]}
          <span className="text-sm font-medium text-slate-900 dark:text-white">
            Attempt {log.attempt}
          </span>
          <span className="text-sm text-slate-500">
            on worker {log.worker_id.slice(0, 8)}...
          </span>
        </div>
        <div className="flex items-center gap-3">
          {log.duration_ms !== undefined && (
            <span className="flex items-center gap-1 text-sm text-slate-500">
              <Clock className="w-3 h-3" />
              {log.duration_ms}ms
            </span>
          )}
          <ChevronDown className={`w-4 h-4 text-slate-400 transition-transform ${expanded ? 'rotate-180' : ''}`} />
        </div>
      </button>

      {expanded && (
        <div className="px-4 py-3 border-t border-slate-200 dark:border-slate-700 space-y-3">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Started</label>
              <p className="text-slate-900 dark:text-white">
                {format(new Date(log.started_at), 'PPpp')}
              </p>
            </div>
            {log.completed_at && (
              <div>
                <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Completed</label>
                <p className="text-slate-900 dark:text-white">
                  {format(new Date(log.completed_at), 'PPpp')}
                </p>
              </div>
            )}
          </div>

          {log.input && Object.keys(log.input).length > 0 && (
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Input</label>
              <pre className="bg-slate-100 dark:bg-slate-800 rounded p-2 text-xs font-mono overflow-x-auto">
                {JSON.stringify(log.input, null, 2)}
              </pre>
            </div>
          )}

          {log.output && Object.keys(log.output).length > 0 && (
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Output</label>
              <pre className="bg-slate-100 dark:bg-slate-800 rounded p-2 text-xs font-mono overflow-x-auto">
                {JSON.stringify(log.output, null, 2)}
              </pre>
            </div>
          )}

          {log.error && (
            <div>
              <label className="block text-xs font-medium text-red-500 uppercase mb-1">Error</label>
              <pre className="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 rounded p-2 text-xs font-mono overflow-x-auto">
                {log.error}
              </pre>
              {log.error_stack && (
                <pre className="mt-2 bg-slate-100 dark:bg-slate-800 rounded p-2 text-xs font-mono overflow-x-auto text-slate-600 dark:text-slate-400">
                  {log.error_stack}
                </pre>
              )}
            </div>
          )}

          {log.logs && log.logs.length > 0 && (
            <div>
              <label className="block text-xs font-medium text-slate-500 uppercase mb-1">Logs</label>
              <div className="bg-slate-900 rounded p-2 text-xs font-mono overflow-x-auto max-h-48">
                {log.logs.map((entry, i) => (
                  <div key={i} className={`py-0.5 ${entry.level === 'error' ? 'text-red-400' : entry.level === 'warn' ? 'text-yellow-400' : 'text-slate-300'}`}>
                    <span className="text-slate-500">{entry.timestamp}</span>{' '}
                    <span className="uppercase text-xs">[{entry.level}]</span>{' '}
                    {entry.message}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function EnqueueModal({ onClose }: { onClose: () => void }) {
  const [kind, setKind] = useState('')
  const [payload, setPayload] = useState('{}')
  const [queue, setQueue] = useState('default')
  const queryClient = useQueryClient()

  const enqueueMutation = useMutation({
    mutationFn: () => api.enqueueJob({
      kind,
      payload: JSON.parse(payload),
      queue,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      onClose()
    },
  })

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-md mx-4">
        <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
            Enqueue Job
          </h2>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault()
            enqueueMutation.mutate()
          }}
          className="p-6 space-y-4"
        >
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
              Kind
            </label>
            <input
              type="text"
              value={kind}
              onChange={(e) => setKind(e.target.value)}
              className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
              placeholder="send-email"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
              Queue
            </label>
            <input
              type="text"
              value={queue}
              onChange={(e) => setQueue(e.target.value)}
              className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
              Payload (JSON)
            </label>
            <textarea
              value={payload}
              onChange={(e) => setPayload(e.target.value)}
              rows={4}
              className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900 text-slate-900 dark:text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={enqueueMutation.isPending}
              className="px-4 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {enqueueMutation.isPending ? 'Enqueueing...' : 'Enqueue'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
