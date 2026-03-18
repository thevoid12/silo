import { contextBridge, ipcRenderer } from 'electron'
import { IPC } from '@shared/ipc'
import type { SiloConnection, UnlockResult, SetupParams, SetupResult } from '@shared/types'

contextBridge.exposeInMainWorld('silo', {
  getConnection: (): Promise<SiloConnection | null> =>
    ipcRenderer.invoke(IPC.GET_CONNECTION),

  vaultExists: (): Promise<boolean> =>
    ipcRenderer.invoke(IPC.VAULT_EXISTS),

  setup: (params: SetupParams): Promise<SetupResult> =>
    ipcRenderer.invoke(IPC.SETUP, params),

  unlock: (password: string): Promise<UnlockResult> =>
    ipcRenderer.invoke(IPC.UNLOCK, password),

  onReady: (cb: (conn: SiloConnection) => void): void => {
    ipcRenderer.on(IPC.READY, (_, conn: SiloConnection) => cb(conn))
  },

  onError: (cb: (message: string) => void): void => {
    ipcRenderer.on(IPC.ERROR, (_, message: string) => cb(message))
  },
})
