import { UsersIcon } from 'lucide-react'
import { Link } from 'react-router-dom'

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-30 border-b bg-background/85 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
        <Link to="/" className="flex items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <span className="grid size-7 place-items-center rounded-lg bg-primary text-primary-foreground">
            <UsersIcon className="size-4" aria-hidden="true" />
          </span>
          <span className="font-semibold tracking-tight">Groupify</span>
        </Link>

        <Link
          to="/admin"
          className="rounded-md px-2 py-1 text-sm text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        >
          Organiser
        </Link>
      </div>
    </header>
  )
}
