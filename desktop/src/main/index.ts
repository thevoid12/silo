import { app, BrowserWindow, ipcMain } from 'electron'
import { createMainWindow } from './window'
import { Sidecar } from './sidecar'
import { IPC } from '@shared/ipc'
import type { SiloConnection } from '@shared/types'

const sidecar = new Sidecar()
let connection: SiloConnection | null = null
let mainWindow: BrowserWindow | null = null

function setupIpc(): void {
  ipcMain.handle(IPC.GET_CONNECTION, () => connection)

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
