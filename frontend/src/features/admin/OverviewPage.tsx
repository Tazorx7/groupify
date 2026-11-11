import { ArrowRightIcon } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { formatNumber } from '@/lib/format'
import type { AdminTopic } from '@/lib/types'
import { useAdminTopics, useOverview } from './api'

function Stat({
  label,
  value,
  hint,
  loading,
}: {
  label: string
  value: string
  hint?: string
  loading: boolean
}) {
  return (
    <Card size="sm">
      <CardContent>
        <p className="text-xs tracking-wide text-muted-foreground uppercase">{label}</p>
        {loading ? (
          <Skeleton className="mt-2 h-7 w-16" />
        ) : (
          <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
        )}
        {hint && <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p>}
      </CardContent>
    </Card>
  )
}

function statusBadge(topic: AdminTopic) {
  if (!topic.is_active) return <Badge variant="outline">Inactive</Badge>
  if (topic.status === 'full') return <Badge variant="secondary">Full</Badge>
  return <Badge variant="outline">Open</Badge>
}

export default function OverviewPage() {
  const overview = useOverview()
  const topics = useAdminTopics()

  const data = overview.data
  const loading = overview.isLoading

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            How the course is distributing across topics.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {data && (
            <span
              className={cn(
                'inline-flex items-center gap-2 rounded-full border px-3 py-1 text-sm',
                data.registration_open
                  ? 'border-emerald-600/25 bg-emerald-500/10 text-emerald-800 dark:text-emerald-300'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              <span
                className={cn(
                  'size-1.5 rounded-full',
                  data.registration_open ? 'bg-emerald-600' : 'bg-muted-foreground',
                )}
                aria-hidden="true"
              />
              {data.registration_open ? 'Registration open' : 'Registration closed'}
            </span>
          )}
        </div>
      </div>

      {overview.isError && (
        <Card>
          <CardContent className="text-sm">
            <p className="font-medium">Could not load the overview.</p>
            <Button
              variant="outline"
              size="sm"
              className="mt-2"
              onClick={() => void overview.refetch()}
            >
              Try again
            </Button>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-3 grid-cols-2 lg:grid-cols-4">
        <Stat
          label="Registrations"
          value={data ? formatNumber(data.total_registrations) : '—'}
          hint="Students placed"
          loading={loading}
        />
        <Stat
          label="Active topics"
          value={data ? `${data.active_topics} of ${data.total_topics}` : '—'}
          hint="Visible to students"
          loading={loading}
        />
        <Stat
          label="Full topics"
          value={data ? formatNumber(data.full_topics) : '—'}
          hint="No places left"
          loading={loading}
        />
        <Stat
          label="Free places"
          value={data ? formatNumber(data.total_available_spaces) : '—'}
          hint="Across active topics"
          loading={loading}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2 className="text-base font-semibold">Occupancy by topic</h2>
          </CardTitle>
          <CardDescription>
            Counts come straight from the topics table, so they always match what students see.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {topics.isLoading && (
            <div className="space-y-3">
              {[0, 1, 2].map((index) => (
                <Skeleton key={index} className="h-10" />
              ))}
            </div>
          )}

          {topics.data && topics.data.topics.length === 0 && (
            <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
              <p>No topics yet. Students have nothing to choose from.</p>
              <Button className="mt-3" size="sm" nativeButton={false} render={<Link to="/admin/topics" />}>
                Create the first topic
              </Button>
            </div>
          )}

          {topics.data && topics.data.topics.length > 0 && (
            <ul className="divide-y">
              {topics.data.topics.map((topic) => {
                const percent =
                  topic.capacity > 0
                    ? Math.min(Math.round((topic.registered_count / topic.capacity) * 100), 100)
                    : 0
                return (
                  <li key={topic.id} className="py-3 first:pt-0 last:pb-0">
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="truncate font-medium">{topic.name}</span>
                        {statusBadge(topic)}
                      </div>
                      <span className="shrink-0 text-sm text-muted-foreground tabular-nums">
                        {topic.registered_count}/{topic.capacity}
                      </span>
                    </div>
                    <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        className={cn(
                          'h-full rounded-full',
                          topic.status === 'full' ? 'bg-foreground/40' : 'bg-primary',
                        )}
                        style={{ width: `${percent}%` }}
                      />
                    </div>
                  </li>
                )
              })}
            </ul>
          )}

          <div className="mt-4 flex flex-wrap gap-2">
            <Button variant="outline" size="sm" nativeButton={false} render={<Link to="/admin/topics" />}>
              Manage topics
              <ArrowRightIcon aria-hidden="true" />
            </Button>
            <Button variant="outline" size="sm" nativeButton={false} render={<Link to="/admin/registrations" />}>
              View registrations
              <ArrowRightIcon aria-hidden="true" />
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
