import { CheckIcon, PrinterIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { formatDateTime } from '@/lib/format'
import type { RegistrationConfirmation } from '@/lib/types'

export function ConfirmationView({ confirmation }: { confirmation: RegistrationConfirmation }) {
  return (
    <div className="mx-auto w-full max-w-lg px-4 py-12 sm:py-20">
      <Card>
        <CardContent className="space-y-6">
          <div className="flex flex-col items-center text-center">
            <span className="grid size-14 place-items-center rounded-full bg-emerald-500/12 text-emerald-700 dark:text-emerald-400">
              <CheckIcon className="size-7" aria-hidden="true" />
            </span>
            <h1 className="mt-4 text-2xl font-semibold tracking-tight">You are registered</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Your place in {confirmation.topic.name} is reserved.
            </p>
          </div>

          <Separator />

          <dl className="space-y-3 text-sm">
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Name</dt>
              <dd className="text-right font-medium">{confirmation.full_name}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Index number</dt>
              <dd className="text-right font-mono font-medium">{confirmation.index_number}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Topic</dt>
              <dd className="text-right font-medium">{confirmation.topic.name}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Registered</dt>
              <dd className="text-right">{formatDateTime(confirmation.registered_at)}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Reference</dt>
              <dd className="text-right font-mono text-muted-foreground">
                #{confirmation.registration_id}
              </dd>
            </div>
          </dl>

          <div className="rounded-lg bg-muted/60 p-3 text-sm">
            <p className="font-medium">This selection is final</p>
            <p className="mt-1 text-muted-foreground">
              You cannot switch or cancel it here. If something is wrong, contact your course rep and
              they can correct it.
            </p>
          </div>

          <Button
            variant="outline"
            className="w-full"
            onClick={() => {
              window.print()
            }}
          >
            <PrinterIcon aria-hidden="true" />
            Print this confirmation
          </Button>
        </CardContent>
      </Card>

      <p className="mt-4 text-center text-xs text-muted-foreground">
        Keep this reference. It is the only proof of your registration.
      </p>
    </div>
  )
}
