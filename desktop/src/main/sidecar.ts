import { spawn, ChildProcess } from 'child_process'
import { createServer, AddressInfo } from 'net'
import { existsSync } from 'fs'
import { join } from 'path'
import { homedir } from 'os'
import { randomBytes } from 'crypto'
import { app } from 'electron'
import type { SiloConnection, SetupParams } from '@shared/types'

// getFreePort binds to port 0 and returns the OS-assigned port
function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = createServer()
    srv.listen(0, '127.0.0.1', () => {
      const { port } = srv.address() as AddressInfo
      srv.close(() => resolve(port))
    })
    srv.on('error', reject)
  })
}

// getBinaryPath resolves the silo binary for packaged, Playwright test, or dev mode
function getBinaryPath(): string {
  const ext = process.platform === 'win32' ? '.exe' : ''
  if (process.env.SILO_BIN) return process.env.SILO_BIN
  const sidecar = join(process.resourcesPath, 'sidecar', `silo${ext}`)
  if (app.isPackaged || existsSync(sidecar)) return sidecar
  // dev: binary is built in the repo root, one level above desktop/
  return join(app.getAppPath(), '..', `silo${ext}`)
}

// pollHealth retries GET /health until it succeeds or times out
async function pollHealth(port: number, token: string, timeoutMs = 10000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/health`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok) return
    } catch { /* not ready yet */ }
    await new Promise(r => setTimeout(r, 200))
  }
  throw new Error('silo server did not become ready in time')
}

// ptyRun spawns a command via PTY and drives it through a sequence of prompt/response pairs
async function ptyRun(
  bin: string,
  args: string[],
  dialogue: Array<{ trigger: string; response: string }>,
): Promise<void> {
  const pty = await import('node-pty')
  return new Promise((resolve, reject) => {
    const term = pty.spawn(bin, args, { name: 'xterm', cols: 80, rows: 24, env: process.env as Record<string, string> })
    let buf = ''
    let step = 0
    let done = false

    term.onData((data: string) => {
      buf += data
      if (step < dialogue.length && buf.includes(dialogue[step].trigger)) {
        const r = dialogue[step].response
        step++
        buf = ''
        term.write(r + '\r')
      }
    })

    term.onExit(({ exitCode }: { exitCode: number }) => {
      done = true
      exitCode === 0 ? resolve() : reject(new Error(`silo vault command exited with code ${exitCode}`))
    })

    setTimeout(() => { if (!done) { term.kill(); reject(new Error('vault operation timed out')) } }, 20000)
  })
}

// spawnVault runs a vault command with password (and optional set-value) via env vars
function spawnVault(bin: string, args: string[], password: string, setValue?: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const env: Record<string, string> = { ...(process.env as Record<string, string>), SILO_VAULT_PASSWORD: password }
    if (setValue !== undefined) env.SILO_VAULT_SET_VALUE = setValue
    const proc = spawn(bin, args, { env, stdio: ['ignore', 'pipe', 'pipe'] })
    let out = ''
    let err = ''
    proc.stdout?.on('data', (d: Buffer) => { out += d.toString() })
    proc.stderr?.on('data', (d: Buffer) => { err += d.toString() })
    proc.on('exit', code => {
      code === 0 ? resolve(out.trim()) : reject(new Error(err.trim() || `vault command exited with code ${code}`))
    })
    proc.on('error', reject)
  })
}

export class Sidecar {
  private proc: ChildProcess | null = null

  // start spawns the silo binary in desktop mode and returns the connection once ready
  async start(vaultPassword: string): Promise<SiloConnection> {
    const port = await getFreePort()
    const bin = getBinaryPath()
    let token = ''

    this.proc = spawn(bin, ['start', '--serve', '--desktop-mode', '--port', String(port)], {
      env: { ...process.env, SILO_VAULT_PASSWORD: vaultPassword },
      stdio: ['ignore', 'pipe', 'pipe'],
    })

    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(
        () => reject(new Error('silo startup timed out')),
        15000
      )
      let stderr = ''

      this.proc!.stdout?.on('data', (chunk: Buffer) => {
        const line = chunk.toString().trim()
        if (line.startsWith('token:')) {
          token = line.slice('token:'.length)
          clearTimeout(timeout)
          resolve()
        }
      })

      this.proc!.stderr?.on('data', (chunk: Buffer) => { stderr += chunk.toString() })

      this.proc!.on('error', err => { clearTimeout(timeout); reject(err) })
      this.proc!.on('exit', code => {
        clearTimeout(timeout)
        if (!token) reject(new Error(stderr.trim() || `silo exited with code ${code}`))
      })
    })

    await pollHealth(port, token)
    return { port, token }
  }

  // checkVaultExists returns true if the vault file exists on disk
  checkVaultExists(): boolean {
    return existsSync(join(homedir(), '.silo', 'vault.enc'))
  }

  // initVault creates the vault and stores provider, api key, and gateway token
  async initVault({ password, provider, apiKey }: SetupParams): Promise<void> {
    const bin = getBinaryPath()
    const token = randomBytes(32).toString('hex')
    const env = { ...(process.env as Record<string, string>), SILO_VAULT_INIT_PASSWORD: password }

    await new Promise<void>((resolve, reject) => {
      const proc = spawn(bin, ['vault', 'init'], { env, stdio: ['ignore', 'pipe', 'pipe'] })
      let err = ''
      proc.stderr?.on('data', (d: Buffer) => { err += d.toString() })
      proc.on('exit', code => code === 0 ? resolve() : reject(new Error(err.trim() || `vault init exited with code ${code}`)))
      proc.on('error', reject)
    })

    await spawnVault(bin, ['vault', 'set', 'provider'], password, provider)
    await spawnVault(bin, ['vault', 'set', 'llm_api_key'], password, apiKey)
    await spawnVault(bin, ['vault', 'set', 'gateway-token'], password, token)
  }

  // vaultList returns all secret keys stored in the vault
  async vaultList(password: string): Promise<string[]> {
    const bin = getBinaryPath()
    const output = await spawnVault(bin, ['vault', 'list'], password)
    if (!output || output.includes('No secrets stored')) return []
    return output.split('\n').map(l => l.trim()).filter(l => l.length > 0)
  }

  // vaultGetAll fetches values for every key in a single batch
  async vaultGetAll(password: string, keys: string[]): Promise<Record<string, string>> {
    const bin = getBinaryPath()
    const results: Record<string, string> = {}
    await Promise.all(keys.map(async key => {
      results[key] = await spawnVault(bin, ['vault', 'get', key], password)
    }))
    return results
  }

  // vaultSet stores or updates a secret
  async vaultSet(password: string, key: string, value: string): Promise<void> {
    const bin = getBinaryPath()
    await spawnVault(bin, ['vault', 'set', key], password, value)
  }

  // vaultDelete removes a secret from the vault
  async vaultDelete(password: string, key: string): Promise<void> {
    const bin = getBinaryPath()
    await spawnVault(bin, ['vault', 'delete', key], password)
  }

  // connectDev connects to an already-running server via env vars (dev only)
  connectDev(): SiloConnection {
    const port = parseInt(process.env.SILO_DEV_PORT ?? '', 10)
    const token = process.env.SILO_DEV_TOKEN ?? ''
    if (!port) throw new Error('SILO_DEV_PORT not set')
    if (!token) throw new Error('SILO_DEV_TOKEN not set')
    return { port, token }
  }

  kill(): void {
    this.proc?.kill('SIGTERM')
    this.proc = null
  }
}
