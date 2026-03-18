import { contextBridge, ipcRenderer } from 'electron'
import { IPC } from '@shared/ipc'
import type { SiloConnection, UnlockResult } from '@shared/types'

contextBridge.exposeInMainWorld('silo', {
  getConnection: (): Promise<SiloConnection | null> =>
    ipcRenderer.invoke(IPC.GET_CONNECTION),

  unlock: (password: string): Promise<UnlockResult> =>
    ipcRenderer.invoke(IPC.UNLOCK, password),

  onReady: (cb: (conn: SiloConnection) => void): void => {
    ipcRenderer.on(IPC.READY, (_, conn: SiloConnection) => cb(conn))
  },

  onError: (cb: (message: string) => void): void => {
    ipcRenderer.on(IPC.ERROR, (_, message: string) => cb(message))
  },
})
