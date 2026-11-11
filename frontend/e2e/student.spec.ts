import { expect, test, type APIRequestContext } from '@playwright/test'
import {
  type AdminApi,
  type Topic,
  createTopic,
  registerStudent,
  signInAsAdmin,
  uniqueIndex,
  uniqueName,
  setRegistrationOpen,
  setTopicActive,
} from './helpers'

/**
 * The student flow, driven the way a student drives it: a real browser against
 * the real API. Nothing here stubs the network.
 */
test.describe('student registration', () => {
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

  async function newTopic(request: APIRequestContext, capacity: number): Promise<Topic> {
    const topic = await createTopic(request, admin, { name: uniqueName('E2E Topic'), capacity })
    created.push(topic.id)
    return topic
  }

  test('registers after a review step, and the confirmation survives a refresh', async ({
    page,
    request,
  }) => {
    const topic = await newTopic(request, 3)
    const index = uniqueIndex()

    await page.goto('/')
    await expect(page.getByText('Registration is open')).toBeVisible()

    await page.locator('label', { hasText: topic.name }).click()
    await page.getByLabel('Full name').fill('Akosua Nyarko')
    // Deliberately messy: the server normalises it, and the form says so.
    await page.getByLabel('Index number').fill(`  ${index.toLowerCase()} `)
    await expect(page.getByText('Saved as')).toBeVisible()

    await page.getByRole('button', { name: 'Review selection' }).click()
    await expect(page.getByText('Check and confirm')).toBeVisible()
    // The normalised form is what gets submitted, so it must be what is shown.
    await expect(page.getByText(index, { exact: false })).toBeVisible()

    await page.getByRole('button', { name: 'Confirm registration' }).click()
    await expect(page.getByText('You are registered')).toBeVisible()
    // The confirmation names the topic in its summary row.
    await expect(page.locator('dd').getByText(topic.name, { exact: true })).toBeVisible()

    // A reload must not look like a failed registration.
    await page.reload()
    await expect(page.getByText('You are registered')).toBeVisible()
  })

  test('refuses a second registration for the same index number', async ({ page, request }) => {
    const topic = await newTopic(request, 3)
    const index = uniqueIndex()
    await registerStudent(request, { topicId: topic.id, name: 'First Student', index })

    await page.goto('/')
    await page.locator('label', { hasText: topic.name }).click()
    await page.getByLabel('Full name').fill('Someone Else')
    await page.getByLabel('Index number').fill(index)
    await page.getByRole('button', { name: 'Review selection' }).click()
    await page.getByRole('button', { name: 'Confirm registration' }).click()

    await expect(page.getByText('This index number is already registered.')).toBeVisible()
    // Back on the form, not stuck on the review screen.
    await expect(page.getByLabel('Full name')).toBeVisible()
  })

  test('a full topic is shown but cannot be selected', async ({ page, request }) => {
    const topic = await newTopic(request, 1)
    await registerStudent(request, { topicId: topic.id })

    await page.goto('/')
    const card = page.locator('label', { hasText: topic.name })
    await expect(card.getByText('Full')).toBeVisible()
    await expect(card.locator('input[type=radio]')).toBeDisabled()
  })

  test('closing registration disables the form immediately', async ({ page, request }) => {
    await setRegistrationOpen(request, admin, false)

    await page.goto('/')
    await expect(page.getByText('Registration is closed').first()).toBeVisible()
    await expect(page.getByRole('button', { name: 'Review selection' })).toBeDisabled()
    await expect(page.locator('input[type=radio]').first()).toBeDisabled()

    await setRegistrationOpen(request, admin, true)
  })

  test('reports a topic that fills up during review and returns to the list', async ({
    page,
    request,
  }) => {
    // One seat, taken by somebody else after this student has reviewed.
    const topic = await newTopic(request, 1)

    await page.goto('/')
    await page.locator('label', { hasText: topic.name }).click()
    await page.getByLabel('Full name').fill('Unlucky Student')
    await page.getByLabel('Index number').fill(uniqueIndex())
    await page.getByRole('button', { name: 'Review selection' }).click()
    await expect(page.getByText('Check and confirm')).toBeVisible()

    await registerStudent(request, { topicId: topic.id })

    await page.getByRole('button', { name: 'Confirm registration' }).click()

    await expect(page.getByText(/filled up while you were reviewing/)).toBeVisible()
    await expect(page.getByLabel('Full name')).toBeVisible()
    // Nothing was reserved, so the confirmation screen must not appear.
    await expect(page.getByText('You are registered')).toBeHidden()
  })

  test('blocks an empty submit before it reaches the server', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Review selection' }).click()
    await expect(page.getByText('Enter your full name.')).toBeVisible()
    await expect(page.getByText('Check and confirm')).toBeHidden()
  })
})
