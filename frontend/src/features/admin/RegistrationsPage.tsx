import { DownloadIcon, SearchIcon, Trash2Icon } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ApiError, downloadFile, toQuery } from '@/lib/api'
import { formatDateTime, formatNumber, formatRelative } from '@/lib/format'
import type { AdminRegistration, RegistrationSort, SortDirection } from '@/lib/types'
import { useAdminTopics, useDeleteRegistration, useRegistrations } from './api'
import { ConfirmDialog } from './ConfirmDialog'

const PAGE_SIZE = 50

const sortOptions: { value: RegistrationSort; label: string }[] = [
  { value: 'registered_at', label: 'Registration time' },
  { value: 'full_name', label: 'Student name' },
  { value: 'index_number', label: 'Index number' },
  { value: 'topic', label: 'Topic' },
]

const selectClass =
  'h-9 rounded-lg border border-input bg-background px-2 text-sm outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50'

export default function RegistrationsPage() {
  const topics = useAdminTopics()
  const deleteRegistration = useDeleteRegistration()

  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [topicId, setTopicId] = useState<number | null>(null)
  const [sort, setSort] = useState<RegistrationSort>('registered_at')
  const [dir, setDir] = useState<SortDirection>('desc')
  const [page, setPage] = useState(1)
  const [pendingDelete, setPendingDelete] = useState<AdminRegistration | null>(null)

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search.trim())
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [search])

  const filters = useMemo(
    () => ({
      search: debouncedSearch,
      topicId,
      sort,
      dir,
      page,
      pageSize: PAGE_SIZE,
    }),
    [debouncedSearch, topicId, sort, dir, page],
  )

  const registrations = useRegistrations(filters)
  const data = registrations.data
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const firstRow = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1
  const lastRow = Math.min(page * PAGE_SIZE, total)

  const exportQuery = toQuery({
    search: debouncedSearch,
    topic_id: topicId,
    sort,
    dir,
  })

  function changeFilter<T>(setter: (value: T) => void) {
    return (value: T) => {
      setter(value)
      setPage(1)
    }
  }

  function handleDelete() {
    if (!pendingDelete) return
    deleteRegistration.mutate(pendingDelete.id, {
      onSuccess: () => {
        toast.success(`Removed ${pendingDelete.full_name}. The place is free again.`)
        setPendingDelete(null)
        // Stepping back a page avoids landing on an empty last page.
        if (data && data.items.length === 1 && page > 1) setPage(page - 1)
      },
      onError: (error) => {
        toast.error(
          error instanceof ApiError ? error.message : 'Could not remove the registration.',
        )
      },
    })
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Registrations</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {data
              ? `${formatNumber(total)} ${total === 1 ? 'registration' : 'registrations'}${debouncedSearch || topicId ? ' matching this filter' : ''}.`
              : 'Student records, visible only to organisers.'}
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => downloadFile(`/api/admin/registrations/export${exportQuery}`)}
          >
            <DownloadIcon aria-hidden="true" />
            CSV
          </Button>
          <Button
            variant="outline"
            onClick={() =>
              downloadFile(
                `/api/admin/registrations/export${toQuery({
                  format: 'xlsx',
                  search: debouncedSearch,
                  topic_id: topicId,
                  sort,
                  dir,
                })}`,
              )
            }
          >
            <DownloadIcon aria-hidden="true" />
            Excel (grouped)
          </Button>
        </div>
      </div>

      <Card className="py-0">
        <div className="flex flex-wrap items-center gap-2 border-b p-3">
          <div className="relative min-w-52 flex-1">
            <SearchIcon
              className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search name or index number"
              aria-label="Search registrations"
              className="h-9 pl-8"
            />
          </div>

          <select
            aria-label="Filter by topic"
            className={selectClass}
            value={topicId ?? ''}
            onChange={(event) =>
              changeFilter(setTopicId)(event.target.value ? Number(event.target.value) : null)
            }
          >
            <option value="">All topics</option>
            {topics.data?.topics.map((topic) => (
              <option key={topic.id} value={topic.id}>
                {topic.name}
              </option>
            ))}
          </select>

          <select
            aria-label="Sort by"
            className={selectClass}
            value={sort}
            onChange={(event) => changeFilter(setSort)(event.target.value as RegistrationSort)}
          >
            {sortOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>

          <Button
            variant="outline"
            className="h-9"
            onClick={() => {
              setDir(dir === 'asc' ? 'desc' : 'asc')
              setPage(1)
            }}
            aria-label={`Sort direction: ${dir === 'asc' ? 'ascending' : 'descending'}`}
          >
            {dir === 'asc' ? '↑ Asc' : '↓ Desc'}
          </Button>
        </div>

        <CardContent className="px-0">
          {registrations.isLoading && (
            <div className="space-y-2 p-4">
              {[0, 1, 2, 3, 4].map((index) => (
                <Skeleton key={index} className="h-12" />
              ))}
            </div>
          )}

          {registrations.isError && (
            <div className="p-6 text-sm">
              <p className="font-medium">Could not load registrations.</p>
              <Button
                variant="outline"
                size="sm"
                className="mt-2"
                onClick={() => void registrations.refetch()}
              >
                Try again
              </Button>
            </div>
          )}

          {data && data.items.length === 0 && (
            <div className="p-10 text-center text-sm text-muted-foreground">
              {debouncedSearch || topicId
                ? 'No registrations match this filter.'
                : 'Nobody has registered yet.'}
            </div>
          )}

          {data && data.items.length > 0 && (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Student</TableHead>
                    <TableHead>Topic</TableHead>
                    <TableHead className="hidden sm:table-cell">Registered</TableHead>
                    <TableHead className="w-12 text-right">
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((registration) => (
                    <TableRow key={registration.id}>
                      <TableCell>
                        <p className="font-medium">{registration.full_name}</p>
                        <p className="font-mono text-xs text-muted-foreground">
                          {registration.index_number}
                        </p>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{registration.topic_name}</Badge>
                      </TableCell>
                      <TableCell className="hidden text-sm text-muted-foreground sm:table-cell">
                        <span title={formatDateTime(registration.created_at)}>
                          {formatRelative(registration.created_at)}
                        </span>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Remove registration for ${registration.full_name}`}
                          onClick={() => setPendingDelete(registration)}
                        >
                          <Trash2Icon aria-hidden="true" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          <div className="flex flex-wrap items-center justify-between gap-3 border-t p-3 text-sm text-muted-foreground">
            <span className="tabular-nums">
              {total === 0 ? 'No rows' : `${firstRow}–${lastRow} of ${formatNumber(total)}`}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1 || registrations.isFetching}
                onClick={() => setPage(page - 1)}
              >
                Previous
              </Button>
              <span className="tabular-nums">
                Page {page} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages || registrations.isFetching}
                onClick={() => setPage(page + 1)}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="Remove this registration?"
        description={
          <>
            <strong>{pendingDelete?.full_name}</strong> ({pendingDelete?.index_number}) will lose their
            place in {pendingDelete?.topic_name}, and the slot becomes available to someone else.
            This is recorded in the audit log. The student is not notified automatically.
          </>
        }
        confirmLabel="Remove registration"
        destructive
        pending={deleteRegistration.isPending}
        onConfirm={handleDelete}
      />
    </div>
  )
}
