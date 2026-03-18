import { test, expect, _electron as electron, Page } from '@playwright/test'
import { findLatestBuild, parseElectronApp } from 'electron-playwright-helpers'
import { join } from 'path'

// App startup (IPC connect + React render) can take a few seconds in CI
const APP_READY_TIMEOUT = 10_000
// All vault UI interactions are synchronous React state — should be instant
const UI_TIMEOUT = 2_000
// PTY-driven vault CLI ops (list, get, set, delete) run a subprocess each time
const VAULT_OP_TIMEOUT = 15_000

function launchApp() {
  const latestBuild = findLatestBuild(join(__dirname, '../dist'))
  const appInfo = parseElectronApp(latestBuild)
  return electron.launch({
    args: [appInfo.main],
    env: {
      ...process.env,
      SILO_DEV_PORT: process.env.SILO_DEV_PORT ?? '',
      SILO_DEV_TOKEN: process.env.SILO_DEV_TOKEN ?? '',
    },
  })
}

async function waitForShell(page: Page) {
  await page.waitForSelector('textarea[placeholder="Message silo…"]', { timeout: APP_READY_TIMEOUT })
}

async function openVault(page: Page) {
  await page.locator('button[title="Open vault"]').click({ timeout: UI_TIMEOUT })
  await expect(page.locator('text=Enter vault password to view and edit secrets')).toBeVisible({ timeout: UI_TIMEOUT })
}

test.describe('vault panel', () => {
  test('vault button opens the vault panel with password prompt', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.setTimeout(15_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await expect(page.locator('input[type="password"]')).toBeVisible({ timeout: UI_TIMEOUT })

    await app.close()
  })

  test('vault panel closes when clicking the X button', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.setTimeout(15_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await page.locator('button:has-text("✕")').click()
    await expect(page.locator('text=Enter vault password to view and edit secrets')).not.toBeVisible({ timeout: UI_TIMEOUT })

    await app.close()
  })

  test('vault panel closes when clicking outside the panel', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.setTimeout(15_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await page.mouse.click(5, 5)
    await expect(page.locator('text=Enter vault password to view and edit secrets')).not.toBeVisible({ timeout: UI_TIMEOUT })

    await app.close()
  })

  test('vault unlock button is disabled when password is empty', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.setTimeout(15_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await expect(page.locator('button:has-text("Unlock vault")')).toBeDisabled({ timeout: UI_TIMEOUT })

    await app.close()
  })

  test('vault unlock button enables when password is entered', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.setTimeout(15_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await page.locator('input[type="password"]').fill('somepassword')
    await expect(page.locator('button:has-text("Unlock vault")')).toBeEnabled({ timeout: UI_TIMEOUT })

    await app.close()
  })

  test('vault shows secrets after correct password', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.skip(!process.env.VAULT_PASSWORD, 'requires VAULT_PASSWORD env var')
    test.setTimeout(30_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await page.locator('input[type="password"]').fill(process.env.VAULT_PASSWORD!)
    await page.locator('button:has-text("Unlock vault")').click()

    await expect(page.locator('text=+ Add secret')).toBeVisible({ timeout: VAULT_OP_TIMEOUT })

    await app.close()
  })

  test('vault shows error on wrong password', async () => {
    test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')
    test.setTimeout(30_000)

    const app = await launchApp()
    const page = await app.firstWindow()

    await waitForShell(page)
    await openVault(page)

    await page.locator('input[type="password"]').fill('wrongpassword12345')
    await page.locator('button:has-text("Unlock vault")').click()

    await expect(page.locator('input[type="password"]')).toBeVisible({ timeout: VAULT_OP_TIMEOUT })

    await app.close()
  })
})
