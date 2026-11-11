import { BrowserRouter, Link, Outlet, Route, Routes } from 'react-router-dom'
import { SiteHeader } from '@/components/site-header'
import { Button } from '@/components/ui/button'
import StudentPage from '@/features/student/StudentPage'

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
      <Button className="mt-6" render={<Link to="/" />}>
        Back to topics
      </Button>
    </div>
  )
}

function AdminPlaceholder() {
  return (
    <div className="mx-auto max-w-lg px-4 py-20">
      <h1 className="text-2xl font-semibold tracking-tight">Organiser console</h1>
      <p className="mt-2 text-muted-foreground">Sign-in and dashboards land here next.</p>
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<PublicLayout />}>
          <Route index element={<StudentPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        <Route path="/admin/*" element={<AdminPlaceholder />} />
      </Routes>
    </BrowserRouter>
  )
}
