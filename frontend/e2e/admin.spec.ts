import { devices, expect, test } from '@playwright/test'
import { readFileSync } from 'node:fs'
import {
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  type AdminApi,
  createTopic,
  publicTopics,
  registerStudent,
  setRegistrationOpen,
  setTopicActive,
  signInAsAdmin,
  uniqueIndex,
  uniqueName,
} from './helpers'

const SHOTS = 'test-results'

test.describe('organiser console', () => {
  let admin: AdminApi
  const created: number[] = []

  test.beforeEach(async ({ request }) => {
    admin = await signInAsAdmin(request)
    await setRegistrationOpen(request, admin, true)
  })

  test.afterAll(async ({ request }) => {
    if (!admin) return
    for (const id of created) await setTopicActive(request, admin, id, false)
  })

  async function signIn(page: import('@playwright/test').Page) {
    await page.getByLabel('Email').fill(ADMIN_EMAIL)
    await page.getByLabel('Password').fill(ADMIN_PASSWORD)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  }

  test('is gated, rejects a wrong password, and keeps the session across a reload', async ({
    page,
  }) => {
    await page.goto('/admin')
    await expect(page.getByRole('heading', { name: 'Organiser sign-in' })).toBeVisible()
    expect(new URL(page.url()).pathname).toBe('/admin/login')

    await page.getByLabel('Email').fill(ADMIN_EMAIL)
    await page.getByLabel('Password').fill('definitely-not-the-password')
    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page.getByText('Sign-in failed')).toBeVisible()

    await page.getByLabel('Password').fill(ADMIN_PASSWORD)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()

    await page.reload()
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  })

  test('creates a topic and refuses a capacity below the members already in it', async ({
    page,
    request,
  }) => {
    const name = uniqueName('E2E Created')

    await page.goto('/admin/login')
    await signIn(page)
    await page.getByRole('link', { name: 'Topics', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Topics' })).toBeVisible()

    await page.getByRole('button', { name: 'New topic' }).click()
    await page.getByLabel('Name').fill(name)
    await page.getByLabel('Description').fill('Created through the console by the e2e suite.')
    await page.getByLabel('Capacity').fill('5')
    await page.getByRole('button', { name: 'Create topic' }).click()

    const row = page.locator('tr', { hasText: name })
    await expect(row).toBeVisible()
    const topic = (await publicTopics(request)).find((t) => t.name === name)
    expect(topic, 'new topic is public immediately').toBeTruthy()
    created.push(topic!.id)

    // Give it two members, then try to shrink past them.
    await registerStudent(request, { topicId: topic!.id })
    await registerStudent(request, { topicId: topic!.id })

    await row.getByRole('button', { name: 'Edit' }).click()
    await page.getByLabel('Capacity').fill('1')
    await page.getByRole('button', { name: 'Save changes' }).click()
    await expect(page.getByText(/cannot be lower than/)).toBeVisible()

    // The server refused, so the topic still has room for five.
    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByRole('dialog')).toBeHidden()
    expect((await publicTopics(request)).find((t) => t.id === topic!.id)?.capacity).toBe(5)
  })

  test('deactivating a topic removes it from the student API and hides it again', async ({
    page,
    request,
  }) => {
    const topic = await createTopic(request, admin, { name: uniqueName('E2E Hide'), capacity: 4 })
    created.push(topic.id)

    await page.goto('/admin/login')
    await signIn(page)
    await page.getByRole('link', { name: 'Topics', exact: true }).click()

    const row = page.locator('tr', { hasText: topic.name })
    await row.getByRole('button', { name: 'Deactivate' }).click()
    await page.getByRole('button', { name: 'Deactivate topic' }).click()
    await expect(page.getByText('is hidden from students')).toBeVisible()

    expect((await publicTopics(request)).some((t) => t.id === topic.id)).toBe(false)
  })

  test('searches, downloads both export formats, and removes a registration', async ({
    page,
    request,
  }) => {
    const topic = await createTopic(request, admin, { name: uniqueName('E2E Roster'), capacity: 5 })
    created.push(topic.id)

    const target = `Roster Student ${Date.now().toString(36)}`
    const targetIndex = uniqueIndex()
    await registerStudent(request, { topicId: topic.id, name: target, index: targetIndex })
    // A decoy, so "the search narrowed it" means something.
    await registerStudent(request, { topicId: topic.id, name: 'Decoy Student' })

    await page.goto('/admin/login')
    await signIn(page)
    await page.getByRole('link', { name: 'Registrations', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Registrations' })).toBeVisible()

    await page.getByLabel('Search registrations').fill(target)
    await expect(page.locator('tbody tr')).toHaveCount(1)
    await expect(page.locator('tbody tr')).toContainText(targetIndex)

    const csv = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'CSV' }).click(),
    ]).then(([download]) => download)
    expect(csv.suggestedFilename()).toMatch(/\.csv$/)

    const csvPath = `${SHOTS}/${csv.suggestedFilename()}`
    await csv.saveAs(csvPath)
    const csvText = readFileSync(csvPath, 'utf8')
    expect(csvText).toContain('index_number,full_name,topic,registered_at')
    // The export honours the filter, so the decoy must not be in it.
    expect(csvText).toContain(targetIndex)
    expect(csvText).not.toContain('Decoy Student')

    const xlsx = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: /Excel/ }).click(),
    ]).then(([download]) => download)
    expect(xlsx.suggestedFilename()).toMatch(/\.xlsx$/)

    await page.getByRole('button', { name: `Remove registration for ${target}` }).click()
    await page.getByRole('button', { name: 'Remove registration' }).click()
    await expect(page.getByText(/The place is free again/)).toBeVisible()

    const roster = await (await request.get('/api/admin/registrations?limit=200', {
      headers: { Cookie: admin.cookie },
    })).json() as { items: Array<{ full_name: string }> }
    expect(roster.items.some((item) => item.full_name === target)).toBe(false)
  })

  test('the registration period switch is enforced by the server, not the UI', async ({
    page,
  }) => {
    await page.goto('/admin/login')
    await signIn(page)
    await page.getByRole('link', { name: 'Registration period', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Registration period' })).toBeVisible()

    const dialog = page.getByRole('dialog')
    await page.getByRole('button', { name: 'Close registration' }).click()
    await dialog.getByRole('button', { name: 'Close registration' }).click()
    await expect(dialog).toBeHidden()

    const closed = await (await page.request.get('/api/registration-status')).json()
    expect(closed.registration_open).toBe(false)

    await page.getByRole('button', { name: 'Open registration' }).click()
    await dialog.getByRole('button', { name: 'Open registration' }).click()
    await expect(dialog).toBeHidden()

    const reopened = await (await page.request.get('/api/registration-status')).json()
    expect(reopened.registration_open).toBe(true)
  })

  test('the audit log records the change and never carries student identity', async ({
    page,
    request,
  }) => {
    const topic = await createTopic(request, admin, { name: uniqueName('E2E Audit'), capacity: 3 })
    created.push(topic.id)
    const studentName = `Audit Student ${Date.now().toString(36)}`
    const studentIndex = uniqueIndex()
    await registerStudent(request, { topicId: topic.id, name: studentName, index: studentIndex })

    await page.goto('/admin/login')
    await signIn(page)
    await page.getByRole('link', { name: 'Audit log', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Audit log' })).toBeVisible()

    const entries = page.locator('li')
    await expect(entries.filter({ hasText: 'Registration created' }).first()).toBeVisible()
    await expect(entries.filter({ hasText: 'Topic created' }).first()).toBeVisible()

    const text = (await page.locator('main').innerText()).toLowerCase()
    expect(text).not.toContain(studentName.toLowerCase())
    expect(text).not.toContain(studentIndex.toLowerCase())
  })

  test('is usable on a phone: navigation works and controls are thumb-sized', async ({
    browser,
  }) => {
    // A real phone profile, not a narrow desktop window: `pointer: coarse` is
    // what triggers the expanded hit areas, and only device emulation sets it.
    const context = await browser.newContext({ ...devices['Pixel 7'] })
    const page = await context.newPage()

    await page.goto('/admin/login')
    await signIn(page)
    expect(await page.evaluate(() => matchMedia('(pointer: coarse)').matches)).toBe(true)

    await page.getByRole('link', { name: 'Registrations', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Registrations' })).toBeVisible()

    const small = await page.evaluate(() => {
      const controls = Array.from(
        document.querySelectorAll<HTMLElement>('[data-slot="button"], [data-slot="input"], select'),
      )
      return controls
        .map((el) => ({
          label: (el.getAttribute('aria-label') ?? el.textContent ?? '').trim().slice(0, 32),
          height: Math.round(el.getBoundingClientRect().height),
        }))
        .filter((control) => control.height > 0 && control.height < 44)
    })
    expect(small, `controls under 44px: ${JSON.stringify(small)}`).toHaveLength(0)

    await context.close()
  })
})
