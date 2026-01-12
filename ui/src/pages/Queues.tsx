import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, QueueConfig, QueueStats, Worker, WorkerHealth, Handler } from '../api/client'
import { formatDistanceToNow } from 'date-fns'
import {
  Server,
  Activity,
  Pause,
  Play,
  Settings,
  Heart,
  HeartOff,
  X,
  ChevronDown,
  ChevronUp,
  Users,
  Layers,
  Zap,
  Code
} from 'lucide-react'

export default function Queues() {
  const [expandedQueue, setExpandedQueue] = useState<string | null>(null)
  const [editingQueue, setEditingQueue] = useState<QueueConfig | null>(null)
  const [runningHandler, setRunningHandler] = useState<Handler | null>(null)

  const { data: queueStats } = useQuery({
    queryKey: ['queue-stats'],
    queryFn: () => api.getQueueStats(),
    refetchInterval: 5000,
  })

  const { data: queues } = useQuery({
    queryKey: ['queues'],
    queryFn: () => api.getQueues(),
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

  // Merge queue configs with stats
  const mergedQueues = (queueStats || []).map(stat => {
    const config = queues?.find(q => q.name === stat.name)
    return {
      ...stat,
      concurrency: config?.concurrency ?? 10,
      paused: config?.paused ?? false,
      priority: config?.priority ?? 0,
    }
  })

  const healthyWorkers = workerHealth?.filter(w => w.is_healthy).length ?? 0
  const totalWorkers = workerHealth?.length ?? 0

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
          Queues & Workers
        </h1>
      </div>

      {/* Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
              <Layers className="w-5 h-5 text-blue-600" />
            </div>
            <div>
              <p className="text-sm text-slate-500">Total Queues</p>
              <p className="text-2xl font-bold text-slate-900 dark:text-white">
                {mergedQueues.length}
              </p>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-green-100 dark:bg-green-900/30 rounded-lg">
              <Users className="w-5 h-5 text-green-600" />
            </div>
            <div>
              <p className="text-sm text-slate-500">Active Workers</p>
              <p className="text-2xl font-bold text-slate-900 dark:text-white">
                {healthyWorkers} / {totalWorkers}
              </p>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
              <Code className="w-5 h-5 text-purple-600" />
            </div>
            <div>
              <p className="text-sm text-slate-500">Handlers</p>
              <p className="text-2xl font-bold text-slate-900 dark:text-white">
                {handlers?.length ?? 0}
              </p>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-yellow-100 dark:bg-yellow-900/30 rounded-lg">
              <Activity className="w-5 h-5 text-yellow-600" />
            </div>
            <div>
              <p className="text-sm text-slate-500">Jobs Running</p>
              <p className="text-2xl font-bold text-slate-900 dark:text-white">
                {mergedQueues.reduce((sum, q) => sum + q.running, 0)}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Handlers Section */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4 flex items-center gap-2">
          <Code className="w-5 h-5" />
          Registered Handlers
        </h2>

        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          {handlers && handlers.length > 0 ? (
            <table className="w-full">
              <thead className="bg-slate-50 dark:bg-slate-900/50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Handler
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Queue
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Max Attempts
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Concurrency
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-slate-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
                {handlers.map(handler => (
                  <tr key={handler.kind} className="hover:bg-slate-50 dark:hover:bg-slate-700/50">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Code className="w-4 h-4 text-purple-500" />
                        <span className="font-medium text-slate-900 dark:text-white">{handler.kind}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
                      {handler.queue}
                    </td>
                    <td className="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
                      {handler.max_attempts}
                    </td>
                    <td className="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
                      {handler.concurrency > 0 ? handler.concurrency : '-'}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => setRunningHandler(handler)}
                        className="inline-flex items-center gap-1 px-3 py-1.5 bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium rounded-lg transition-colors"
                      >
                        <Zap className="w-4 h-4" />
                        Run
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="p-8 text-center text-slate-500">
              <Code className="w-12 h-12 mx-auto mb-3 text-slate-300" />
              <p>No handlers registered</p>
              <p className="text-sm mt-1">Register handlers in your Go code</p>
            </div>
          )}
        </div>
      </section>

      {/* Workers Section */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4 flex items-center gap-2">
          <Server className="w-5 h-5" />
          Workers
        </h2>

        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          {workers && workers.length > 0 ? (
            <div className="divide-y divide-slate-200 dark:divide-slate-700">
              {workers.map(worker => {
                const health = workerHealth?.find(h => h.id === worker.id)
                return (
                  <WorkerRow key={worker.id} worker={worker} health={health} />
                )
              })}
            </div>
          ) : (
            <div className="p-8 text-center text-slate-500">
              <Server className="w-12 h-12 mx-auto mb-3 text-slate-300" />
              <p>No workers connected</p>
              <p className="text-sm mt-1">Start a Rivo client to see workers here</p>
            </div>
          )}
        </div>
      </section>

      {/* Queues Section */}
      <section>
        <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-4 flex items-center gap-2">
          <Layers className="w-5 h-5" />
          Queue Statistics
        </h2>

        <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full">
            <thead className="bg-slate-50 dark:bg-slate-900/50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Queue
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Available
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Running
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Completed
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Concurrency
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium text-slate-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {mergedQueues.length > 0 ? (
                mergedQueues.map(queue => (
                  <QueueRow
                    key={queue.name}
                    queue={queue}
                    expanded={expandedQueue === queue.name}
                    onToggle={() => setExpandedQueue(expandedQueue === queue.name ? null : queue.name)}
                    onEdit={() => setEditingQueue({
                      name: queue.name,
                      namespace: 'default',
                      concurrency: queue.concurrency,
                      paused: queue.paused,
                      priority: queue.priority,
                      created_at: '',
                      updated_at: '',
                    })}
                  />
                ))
              ) : (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-slate-500">
                    No queues found. Enqueue a job to see queues here.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Edit Queue Modal */}
      {editingQueue && (
        <EditQueueModal
          queue={editingQueue}
          onClose={() => setEditingQueue(null)}
        />
      )}

      {/* Run Handler Modal */}
      {runningHandler && (
        <RunHandlerModal
          handler={runningHandler}
          onClose={() => setRunningHandler(null)}
        />
      )}
    </div>
  )
}

function WorkerRow({ worker, health }: { worker: Worker; health?: WorkerHealth }) {
  const isHealthy = health?.is_healthy ?? false

  return (
    <div className="px-4 py-4 flex items-center justify-between">
      <div className="flex items-center gap-4">
        <div className={`p-2 rounded-full ${isHealthy ? 'bg-green-100 dark:bg-green-900/30' : 'bg-red-100 dark:bg-red-900/30'}`}>
          {isHealthy ? (
            <Heart className="w-5 h-5 text-green-600" />
          ) : (
            <HeartOff className="w-5 h-5 text-red-600" />
          )}
        </div>
        <div>
          <p className="font-mono text-sm text-slate-900 dark:text-white">
            {worker.id.slice(0, 8)}...{worker.id.slice(-4)}
          </p>
          <p className="text-xs text-slate-500">
            Queues: {worker.queues.join(', ')}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-8 text-sm">
        <div className="text-center">
          <p className="text-slate-500">Concurrency</p>
          <p className="font-medium text-slate-900 dark:text-white">{worker.concurrency}</p>
        </div>
        <div className="text-center">
          <p className="text-slate-500">Processed</p>
          <p className="font-medium text-green-600">{worker.jobs_processed}</p>
        </div>
        <div className="text-center">
          <p className="text-slate-500">Failed</p>
          <p className="font-medium text-red-600">{worker.jobs_failed}</p>
        </div>
        <div className="text-center">
          <p className="text-slate-500">Last Heartbeat</p>
          <p className={`font-medium ${isHealthy ? 'text-green-600' : 'text-red-600'}`}>
            {health ? formatDistanceToNow(new Date(health.last_heartbeat), { addSuffix: true }) : 'Unknown'}
          </p>
        </div>
      </div>
    </div>
  )
}

function QueueRow({
  queue,
  expanded,
  onToggle,
  onEdit
}: {
  queue: QueueStats & { concurrency: number; paused: boolean; priority: number }
  expanded: boolean
  onToggle: () => void
  onEdit: () => void
}) {
  const queryClient = useQueryClient()

  const pauseMutation = useMutation({
    mutationFn: () => queue.paused ? api.resumeQueue(queue.name) : api.pauseQueue(queue.name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['queues'] }),
  })

  return (
    <>
      <tr className="hover:bg-slate-50 dark:hover:bg-slate-700/50">
        <td className="px-4 py-3 text-sm font-medium text-slate-900 dark:text-white">
          <button onClick={onToggle} className="flex items-center gap-2">
            {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            {queue.name}
          </button>
        </td>
        <td className="px-4 py-3">
          {queue.paused ? (
            <span className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-full bg-yellow-100 text-yellow-700">
              <Pause className="w-3 h-3" /> Paused
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-full bg-green-100 text-green-700">
              <Play className="w-3 h-3" /> Active
            </span>
          )}
        </td>
        <td className="px-4 py-3 text-sm text-blue-600 font-medium">
          {queue.available}
        </td>
        <td className="px-4 py-3 text-sm text-yellow-600 font-medium">
          {queue.running}
        </td>
        <td className="px-4 py-3 text-sm text-green-600 font-medium">
          {queue.completed}
        </td>
        <td className="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
          {queue.concurrency}
        </td>
        <td className="px-4 py-3 text-right">
          <div className="flex items-center justify-end gap-2">
            <button
              onClick={() => pauseMutation.mutate()}
              disabled={pauseMutation.isPending}
              className="p-1.5 text-slate-400 hover:text-yellow-600 transition-colors"
              title={queue.paused ? 'Resume' : 'Pause'}
            >
              {queue.paused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
            </button>
            <button
              onClick={onEdit}
              className="p-1.5 text-slate-400 hover:text-primary-600 transition-colors"
              title="Configure"
            >
              <Settings className="w-4 h-4" />
            </button>
          </div>
        </td>
      </tr>
      {expanded && (
        <tr>
          <td colSpan={7} className="px-4 py-3 bg-slate-50 dark:bg-slate-900/50">
            <div className="grid grid-cols-4 gap-4 text-sm">
              <div>
                <p className="text-slate-500">Retryable</p>
                <p className="font-medium text-orange-600">{queue.retryable}</p>
              </div>
              <div>
                <p className="text-slate-500">Failed</p>
                <p className="font-medium text-red-600">{queue.failed}</p>
              </div>
              <div>
                <p className="text-slate-500">Total Jobs</p>
                <p className="font-medium text-slate-900 dark:text-white">{queue.total}</p>
              </div>
              <div>
                <p className="text-slate-500">Priority</p>
                <p className="font-medium text-slate-900 dark:text-white">{queue.priority}</p>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

function EditQueueModal({ queue, onClose }: { queue: QueueConfig; onClose: () => void }) {
  const [concurrency, setConcurrency] = useState(queue.concurrency)
  const [priority, setPriority] = useState(queue.priority)
  const queryClient = useQueryClient()

  const updateMutation = useMutation({
    mutationFn: () => api.updateQueue(queue.name, {
      concurrency,
      priority,
      paused: queue.paused,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queues'] })
      onClose()
    },
  })

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-md mx-4">
        <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
            Configure Queue: {queue.name}
          </h2>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-slate-600">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault()
            updateMutation.mutate()
          }}
          className="p-6 space-y-4"
        >
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
              Concurrency
            </label>
            <input
              type="number"
              value={concurrency}
              onChange={(e) => setConcurrency(Number(e.target.value))}
              min={1}
              max={1000}
              className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="text-xs text-slate-500 mt-1">
              Maximum number of concurrent jobs for this queue
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
              Priority
            </label>
            <input
              type="number"
              value={priority}
              onChange={(e) => setPriority(Number(e.target.value))}
              min={-100}
              max={100}
              className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="text-xs text-slate-500 mt-1">
              Higher priority queues are processed first
            </p>
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
              disabled={updateMutation.isPending}
              className="px-4 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function RunHandlerModal({ handler, onClose }: { handler: Handler; onClose: () => void }) {
  const [payload, setPayload] = useState('{}')
  const [result, setResult] = useState<{ job_id: number; message: string } | null>(null)
  const queryClient = useQueryClient()

  const runMutation = useMutation({
    mutationFn: () => {
      const parsedPayload = JSON.parse(payload)
      return api.runHandler(handler.kind, parsedPayload)
    },
    onSuccess: (data) => {
      setResult(data)
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      queryClient.invalidateQueries({ queryKey: ['queue-stats'] })
    },
  })

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-md mx-4">
        <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
              Run Handler
            </h2>
            <p className="text-sm text-slate-500">{handler.kind}</p>
          </div>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-slate-600">
            <X className="w-5 h-5" />
          </button>
        </div>

        {result ? (
          <div className="p-6">
            <div className="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4 mb-4">
              <p className="text-green-700 dark:text-green-300 font-medium">
                {result.message}
              </p>
              <p className="text-sm text-green-600 dark:text-green-400 mt-1">
                Job ID: {result.job_id}
              </p>
            </div>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setResult(null)}
                className="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white transition-colors"
              >
                Run Another
              </button>
              <button
                onClick={onClose}
                className="px-4 py-2 bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium rounded-lg transition-colors"
              >
                Done
              </button>
            </div>
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              runMutation.mutate()
            }}
            className="p-6 space-y-4"
          >
            <div>
              <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
                Payload (JSON)
              </label>
              <textarea
                value={payload}
                onChange={(e) => setPayload(e.target.value)}
                rows={6}
                className="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900 text-slate-900 dark:text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                placeholder='{"key": "value"}'
              />
              <p className="text-xs text-slate-500 mt-1">
                Enter the job payload as JSON
              </p>
            </div>

            {runMutation.error && (
              <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3">
                <p className="text-sm text-red-700 dark:text-red-300">
                  {runMutation.error instanceof Error ? runMutation.error.message : 'Failed to run job'}
                </p>
              </div>
            )}

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
                disabled={runMutation.isPending}
                className="flex items-center gap-2 px-4 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
              >
                <Zap className="w-4 h-4" />
                {runMutation.isPending ? 'Running...' : 'Run Job'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
