import { AlertCircle } from 'lucide-react'

export default function Schedules() {
  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-slate-900 dark:text-white mb-6">
        Schedules
      </h1>

      <div className="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-8 text-center">
        <AlertCircle className="w-12 h-12 text-slate-400 mx-auto mb-4" />
        <h3 className="text-lg font-medium text-slate-900 dark:text-white mb-2">
          No Schedules Configured
        </h3>
        <p className="text-slate-500 dark:text-slate-400 max-w-md mx-auto">
          Periodic jobs will appear here once you configure them using the Go API.
          Schedules use cron expressions for flexible timing.
        </p>
        <div className="mt-6 p-4 bg-slate-50 dark:bg-slate-900 rounded-lg text-left max-w-md mx-auto">
          <p className="text-sm text-slate-600 dark:text-slate-400 mb-2">Example:</p>
          <pre className="text-sm font-mono text-slate-800 dark:text-slate-200">
{`client.Schedule(ctx, rivo.ScheduleParams{
    Name:     "daily-report",
    Kind:     "generate-report",
    CronExpr: "0 9 * * *", // 9 AM daily
})`}
          </pre>
        </div>
      </div>
    </div>
  )
}
