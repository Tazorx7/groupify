import { expect, type APIRequestContext } from '@playwright/test'

export const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'rep@example.edu'
export const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'change-me-please-123'

/** The two credentials an admin call needs: session cookie and CSRF token. */
export interface AdminApi {
  cookie: string
  csrf: string
}

/** Everything these helpers create is stamped so repeated runs never collide. */
let sequence = 0
function stamp(): string {
  sequence += 1
  return `${Date.now().toString(36)}${sequence}`
}

export function uniqueName(prefix: string): string {
  return `${prefix} ${stamp()}`
}

/** Normalised server-side; unique so a re-run cannot hit the duplicate rule. */
export function uniqueIndex(): string {
  return `E2E/${stamp().toUpperCase()}`
}

export async function signInAsAdmin(request: APIRequestContext): Promise<AdminApi> {
  const response = await request.post('/api/admin/login', {
    data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  })
  expect(response.status(), 'admin login via API').toBe(200)
  const body = (await response.json()) as { csrf_token: string }
  const cookie = (response.headers()['set-cookie'] ?? '').split(';')[0]
  expect(cookie, 'session cookie').toContain('groupify_admin=')
  return { cookie, csrf: body.csrf_token }
}

export function adminHeaders(admin: AdminApi): Record<string, string> {
  return { Cookie: admin.cookie, 'X-CSRF-Token': admin.csrf }
}

export async function setRegistrationOpen(
  request: APIRequestContext,
  admin: AdminApi,
  open: boolean,
): Promise<void> {
  const response = await request.patch('/api/admin/settings', {
    headers: adminHeaders(admin),
    data: { registration_open: open },
  })
  expect(response.status(), `set registration_open=${open}`).toBe(200)
}

export async function registrationIsOpen(request: APIRequestContext): Promise<boolean> {
  const response = await request.get('/api/registration-status')
  const body = (await response.json()) as { registration_open: boolean }
  return body.registration_open
}

export interface Topic {
  id: number
  name: string
  capacity: number
  registered_count: number
  remaining_spaces: number
  status: 'available' | 'full' | 'unavailable'
}

export async function createTopic(
  request: APIRequestContext,
  admin: AdminApi,
  input: { name: string; capacity: number; description?: string },
): Promise<Topic> {
  const response = await request.post('/api/admin/topics', {
    headers: adminHeaders(admin),
    data: {
      name: input.name,
      capacity: input.capacity,
      description: input.description ?? 'Created by the end-to-end suite.',
    },
  })
  expect(response.status(), 'create topic via API').toBe(201)
  return (await response.json()) as Topic
}

export async function setTopicActive(
  request: APIRequestContext,
  admin: AdminApi,
  id: number,
  active: boolean,
): Promise<void> {
  const response = await request.post(`/api/admin/topics/${id}/${active ? 'activate' : 'deactivate'}`, {
    headers: adminHeaders(admin),
  })
  expect(response.status(), `set topic ${id} active=${active}`).toBe(200)
}

export async function publicTopics(request: APIRequestContext): Promise<Topic[]> {
  const response = await request.get('/api/topics')
  const body = (await response.json()) as { topics: Topic[] }
  return body.topics
}

/** Fills a topic straight through the API, the way another student would. */
export async function registerStudent(
  request: APIRequestContext,
  input: { topicId: number; name?: string; index?: string },
): Promise<void> {
  const response = await request.post('/api/registrations', {
    data: {
      full_name: input.name ?? 'Background Student',
      index_number: input.index ?? uniqueIndex(),
      topic_id: input.topicId,
    },
  })
  expect(response.status(), 'register via API').toBe(201)
}

/** Deactivated at the end of a run so the student page stays demo-clean. */
export async function retireTopics(
  request: APIRequestContext,
  admin: AdminApi,
  ids: number[],
): Promise<void> {
  for (const id of ids) {
    await request.post(`/api/admin/topics/${id}/deactivate`, { headers: adminHeaders(admin) })
  }
}
