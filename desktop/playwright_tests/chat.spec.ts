/**
 * End-to-end tests for the chat and approval modal flows.
 *
 */

import { test, expect, _electron as electron } from '@playwright/test'
import { findLatestBuild, parseElectronApp } from 'electron-playwright-helpers'
import { join } from 'path'

function launchApp() {
  const latestBuild = findLatestBuild(join(__dirname, '../dist'))
  const appInfo = parseElectronApp(latestBuild)
  return electron.launch({
    args: [appInfo.main],
    env: {
      ...process.env,
      // Dev mode env vars skip vault/unlock and connect directly
      SILO_DEV_PORT: process.env.SILO_DEV_PORT ?? '',
      SILO_DEV_TOKEN: process.env.SILO_DEV_TOKEN ?? '',
    },
  })
}

test.describe('shell', () => {
  test('shows chat interface after auto-connect in dev mode', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')

    const app = await launchApp()
    const page = await app.firstWindow()

    await expect(page.locator('text=What can I help with?')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('textarea[placeholder="Message silo…"]')).toBeVisible()
    await expect(page.locator('text=Sessions')).toBeVisible()

    await app.close()
  })

  test('send button is disabled when input is empty', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')

    const app = await launchApp()
    const page = await app.firstWindow()

    await page.waitForSelector('textarea[placeholder="Message silo…"]')
    await expect(page.locator('button[type="submit"]')).toBeDisabled()

    await app.close()
  })

  test('send button enables when input has text', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')

    const app = await launchApp()
    const page = await app.firstWindow()

    const textarea = page.locator('textarea[placeholder="Message silo…"]')
    await textarea.waitFor()
    await textarea.fill('hello')
    await expect(page.locator('button[type="submit"]')).toBeEnabled()

    await app.close()
  })
})

test.describe('chat', () => {
  test('sends a message and receives a response', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT and a running silo server')
    test.setTimeout(90_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    const textarea = page.locator('textarea[placeholder="Message silo…"]')
    await textarea.waitFor({ timeout: 10_000 })

    await textarea.fill('say exactly: hello')
    await page.keyboard.press('Enter')

    // user bubble appears immediately and textarea clears
    await expect(page.locator('text=say exactly: hello')).toBeVisible()
    await expect(textarea).toHaveValue('')

    // send button re-enables once streaming finishes
    await expect(page.locator('button[type="submit"]')).toBeEnabled({ timeout: 60_000 })

    await app.close()
  })

  test('new session button resets the chat', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')

    const app = await launchApp()
    const page = await app.firstWindow()

    await page.waitForSelector('textarea[placeholder="Message silo…"]')

    // click the + new session button
    await page.locator('button[title="New session"]').click()

    // empty state should show again
    await expect(page.locator('text=What can I help with?')).toBeVisible()

    await app.close()
  })
})

test.describe('unlock screen', () => {
  test('shows password input when no dev mode env vars', async () => {
    test.skip(!!process.env.SILO_DEV_PORT, 'skipped in dev mode — unlock screen is bypassed')

    const app = await launchApp()
    const page = await app.firstWindow()

    // Either unlock screen or setup screen appears (depending on vault state)
    const hasUnlock = await page.locator('input[type="password"]').isVisible({ timeout: 5_000 }).catch(() => false)
    expect(hasUnlock).toBe(true)

    await app.close()
  })
})

test.describe('approval modal', () => {
  test('modal is absent when no approval is pending', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')

    const app = await launchApp()
    const page = await app.firstWindow()

    await page.waitForSelector('textarea[placeholder="Message silo…"]')
    await expect(page.locator('text=Tool Approval Required')).not.toBeVisible()

    await app.close()
  })
})
