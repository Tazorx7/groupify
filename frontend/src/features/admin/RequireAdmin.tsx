import { Loader2Icon } from 'lucide-react'
import { Navigate, useLocation } from 'react-router-dom'
import { Skeleton } from '@/components/ui/skeleton'
import { useAdminSession } from './api'

export function FullPageLoader() {
  return (
    <div className="mx-auto w-full max-w-5xl space-y-4 px-4 py-10 sm:px-6">
      <Skeleton className="h-8 w-56" />
      <Skeleton className="h-4 w-80" />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[0, 1, 2, 3].map((index) => (
          <Skeleton key={index} className="h-24 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-64 rounded-xl" />
    </div>
  )
}

export function RequireAdmin({ children }: { children: React.ReactNode }) {
  const session = useAdminSession()
  const location = useLocation()

  if (session.isLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center">
        <Loader2Icon className="size-5 animate-spin text-muted-foreground" aria-label="Loading" />
      </div>
    )
  }

  if (session.isError) {
    return <Navigate to="/admin/login" replace state={{ from: location.pathname }} />
  }

  return <>{children}</>
}
