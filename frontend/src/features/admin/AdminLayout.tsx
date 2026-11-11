import {
  ExternalLinkIcon,
  LayersIcon,
  LayoutDashboardIcon,
  LogOutIcon,
  ScrollTextIcon,
  SlidersHorizontalIcon,
  UsersIcon,
} from 'lucide-react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useAdminSession, useLogout } from './api'

const navItems = [
  { to: '/admin', label: 'Overview', icon: LayoutDashboardIcon, end: true },
  { to: '/admin/topics', label: 'Topics', icon: LayersIcon, end: false },
  { to: '/admin/registrations', label: 'Registrations', icon: UsersIcon, end: false },
  { to: '/admin/settings', label: 'Registration period', icon: SlidersHorizontalIcon, end: false },
  { to: '/admin/audit', label: 'Audit log', icon: ScrollTextIcon, end: false },
]

export default function AdminLayout() {
  const session = useAdminSession()
  const logout = useLogout()
  const navigate = useNavigate()

  function handleLogout() {
    logout.mutate(undefined, {
      onSettled: () => navigate('/admin/login', { replace: true }),
    })
  }

  return (
    <div className="min-h-dvh bg-muted/30">
      {/* Mobile: a normal header with horizontally scrollable tabs. */}
      <header className="sticky top-0 z-30 border-b bg-background/90 backdrop-blur md:hidden">
        <div className="flex h-14 items-center justify-between px-4">
          <span className="flex items-center gap-2">
            <span className="grid size-7 place-items-center rounded-lg bg-primary text-primary-foreground">
              <LayersIcon className="size-4" aria-hidden="true" />
            </span>
            <span className="font-semibold tracking-tight">Groupify</span>
            <span className="text-xs text-muted-foreground">Organiser</span>
          </span>
          <Button variant="ghost" size="sm" onClick={handleLogout} disabled={logout.isPending}>
            <LogOutIcon aria-hidden="true" />
            Sign out
          </Button>
        </div>
        <nav aria-label="Sections" className="flex gap-1 overflow-x-auto px-2 pb-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  'shrink-0 rounded-lg px-3 py-1.5 text-sm whitespace-nowrap transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </header>

      <div className="md:flex">
        <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col border-r bg-background md:flex">
          <div className="flex h-14 items-center gap-2 border-b px-4">
            <span className="grid size-7 place-items-center rounded-lg bg-primary text-primary-foreground">
              <LayersIcon className="size-4" aria-hidden="true" />
            </span>
            <span className="font-semibold tracking-tight">Groupify</span>
            <span className="text-xs text-muted-foreground">Organiser</span>
          </div>

          <nav aria-label="Sections" className="flex-1 space-y-1 p-3">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  cn(
                    'flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors',
                    isActive
                      ? 'bg-secondary font-medium text-foreground'
                      : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                  )
                }
              >
                <item.icon className="size-4" aria-hidden="true" />
                {item.label}
              </NavLink>
            ))}
          </nav>

          <div className="space-y-2 border-t p-3">
            <a
              href="/"
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            >
              <ExternalLinkIcon className="size-4" aria-hidden="true" />
              Student view
            </a>
            <p className="px-3 text-xs break-all text-muted-foreground">
              {session.data?.admin.email}
            </p>
            <Button
              variant="outline"
              className="w-full justify-start"
              onClick={handleLogout}
              disabled={logout.isPending}
            >
              <LogOutIcon aria-hidden="true" />
              Sign out
            </Button>
          </div>
        </aside>

        <main className="min-w-0 flex-1">
          <div className="mx-auto w-full max-w-5xl px-4 py-6 sm:px-6 lg:py-8">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
