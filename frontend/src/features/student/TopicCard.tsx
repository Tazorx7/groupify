import { CheckIcon } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { Topic } from '@/lib/types'

interface TopicCardProps {
  topic: Topic
  selected: boolean
  disabled: boolean
  onSelect: (id: number) => void
}

/**
 * A radio button dressed as a card.
 *
 * It is a real `<input type="radio">` rather than a styled div, so arrow keys,
 * screen readers and form semantics work without re-implementing them.
 */
export function TopicCard({ topic, selected, disabled, onSelect }: TopicCardProps) {
  const isFull = topic.status === 'full'
  const unavailable = disabled || isFull
  const ratio = topic.capacity > 0 ? Math.min(topic.registered_count / topic.capacity, 1) : 0
  const percent = Math.round(ratio * 100)

  return (
    <label
      className={cn(
        'relative block rounded-xl bg-card p-4 ring-1 ring-foreground/10 transition',
        'has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring',
        !unavailable && 'cursor-pointer hover:ring-foreground/25',
        selected && !unavailable && 'ring-2 ring-primary hover:ring-primary',
        unavailable && 'opacity-65',
      )}
    >
      <input
        type="radio"
        name="topic"
        value={topic.id}
        checked={selected}
        disabled={unavailable}
        onChange={() => onSelect(topic.id)}
        className="sr-only"
      />

      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="font-medium leading-tight">{topic.name}</p>
          <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">
            {topic.description || 'No description provided.'}
          </p>
        </div>

        {isFull ? (
          <Badge variant="secondary" className="shrink-0">
            Full
          </Badge>
        ) : selected ? (
          <span className="grid size-5 shrink-0 place-items-center rounded-full bg-primary text-primary-foreground">
            <CheckIcon className="size-3.5" aria-hidden="true" />
            <span className="sr-only">Selected</span>
          </span>
        ) : null}
      </div>

      <div className="mt-4 space-y-1.5">
        <div
          className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={topic.capacity}
          aria-valuenow={topic.registered_count}
          aria-label={`${topic.name}: ${topic.registered_count} of ${topic.capacity} places taken`}
        >
          <div
            className={cn('h-full rounded-full', isFull ? 'bg-foreground/40' : 'bg-primary')}
            style={{ width: `${percent}%` }}
          />
        </div>

        <p className="flex items-baseline justify-between gap-2 text-xs text-muted-foreground">
          <span>
            <span className="font-medium text-foreground tabular-nums">
              {topic.remaining_spaces}
            </span>{' '}
            {topic.remaining_spaces === 1 ? 'place left' : 'places left'}
          </span>
          <span className="tabular-nums">
            {topic.registered_count}/{topic.capacity} taken
          </span>
        </p>
      </div>
    </label>
  )
}
