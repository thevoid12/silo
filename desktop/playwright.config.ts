import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: 'playwright_tests',
  timeout: 30000,
  workers: 1,
  retries: 0,
  use: { trace: 'on-first-retry' },
})
