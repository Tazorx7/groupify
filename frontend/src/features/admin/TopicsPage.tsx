import { PencilIcon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'
import { ApiError } from '@/lib/api'
import type { AdminTopic } from '@/lib/types'
import { useAdminTopics, useSetTopicActive } from './api'
import { ConfirmDialog } from './ConfirmDialog'
import { TopicDialog } from './TopicDialog'

function StatusBadge({ topic }: { topic: AdminTopic }) {
  if (!topic.is_active) return <Badge variant="outline">Inactive</Badge>
  if (topic.status === 'full') return <Badge variant="secondary">Full</Badge>
  return <Badge variant="outline">Open</Badge>
}

export default function TopicsPage() {
  const topics = useAdminTopics()
  const setActive = useSetTopicActive()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<AdminTopic | null>(null)
  // Bumped on every open so the dialog remounts with clean form state.
  const [dialogKey, setDialogKey] = useState(0)
  const [pendingToggle, setPendingToggle] = useState<AdminTopic | null>(null)

  function openCreate() {
    setEditing(null)
    setDialogKey((key) => key + 1)
    setDialogOpen(true)
  }

  function openEdit(topic: AdminTopic) {
    setEditing(topic)
    setDialogKey((key) => key + 1)
    setDialogOpen(true)
  }

  function handleToggle() {
    if (!pendingToggle) return
    const active = !pendingToggle.is_active
    setActive.mutate(
      { id: pendingToggle.id, active },
      {
        onSuccess: () => {
          toast.success(
            active
              ? `“${pendingToggle.name}” is visible to students again.`
              : `“${pendingToggle.name}” is hidden from students.`,
          )
          setPendingToggle(null)
        },
        onError: (error) => {
          toast.error(
            error instanceof ApiError ? error.message : 'Could not change the topic state.',
          )
        },
      },
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Topics</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Capacities are enforced by the database, so they cannot be exceeded.
          </p>
        </div>
        <Button onClick={openCreate}>
          <PlusIcon aria-hidden="true" />
          New topic
        </Button>
      </div>

      <Card className="py-0">
        <CardContent className="px-0">
          {topics.isLoading && (
            <div className="space-y-2 p-4">
              {[0, 1, 2].map((index) => (
                <Skeleton key={index} className="h-12" />
              ))}
            </div>
          )}

          {topics.isError && (
            <div className="p-6 text-sm">
              <p className="font-medium">Could not load topics.</p>
              <Button
                variant="outline"
                size="sm"
                className="mt-2"
                onClick={() => void topics.refetch()}
              >
                Try again
              </Button>
            </div>
          )}

          {topics.data && topics.data.topics.length === 0 && (
            <div className="p-10 text-center text-sm text-muted-foreground">
              <p>No topics yet.</p>
              <Button className="mt-3" size="sm" onClick={openCreate}>
                Create the first topic
              </Button>
            </div>
          )}

          {topics.data && topics.data.topics.length > 0 && (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Topic</TableHead>
                    <TableHead className="w-24 text-right">Capacity</TableHead>
                    <TableHead className="w-24 text-right">Registered</TableHead>
                    <TableHead className="w-28">Status</TableHead>
                    <TableHead className="w-44 text-right">
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {topics.data.topics.map((topic) => {
                    const percent =
                      topic.capacity > 0
                        ? Math.min(
                            Math.round((topic.registered_count / topic.capacity) * 100),
                            100,
                          )
                        : 0
                    return (
                      <TableRow key={topic.id}>
                        <TableCell className="max-w-md whitespace-normal">
                          <p className="font-medium">{topic.name}</p>
                          <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
                            {topic.description || 'No description'}
                          </p>
                          <div className="mt-2 h-1 w-40 max-w-full overflow-hidden rounded-full bg-muted">
                            <div
                              className={cn(
                                'h-full rounded-full',
                                topic.status === 'full' ? 'bg-foreground/40' : 'bg-primary',
                              )}
                              style={{ width: `${percent}%` }}
                            />
                          </div>
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{topic.capacity}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {topic.registered_count}
                          <span className="ml-1 text-xs text-muted-foreground">
                            ({topic.remaining_spaces} left)
                          </span>
                        </TableCell>
                        <TableCell>
                          <StatusBadge topic={topic} />
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-1">
                            <Button variant="ghost" size="sm" onClick={() => openEdit(topic)}>
                              <PencilIcon aria-hidden="true" />
                              Edit
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setPendingToggle(topic)}
                              disabled={setActive.isPending}
                            >
                              {topic.is_active ? 'Deactivate' : 'Activate'}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <TopicDialog
        key={dialogKey}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        topic={editing}
      />

      <ConfirmDialog
        open={pendingToggle !== null}
        onOpenChange={(open) => !open && setPendingToggle(null)}
        title={pendingToggle?.is_active ? 'Hide this topic?' : 'Show this topic again?'}
        description={
          pendingToggle?.is_active ? (
            <>
              Students will no longer see “{pendingToggle?.name}” when choosing a topic.
              {pendingToggle?.registered_count ? (
                <> The {pendingToggle.registered_count} existing registrations are kept.</>
              ) : null}
            </>
          ) : (
            <>
              “{pendingToggle?.name}” will appear on the student page again, with{' '}
              {pendingToggle?.remaining_spaces} places left.
            </>
          )
        }
        confirmLabel={pendingToggle?.is_active ? 'Deactivate topic' : 'Activate topic'}
        pending={setActive.isPending}
        onConfirm={handleToggle}
      />
    </div>
  )
}
