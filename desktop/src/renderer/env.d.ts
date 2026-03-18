import type { SiloConnection, UnlockResult } from '@shared/types'

declare global {
  interface Window {
    silo: {
      getConnection: () => Promise<SiloConnection | null>
      unlock: (password: string) => Promise<UnlockResult>
      onReady: (cb: (conn: SiloConnection) => void) => void
      onError: (cb: (message: string) => void) => void
    }
  }
}
