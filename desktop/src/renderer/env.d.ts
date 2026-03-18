import type { SiloConnection, UnlockResult, SetupParams, SetupResult, VaultListResult, VaultGetAllResult, VaultSetResult, VaultDeleteResult } from '@shared/types'

declare global {
  interface Window {
    silo: {
      getConnection: () => Promise<SiloConnection | null>
      vaultExists: () => Promise<boolean>
      setup: (params: SetupParams) => Promise<SetupResult>
      unlock: (password: string) => Promise<UnlockResult>
      vaultList: (password: string) => Promise<VaultListResult>
      vaultGetAll: (password: string, keys: string[]) => Promise<VaultGetAllResult>
      vaultSet: (password: string, key: string, value: string) => Promise<VaultSetResult>
      vaultDelete: (password: string, key: string) => Promise<VaultDeleteResult>
      onReady: (cb: (conn: SiloConnection) => void) => void
      onError: (cb: (message: string) => void) => void
    }
  }
}
