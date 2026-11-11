import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime, formatNumber, formatRelative } from '@/lib/format'
import { useAuditLogs } from './api'

const PAGE_SIZE = 50

const actionLabels: Record<string, string> = {
  'registration.created': 'Registration created',
  'registration.deleted': 'Registration removed',
  'registration.period.opened': 'Registration opened',
  'registration.period.closed': 'Registration closed',
  'topic.created': 'Topic created',
  'topic.updated': 'Topic updated',
  'topic.capacity_changed': 'Capacity changed',
  'topic.activated': 'Topic activated',
  'topic.deactivated': 'Topic deactivated',
  'admin.login': 'Admin signed in',
  'admin.logout': 'Admin signed out',
}

const selectClass =
  'h-9 rounded-lg border border-input bg-background px-2 text-sm outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50'

function tone(action: string) {
  if (action === 'registration.deleted' || action === 'topic.deactivated') return 'destructive' as const
  if (action.endsWith('.created') || action.endsWith('.opened')) return 'default' as const
  return 'outline' as const
}

export default function AuditPage() {
  const [action, setAction] = useState('')
  const [page, setPage] = useState(1)

  const logs = useAuditLogs(action, page, PAGE_SIZE)
  const data = logs.data
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Audit log</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Every change, newest first. Entries never contain student names or index numbers.
        </p>
      </div>

      <Card className="py-0">
        <div className="flex flex-wrap items-center gap-2 border-b p-3">
          <select
            aria-label="Filter by action"
            className={`${selectClass} min-w-52`}
            value={action}
            onChange={(event) => {
              setAction(event.target.value)
              setPage(1)
            }}
          >
            <option value="">All actions</option>
            {Object.entries(actionLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <span className="text-sm text-muted-foreground">
            {data ? `${formatNumber(total)} entries` : null}
          </span>
        </div>

        <CardContent className="px-0">
          {logs.isLoading && (
            <div className="space-y-2 p-4">
              {[0, 1, 2, 3, 4].map((index) => (
                <Skeleton key={index} className="h-14" />
              ))}
            </div>
          )}

          {logs.isError && (
            <div className="p-6 text-sm">
              <p className="font-medium">Could not load the audit log.</p>
              <Button variant="outline" size="sm" className="mt-2" onClick={() => void logs.refetch()}>
                Try again
              </Button>
            </div>
          )}

          {data && data.items.length === 0 && (
            <p className="p-10 text-center text-sm text-muted-foreground">
              No entries for this filter.
            </p>
          )}

          {data && data.items.length > 0 && (
            <ul className="divide-y">
              {data.items.map((entry) => (
                <li key={entry.id} className="px-4 py-3">
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <Badge variant={tone(entry.action)}>
                        {actionLabels[entry.action] ?? entry.action}
                      </Badge>
                      <span className="text-sm text-muted-foreground">
                        {entry.actor_email ?? entry.actor_label ?? 'system'}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {entry.entity_type}
                        {entry.entity_id ? ` #${entry.entity_id}` : ''}
                      </span>
                    </div>
                    <span
                      className="text-xs whitespace-nowrap text-muted-foreground"
                      title={formatDateTime(entry.created_at)}
                    >
                      {formatRelative(entry.created_at)}
                    </span>
                  </div>

                  {(entry.before_data || entry.after_data) && (
                    <details className="mt-1.5">
                      <summary className="cursor-pointer text-xs text-muted-foreground select-none">
                        Details
                      </summary>
                      <pre className="mt-1.5 overflow-x-auto rounded-lg bg-muted/60 p-2.5 text-xs">
                        {JSON.stringify(
                          { before: entry.before_data, after: entry.after_data },
                          null,
                          2,
                        )}
                      </pre>
                    </details>
                  )}
                </li>
              ))}
            </ul>
          )}

          <div className="flex items-center justify-between gap-3 border-t p-3 text-sm text-muted-foreground">
            <span className="tabular-nums">
              Page {page} of {totalPages}
            </span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1 || logs.isFetching}
                onClick={() => setPage(page - 1)}
              >
                <ChevronLeftIcon aria-hidden="true" />
                Newer
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages || logs.isFetching}
                onClick={() => setPage(page + 1)}
              >
                Older
                <ChevronRightIcon aria-hidden="true" />
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
