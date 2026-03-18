import { test, expect, _electron as electron } from '@playwright/test'
import { findLatestBuild, parseElectronApp } from 'electron-playwright-helpers'
import { join } from 'path'

function launchApp() {
  const latestBuild = findLatestBuild(join(__dirname, '../dist'))
  const appInfo = parseElectronApp(latestBuild)
  return electron.launch({ args: [appInfo.main] })
}

test('app launches and shows shell when dev mode is active', async () => {
  test.skip(!process.env.SILO_DEV_PORT, 'requires SILO_DEV_PORT')

  const app = await launchApp()
  const page = await app.firstWindow()

  await expect(page.locator('text=silo')).toBeVisible({ timeout: 10_000 })
  await expect(page.locator('text=What can I help with?')).toBeVisible({ timeout: 10_000 })

  await app.close()
})

test('app launches and shows unlock or setup screen without dev mode', async () => {
  test.skip(!!process.env.SILO_DEV_PORT, 'skipped in dev mode')

  const app = await launchApp()
  const page = await app.firstWindow()

  await expect(page.locator('text=silo')).toBeVisible({ timeout: 10_000 })
  await expect(page.locator('input[type="password"]')).toBeVisible({ timeout: 10_000 })

  await app.close()
})
