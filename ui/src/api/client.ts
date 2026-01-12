let API_BASE = '/api/v1'

export interface Job {
  id: number
  kind: string
  queue: string
  state: 'available' | 'scheduled' | 'running' | 'retryable' | 'completed' | 'cancelled' | 'discarded'
  payload: Record<string, unknown>
  priority: number
  attempt: number
  max_attempts: number
  scheduled_at: string
  attempted_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface JobLog {
  id: number
  job_id: number
  attempt: number
  worker_id: string
  started_at: string
  completed_at?: string
  duration_ms?: number
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  status: 'running' | 'completed' | 'failed'
  error?: string
  error_stack?: string
  logs?: Array<{ level: string; message: string; timestamp: string }>
  created_at: string
}

export interface QueueStats {
  name: string
  available: number
  running: number
  completed: number
  retryable: number
  failed: number
  total: number
}

export interface JobStats {
  total: number
  available: number
  running: number
  completed: number
  failed: number
  queues: QueueStats[]
}

export interface Worker {
  id: string
  namespace: string
  queues: string[]
  concurrency: number
  status: 'active' | 'stopped'
  started_at: string
  last_heartbeat: string
  jobs_processed: number
  jobs_failed: number
}

export interface WorkerHealth {
  id: string
  status: string
  is_healthy: boolean
  last_heartbeat: string
  heartbeat_age: string
  jobs_processed: number
  jobs_failed: number
}

export interface QueueConfig {
  name: string
  namespace: string
  concurrency: number
  paused: boolean
  priority: number
  created_at: string
  updated_at: string
}

export interface Handler {
  kind: string
  queue: string
  concurrency: number
  max_attempts: number
}

export interface EnqueueRequest {
  kind: string
  payload?: Record<string, unknown>
  queue?: string
  priority?: number
  schedule_at?: string
  idempotency_key?: string
}

export interface StepRunState {
  state: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  output?: Record<string, unknown>
  error?: string
  attempt: number
  started_at?: string
  completed_at?: string
}

export interface WorkflowStepLog {
  id: number
  workflow_run_id: number
  step_id: string
  attempt: number
  worker_id?: string
  started_at: string
  completed_at?: string
  duration_ms?: number
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  status: 'running' | 'completed' | 'failed'
  error?: string
  error_stack?: string
  logs?: Array<{ level: string; message: string; timestamp: string }>
  created_at: string
}

export interface WorkflowRun {
  id: number
  workflow_name: string
  namespace: string
  state: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  input: Record<string, unknown>
  output?: Record<string, unknown>
  step_states: Record<string, StepRunState>
  error?: string
  started_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
    },
    ...options,
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(error.message || 'Request failed')
  }

  return res.json()
}

export const api = {
  // Configuration
  setBaseUrl(url: string) {
    API_BASE = url
  },

  getBaseUrl() {
    return API_BASE
  },

  // Jobs
  async getJobs(params?: { state?: string; queue?: string; limit?: number }): Promise<Job[]> {
    const searchParams = new URLSearchParams()
    if (params?.state) searchParams.set('state', params.state)
    if (params?.queue) searchParams.set('queue', params.queue)
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    const query = searchParams.toString()
    return request(`/jobs${query ? `?${query}` : ''}`)
  },

  async getJob(id: number): Promise<Job> {
    return request(`/jobs/${id}`)
  },

  async getJobLogs(id: number): Promise<JobLog[]> {
    return request(`/jobs/${id}/logs`)
  },

  async enqueueJob(req: EnqueueRequest): Promise<Job> {
    return request('/jobs', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  async cancelJob(id: number): Promise<void> {
    await request(`/jobs/${id}/cancel`, { method: 'POST' })
  },

  async retryJob(id: number): Promise<void> {
    await request(`/jobs/${id}/retry`, { method: 'POST' })
  },

  // Stats
  async getStats(): Promise<JobStats> {
    return request('/stats')
  },

  // Workers
  async getWorkers(): Promise<Worker[]> {
    return request('/workers')
  },

  async getWorkersHealth(): Promise<WorkerHealth[]> {
    return request('/workers/health')
  },

  // Queues
  async getQueues(): Promise<QueueConfig[]> {
    return request('/queues')
  },

  async getQueueStats(): Promise<QueueStats[]> {
    return request('/queues/stats')
  },

  async updateQueue(name: string, config: { concurrency: number; priority: number; paused: boolean }): Promise<void> {
    await request(`/queues/${name}`, {
      method: 'PUT',
      body: JSON.stringify(config),
    })
  },

  async pauseQueue(name: string): Promise<void> {
    await request(`/queues/${name}/pause`, { method: 'POST' })
  },

  async resumeQueue(name: string): Promise<void> {
    await request(`/queues/${name}/resume`, { method: 'POST' })
  },

  // Health
  async getHealth(): Promise<{ status: string }> {
    return request('/health')
  },

  // Handlers
  async getHandlers(): Promise<Handler[]> {
    return request('/handlers')
  },

  async runHandler(kind: string, payload?: Record<string, unknown>): Promise<{ job_id: number; kind: string; queue: string; state: string; message: string }> {
    return request(`/handlers/${kind}/run`, {
      method: 'POST',
      body: JSON.stringify({ payload: payload || {} }),
    })
  },

  // Workflows
  async getWorkflows(): Promise<string[]> {
    return request('/workflows')
  },

  async startWorkflow(name: string, input?: Record<string, unknown>): Promise<WorkflowRun> {
    return request(`/workflows/${name}/start`, {
      method: 'POST',
      body: JSON.stringify({ input: input || {} }),
    })
  },

  async getWorkflowRuns(params?: { state?: string; limit?: number }): Promise<WorkflowRun[]> {
    const searchParams = new URLSearchParams()
    if (params?.state) searchParams.set('state', params.state)
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    const query = searchParams.toString()
    return request(`/workflow-runs${query ? `?${query}` : ''}`)
  },

  async getWorkflowRun(id: number): Promise<WorkflowRun> {
    return request(`/workflow-runs/${id}`)
  },

  async cancelWorkflowRun(id: number): Promise<void> {
    await request(`/workflow-runs/${id}/cancel`, { method: 'POST' })
  },

  async getWorkflowStepLogs(runId: number): Promise<WorkflowStepLog[]> {
    return request(`/workflow-runs/${runId}/steps`)
  },

  async rerunWorkflowStep(runId: number, stepId: string): Promise<{ status: string }> {
    return request(`/workflow-runs/${runId}/steps/${stepId}/rerun`, { method: 'POST' })
  },
}
