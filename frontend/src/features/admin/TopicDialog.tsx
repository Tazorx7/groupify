import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api'
import type { AdminTopic } from '@/lib/types'
import { useCreateTopic, useUpdateTopic } from './api'

interface TopicDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Omitted when creating a new topic. */
  topic: AdminTopic | null
}

interface Values {
  name: string
  description: string
  capacity: string
}

type FieldErrors = Partial<Record<keyof Values, string>>

function initialValues(topic: AdminTopic | null): Values {
  return {
    name: topic?.name ?? '',
    description: topic?.description ?? '',
    capacity: topic ? String(topic.capacity) : '',
  }
}

export function TopicDialog({ open, onOpenChange, topic }: TopicDialogProps) {
  const create = useCreateTopic()
  const update = useUpdateTopic()
  const [values, setValues] = useState<Values>(() => initialValues(topic))
  const [errors, setErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)

  // The parent remounts this dialog (via `key`) each time it opens, so the
  // initial state above is always fresh — no effect needed.

  const pending = create.isPending || update.isPending
  const editing = topic !== null

  function validate(): FieldErrors {
    const next: FieldErrors = {}
    const capacity = Number(values.capacity)
    if (!values.name.trim()) next.name = 'Topic name is required.'
    else if ([...values.name.trim()].length > 120) next.name = 'Topic name is too long (max 120 characters).'
    if ([...values.description].length > 2000) next.description = 'Description is too long (max 2000 characters).'
    if (!values.capacity.trim() || !Number.isFinite(capacity) || !Number.isInteger(capacity)) {
      next.capacity = 'Capacity must be a whole number.'
    } else if (capacity < 1) {
      next.capacity = 'Capacity must be at least 1.'
    } else if (capacity > 1000) {
      next.capacity = 'Capacity may not exceed 1000.'
    }
    return next
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setFormError(null)

    const nextErrors = validate()
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length > 0) return

    const payload = {
      name: values.name.trim(),
      description: values.description.trim(),
      capacity: Number(values.capacity),
    }

    try {
      if (editing) {
        await update.mutateAsync({ id: topic.id, patch: payload })
        toast.success(`Saved “${payload.name}”.`)
      } else {
        await create.mutateAsync(payload)
        toast.success(`Created “${payload.name}”.`)
      }
      onOpenChange(false)
    } catch (error) {
      if (error instanceof ApiError) {
        switch (error.code) {
          case 'topic_name_taken':
            setErrors({ name: 'Another topic already uses this name.' })
            break
          case 'capacity_below_members':
            setErrors({
              capacity: `Capacity cannot be lower than the ${topic?.registered_count ?? 0} students already registered.`,
            })
            break
          case 'validation_failed':
            setErrors((error.fields ?? {}) as FieldErrors)
            setFormError(error.message)
            break
          default:
            setFormError(error.message)
        }
      } else {
        setFormError('Could not reach the server. Try again.')
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit topic' : 'New topic'}</DialogTitle>
          <DialogDescription>
            {editing
              ? 'Students see the name and description on the registration page.'
              : 'The topic becomes visible to students immediately once registration is open.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4">
          {formError && (
            <Alert variant="destructive">
              <AlertTitle>Could not save</AlertTitle>
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="topic-name">Name</Label>
            <Input
              id="topic-name"
              value={values.name}
              onChange={(event) => setValues((v) => ({ ...v, name: event.target.value }))}
              aria-invalid={Boolean(errors.name)}
              aria-describedby={errors.name ? 'topic-name-error' : undefined}
              placeholder="Distributed Systems"
              className="h-10"
            />
            {errors.name && (
              <p id="topic-name-error" className="text-xs text-destructive">
                {errors.name}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="topic-description">Description</Label>
            <Textarea
              id="topic-description"
              value={values.description}
              onChange={(event) => setValues((v) => ({ ...v, description: event.target.value }))}
              aria-invalid={Boolean(errors.description)}
              aria-describedby={errors.description ? 'topic-description-error' : undefined}
              rows={3}
              placeholder="What the topic covers."
            />
            {errors.description && (
              <p id="topic-description-error" className="text-xs text-destructive">
                {errors.description}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="topic-capacity">Capacity</Label>
            <Input
              id="topic-capacity"
              type="number"
              inputMode="numeric"
              min={1}
              max={1000}
              value={values.capacity}
              onChange={(event) => setValues((v) => ({ ...v, capacity: event.target.value }))}
              aria-invalid={Boolean(errors.capacity)}
              aria-describedby={
                errors.capacity ? 'topic-capacity-error' : 'topic-capacity-hint'
              }
              className="h-10 w-28 tabular-nums"
            />
            {errors.capacity ? (
              <p id="topic-capacity-error" className="text-xs text-destructive">
                {errors.capacity}
              </p>
            ) : (
              <p id="topic-capacity-hint" className="text-xs text-muted-foreground">
                {topic
                  ? `${topic.registered_count} students are already registered, so capacity cannot go below that.`
                  : 'Between 1 and 1000. It can be raised or lowered later.'}
              </p>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={pending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {pending && <Loader2Icon className="animate-spin" aria-hidden="true" />}
              {editing ? 'Save changes' : 'Create topic'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
