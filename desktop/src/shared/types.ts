export interface SiloConnection {
  port: number
  token: string
}

export type AppState =
  | { status: 'locked' }
  | { status: 'connecting' }
  | { status: 'ready'; connection: SiloConnection }
  | { status: 'error'; message: string }

export interface UnlockResult {
  ok: boolean
  message?: string
}
