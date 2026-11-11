// Thin typed wrapper over fetch.
//
// Two jobs: turn the backend's uniform error envelope into a throwable
// ApiError the UI can branch on by `code`, and attach the CSRF token to
// mutating admin requests. Everything is same-origin (`/api` via the Vite
// proxy) so cookies ride along without CORS.

export interface ApiErrorPayload {
  code: string
  message: string
  fields?: Record<string, string>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields?: Record<string, string>

  constructor(status: number, payload: ApiErrorPayload) {
    super(payload.message)
    this.name = 'ApiError'
    this.status = status
    this.code = payload.code
    this.fields = payload.fields
  }
}

// Network failures and malformed bodies still need a code so callers do not
// have to special-case them.
const NETWORK_ERROR: ApiErrorPayload = {
  code: 'network_error',
  message: 'Could not reach the server. Check your connection and try again.',
}

let csrfToken: string | null = null

export function setCsrfToken(token: string | null) {
  csrfToken = token
}

export function getCsrfToken(): string | null {
  return csrfToken
}

type Method = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'

interface RequestOptions {
  method?: Method
  body?: unknown
  signal?: AbortSignal
  headers?: Record<string, string>
}

/** Called when the server reports the session is gone, so the app can react. */
let onUnauthenticated: (() => void) | null = null

export function setUnauthenticatedHandler(handler: (() => void) | null) {
  onUnauthenticated = handler
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = options.method ?? 'GET'
  const headers: Record<string, string> = { Accept: 'application/json', ...options.headers }

  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (method !== 'GET' && csrfToken) {
    headers['X-CSRF-Token'] = csrfToken
  }

  let response: Response
  try {
    response = await fetch(`/api${path}`, {
      method,
      headers,
      credentials: 'include',
      signal: options.signal,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ApiError(0, NETWORK_ERROR)
  }

  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()
  const payload = text ? safeParse(text) : null

  if (!response.ok) {
    const body: ApiErrorPayload =
      payload && typeof payload === 'object' && 'error' in payload
        ? (payload as { error: ApiErrorPayload }).error
        : { code: 'unexpected_error', message: 'Something went wrong. Please try again.' }

    // A 401 from the session probe or the login call is a normal "signed out"
    // answer, not an expired session: treating it as one would make the probe
    // fight with the redirect it causes, refetching in a loop.
    if (
      response.status === 401 &&
      path !== '/admin/login' &&
      path !== '/admin/session'
    ) {
      onUnauthenticated?.()
    }
    throw new ApiError(response.status, body)
  }

  return payload as T
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

/** Builds a query string, dropping empty values. */
export function toQuery(params: Record<string, string | number | null | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined || value === '') continue
    search.set(key, String(value))
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

/**
 * Triggers a browser download for an authenticated GET endpoint.
 *
 * The session cookie is HttpOnly, so the file cannot be fetched and saved by
 * script without losing the cookie in some browsers — a same-origin navigation
 * lets the browser handle both the auth and the Content-Disposition filename.
 */
export function downloadFile(url: string) {
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.rel = 'noopener'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}
