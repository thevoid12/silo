import { test, expect, _electron as electron } from '@playwright/test'
import { findLatestBuild, parseElectronApp } from 'electron-playwright-helpers'
import { join } from 'path'

test('app launches and shows unlock screen', async () => {
  const latestBuild = findLatestBuild(join(__dirname, '../dist'))
  const appInfo = parseElectronApp(latestBuild)

  const electronApp = await electron.launch({ args: [appInfo.main] })
  const page = await electronApp.firstWindow()

  await expect(page.locator('text=silo')).toBeVisible()
  await expect(page.locator('input[type="password"]')).toBeVisible()

  await electronApp.close()
})
