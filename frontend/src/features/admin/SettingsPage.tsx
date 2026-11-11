import { LockIcon, UnlockIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ApiError } from '@/lib/api'
import { formatDateTime, formatRelative } from '@/lib/format'
import { useAdminSettings, useUpdateSettings } from './api'
import { ConfirmDialog } from './ConfirmDialog'

export default function SettingsPage() {
  const settings = useAdminSettings()
  const update = useUpdateSettings()
  const [confirmOpen, setConfirmOpen] = useState(false)

  const open = settings.data?.registration_open ?? false

  function handleToggle() {
    const next = !open
    update.mutate(next, {
      onSuccess: () => {
        toast.success(next ? 'Registration is open.' : 'Registration is closed.')
        setConfirmOpen(false)
      },
      onError: (error) => {
        toast.error(
          error instanceof ApiError ? error.message : 'Could not change the registration period.',
        )
      },
    })
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Registration period</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          The server enforces this. Closing registration rejects submissions immediately, even from
          a page that is already open.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2 className="text-base font-semibold">Current state</h2>
          </CardTitle>
          <CardDescription>
            {settings.isLoading
              ? 'Loading…'
              : open
                ? 'Students can view topics and register.'
                : 'Students can browse topics but cannot register.'}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {settings.isLoading ? (
            <Skeleton className="h-10 w-40" />
          ) : (
            <div className="flex flex-wrap items-center justify-between gap-4">
              <span
                className={
                  open
                    ? 'inline-flex items-center gap-2 rounded-full border border-emerald-600/25 bg-emerald-500/10 px-3 py-1.5 text-sm text-emerald-800 dark:text-emerald-300'
                    : 'inline-flex items-center gap-2 rounded-full border bg-muted px-3 py-1.5 text-sm text-muted-foreground'
                }
              >
                {open ? (
                  <UnlockIcon className="size-4" aria-hidden="true" />
                ) : (
                  <LockIcon className="size-4" aria-hidden="true" />
                )}
                {open ? 'Registration open' : 'Registration closed'}
              </span>

              <Button
                variant={open ? 'destructive' : 'default'}
                onClick={() => setConfirmOpen(true)}
                disabled={update.isPending || settings.isLoading}
              >
                {open ? 'Close registration' : 'Open registration'}
              </Button>
            </div>
          )}

          {settings.data && (
            <p className="text-xs text-muted-foreground">
              Last changed {formatRelative(settings.data.updated_at)} ·{' '}
              <span title={formatDateTime(settings.data.updated_at)}>
                {formatDateTime(settings.data.updated_at)}
              </span>
              {settings.data.updated_by_admin_id
                ? ` · admin #${settings.data.updated_by_admin_id}`
                : null}
            </p>
          )}
        </CardContent>
      </Card>

      <Alert>
        <AlertTitle>What closing does</AlertTitle>
        <AlertDescription>
          New submissions are rejected with “registration is closed”, and the student page shows a
          closed banner. Existing registrations are untouched, and you can reopen at any time. Every
          change is written to the audit log.
        </AlertDescription>
      </Alert>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={open ? 'Close registration?' : 'Open registration?'}
        description={
          open
            ? 'Students will no longer be able to register or change their selection. Existing registrations stay as they are.'
            : 'Students will be able to register immediately. Make sure the topics and capacities are final.'
        }
        confirmLabel={open ? 'Close registration' : 'Open registration'}
        destructive={open}
        pending={update.isPending}
        onConfirm={handleToggle}
      />
    </div>
  )
}
