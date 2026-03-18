import { app, BrowserWindow, ipcMain } from 'electron'
import { createMainWindow } from './window'
import { Sidecar } from './sidecar'
import { IPC } from '@shared/ipc'
import type { SiloConnection, SetupParams } from '@shared/types'

const sidecar = new Sidecar()
let connection: SiloConnection | null = null
let mainWindow: BrowserWindow | null = null

function setupIpc(): void {
  ipcMain.handle(IPC.GET_CONNECTION, () => connection)

  ipcMain.handle(IPC.VAULT_EXISTS, () => sidecar.checkVaultExists())

  ipcMain.handle(IPC.SETUP, async (_, params: SetupParams) => {
    try {
      await sidecar.initVault(params)
      return { ok: true }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      return { ok: false, message }
    }
  })

  ipcMain.handle(IPC.VAULT_LIST, async (_, password: string) => {
    try {
      const keys = await sidecar.vaultList(password)
      return { ok: true, keys }
    } catch (err) {
      return { ok: false, message: err instanceof Error ? err.message : String(err) }
    }
  })

  ipcMain.handle(IPC.VAULT_GET_ALL, async (_, password: string, keys: string[]) => {
    try {
      const entries = await sidecar.vaultGetAll(password, keys)
      return { ok: true, entries }
    } catch (err) {
      return { ok: false, message: err instanceof Error ? err.message : String(err) }
    }
  })

  ipcMain.handle(IPC.VAULT_SET, async (_, password: string, key: string, value: string) => {
    try {
      await sidecar.vaultSet(password, key, value)
      return { ok: true }
    } catch (err) {
      return { ok: false, message: err instanceof Error ? err.message : String(err) }
    }
  })

  ipcMain.handle(IPC.VAULT_DELETE, async (_, password: string, key: string) => {
    try {
      await sidecar.vaultDelete(password, key)
      return { ok: true }
    } catch (err) {
      return { ok: false, message: err instanceof Error ? err.message : String(err) }
    }
  })

  ipcMain.handle(IPC.UNLOCK, async (_, password: string) => {
    try {
      connection = process.env.SILO_DEV_PORT
        ? sidecar.connectDev()
        : await sidecar.start(password)
      mainWindow?.webContents.send(IPC.READY, connection)
      return { ok: true }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      mainWindow?.webContents.send(IPC.ERROR, message)
      return { ok: false, message }
    }
  })
}

app.whenReady().then(() => {
  setupIpc()
  mainWindow = createMainWindow()

  // In dev mode auto-connect so the renderer skips vault/unlock screens
  if (process.env.SILO_DEV_PORT) {
    try {
      connection = sidecar.connectDev()
      mainWindow.webContents.once('did-finish-load', () => {
        mainWindow?.webContents.send(IPC.READY, connection)
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      mainWindow.webContents.once('did-finish-load', () => {
        mainWindow?.webContents.send(IPC.ERROR, message)
      })
    }
  }

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      mainWindow = createMainWindow()
    }
  })
})

app.on('before-quit', () => sidecar.kill())

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
