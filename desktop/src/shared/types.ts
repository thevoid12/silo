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

export interface SetupParams {
  password: string
  provider: 'gemini' | 'openai'
  apiKey: string
}

export interface SetupResult {
  ok: boolean
  message?: string
}

export interface VaultListResult {
  ok: boolean
  keys?: string[]
  message?: string
}

export interface VaultGetAllResult {
  ok: boolean
  entries?: Record<string, string>
  message?: string
}

export interface VaultSetResult {
  ok: boolean
  message?: string
}

export interface VaultDeleteResult {
  ok: boolean
  message?: string
}

// SSE payloads — mirror Go's gateway/models/models.go
export interface TokenPayload { text: string }
export interface ToolCallPayload { tool: string; args: Record<string, unknown> }
export interface ToolResultPayload { tool: string; output: Record<string, unknown> }
export interface ApprovalRequiredPayload {
  request_id: string
  tool: string
  command: string
  args?: string[]
}
export interface DonePayload { session_id: string }
export interface ErrorPayload { message: string }

// Chat message model
export interface ToolCallEntry {
  id: string
  tool: string
  args: Record<string, unknown>
  result?: Record<string, unknown>
  pending: boolean
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  text: string
  toolCalls: ToolCallEntry[]
  streaming: boolean
  error?: string
}

// Settings — mirrors Go's SettingsResponse
export interface Settings {
  provider: string
  gemini_model: string
  openai_model: string
  max_iterations: number
  approval_mode: string
  shell_timeout_secs: number
}

// Session — mirrors Go's SessionResponse
export interface Session {
  id: string
  app_name: string
  user_id: string
  updated_at: string
}
