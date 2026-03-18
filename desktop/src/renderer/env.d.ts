import type { SiloConnection, UnlockResult, SetupParams, SetupResult } from '@shared/types'

declare global {
  interface Window {
    silo: {
      getConnection: () => Promise<SiloConnection | null>
      vaultExists: () => Promise<boolean>
      setup: (params: SetupParams) => Promise<SetupResult>
      unlock: (password: string) => Promise<UnlockResult>
      onReady: (cb: (conn: SiloConnection) => void) => void
      onError: (cb: (message: string) => void) => void
    }
  }
}
