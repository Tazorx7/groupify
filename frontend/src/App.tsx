import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { BrowserRouter, Link, Navigate, Outlet, Route, Routes, useNavigate } from 'react-router-dom'
import { SiteHeader } from '@/components/site-header'
import { Button } from '@/components/ui/button'
import AdminLayout from '@/features/admin/AdminLayout'
import AuditPage from '@/features/admin/AuditPage'
import { adminKeys } from '@/features/admin/api'
import LoginPage from '@/features/admin/LoginPage'
import OverviewPage from '@/features/admin/OverviewPage'
import RegistrationsPage from '@/features/admin/RegistrationsPage'
import { RequireAdmin } from '@/features/admin/RequireAdmin'
import SettingsPage from '@/features/admin/SettingsPage'
import TopicsPage from '@/features/admin/TopicsPage'
import StudentPage from '@/features/student/StudentPage'
import { setUnauthenticatedHandler } from '@/lib/api'

function PublicLayout() {
  return (
    <div className="flex min-h-dvh flex-col">
      <SiteHeader />
      <div className="flex-1">
        <Outlet />
      </div>
      <footer className="border-t">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 px-4 py-4 text-xs text-muted-foreground sm:px-6">
          <span>Groupify — topic registration for one course.</span>
          <span>Only topic availability is public.</span>
        </div>
      </footer>
    </div>
  )
}

function NotFoundPage() {
  return (
    <div className="mx-auto max-w-lg px-4 py-20 text-center">
      <h1 className="text-2xl font-semibold tracking-tight">Page not found</h1>
      <p className="mt-2 text-muted-foreground">That link does not lead anywhere.</p>
      <Button className="mt-6" nativeButton={false} render={<Link to="/" />}>
        Back to topics
      </Button>
    </div>
  )
}

/**
 * Turns any 401 from the API into a redirect to the sign-in page. The client
 * wrapper calls this so an expired session cannot leave the console showing
 * stale data that it silently fails to refresh.
 */
function SessionWatcher() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  useEffect(() => {
    setUnauthenticatedHandler(() => {
      queryClient.removeQueries({ queryKey: adminKeys.session })
      navigate('/admin/login', { replace: true })
    })
    return () => setUnauthenticatedHandler(null)
  }, [navigate, queryClient])

  return null
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionWatcher />
      <Routes>
        <Route element={<PublicLayout />}>
          <Route index element={<StudentPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>

        <Route path="/admin/login" element={<LoginPage />} />
        <Route
          path="/admin"
          element={
            <RequireAdmin>
              <AdminLayout />
            </RequireAdmin>
          }
        >
          <Route index element={<OverviewPage />} />
          <Route path="topics" element={<TopicsPage />} />
          <Route path="registrations" element={<RegistrationsPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
