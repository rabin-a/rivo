// Main embeddable component
export { RivoUI, type RivoUIProps, type RivoPage } from './RivoUI'
export { RivoUI as default } from './RivoUI'

// API client for custom integrations
export { api } from './api/client'
export type {
  Job,
  JobLog,
  QueueStats,
  JobStats,
  Worker,
  WorkerHealth,
  QueueConfig,
  Handler,
  EnqueueRequest,
  StepRunState,
  WorkflowRun,
} from './api/client'

// Individual page components for custom layouts
export { default as Dashboard } from './pages/Dashboard'
export { default as Jobs } from './pages/Jobs'
export { default as Queues } from './pages/Queues'
export { default as Workflows } from './pages/Workflows'
export { default as WorkflowRuns } from './pages/WorkflowRuns'
export { default as WorkflowRunDetail } from './pages/WorkflowRunDetail'
export { default as Schedules } from './pages/Schedules'
