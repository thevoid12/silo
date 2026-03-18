import { spawn, ChildProcess } from 'child_process'
import { createServer, AddressInfo } from 'net'
import { join } from 'path'
import { app } from 'electron'
import type { SiloConnection } from '@shared/types'

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

// getBinaryPath resolves the silo binary for packaged or dev mode
function getBinaryPath(): string {
  if (!app.isPackaged) return process.env.SILO_BIN ?? 'silo'
  const ext = process.platform === 'win32' ? '.exe' : ''
  return join(process.resourcesPath, 'sidecar', `silo${ext}`)
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

      this.proc!.stdout?.on('data', (chunk: Buffer) => {
        const line = chunk.toString().trim()
        if (line.startsWith('token:')) {
          token = line.slice('token:'.length)
          clearTimeout(timeout)
          resolve()
        }
      })

      this.proc!.on('error', err => { clearTimeout(timeout); reject(err) })
      this.proc!.on('exit', code => {
        clearTimeout(timeout)
        if (!token) reject(new Error(`silo exited with code ${code}`))
      })
    })

    await pollHealth(port, token)
    return { port, token }
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
