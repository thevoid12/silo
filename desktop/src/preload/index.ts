import { contextBridge, ipcRenderer } from 'electron'
import { IPC } from '@shared/ipc'
import type { SiloConnection, UnlockResult, SetupParams, SetupResult, VaultListResult, VaultGetAllResult, VaultSetResult, VaultDeleteResult } from '@shared/types'

contextBridge.exposeInMainWorld('silo', {
  getConnection: (): Promise<SiloConnection | null> =>
    ipcRenderer.invoke(IPC.GET_CONNECTION),

  vaultExists: (): Promise<boolean> =>
    ipcRenderer.invoke(IPC.VAULT_EXISTS),

  setup: (params: SetupParams): Promise<SetupResult> =>
    ipcRenderer.invoke(IPC.SETUP, params),

  unlock: (password: string): Promise<UnlockResult> =>
    ipcRenderer.invoke(IPC.UNLOCK, password),

  vaultList: (password: string): Promise<VaultListResult> =>
    ipcRenderer.invoke(IPC.VAULT_LIST, password),

  vaultGetAll: (password: string, keys: string[]): Promise<VaultGetAllResult> =>
    ipcRenderer.invoke(IPC.VAULT_GET_ALL, password, keys),

  vaultSet: (password: string, key: string, value: string): Promise<VaultSetResult> =>
    ipcRenderer.invoke(IPC.VAULT_SET, password, key, value),

  vaultDelete: (password: string, key: string): Promise<VaultDeleteResult> =>
    ipcRenderer.invoke(IPC.VAULT_DELETE, password, key),

  onReady: (cb: (conn: SiloConnection) => void): void => {
    ipcRenderer.on(IPC.READY, (_, conn: SiloConnection) => cb(conn))
  },

  onError: (cb: (message: string) => void): void => {
    ipcRenderer.on(IPC.ERROR, (_, message: string) => cb(message))
  },
})
