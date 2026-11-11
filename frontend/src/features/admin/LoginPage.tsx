import { Loader2Icon, LockIcon } from 'lucide-react'
import { useState } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError } from '@/lib/api'
import { useAdminSession, useLogin } from './api'

export default function LoginPage() {
  const session = useAdminSession()
  const login = useLogin()
  const navigate = useNavigate()
  const location = useLocation()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  const redirectTo = (location.state as { from?: string } | null)?.from ?? '/admin'

  if (session.isSuccess) {
    return <Navigate to={redirectTo} replace />
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await login.mutateAsync({ email, password })
      navigate(redirectTo, { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'invalid_credentials') {
          setError('That email and password do not match an organiser account.')
        } else if (err.code === 'rate_limited') {
          setError('Too many sign-in attempts. Wait a minute and try again.')
        } else if (err.code === 'insecure_transport') {
          // Production refuses to hand out a session over plain HTTP, which is
          // otherwise indistinguishable from "sign-in does nothing".
          setError(
            'This deployment only signs in over HTTPS. Open the site at its https:// address.',
          )
        } else {
          setError(err.message)
        }
      } else {
        setError('Could not reach the server. Check your connection and try again.')
      }
    }
  }

  return (
    <div className="flex min-h-dvh flex-col justify-center bg-muted/40 px-4 py-12">
      <div className="mx-auto w-full max-w-sm">
        <Link
          to="/"
          className="mb-6 flex items-center justify-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground">
            <LockIcon className="size-4" aria-hidden="true" />
          </span>
          <span className="text-lg font-semibold tracking-tight">Groupify</span>
        </Link>

        <Card>
          <CardHeader>
            <CardTitle>
              <h1 className="text-xl font-semibold tracking-tight">Organiser sign-in</h1>
            </CardTitle>
            <CardDescription>
              Only the course representative can manage topics and registrations.
            </CardDescription>
          </CardHeader>

          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <Alert variant="destructive">
                  <AlertTitle>Sign-in failed</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  autoComplete="username"
                  className="h-10"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  className="h-10"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                />
              </div>

              <Button
                type="submit"
                size="lg"
                className="h-10 w-full"
                disabled={login.isPending || !email || !password}
              >
                {login.isPending ? (
                  <>
                    <Loader2Icon className="animate-spin" aria-hidden="true" />
                    Signing in…
                  </>
                ) : (
                  'Sign in'
                )}
              </Button>
            </form>
          </CardContent>
        </Card>

        <p className="mt-4 text-center text-xs text-muted-foreground">
          <Link to="/" className="underline-offset-4 hover:underline">
            Back to the student page
          </Link>
        </p>
      </div>
    </div>
  )
}
