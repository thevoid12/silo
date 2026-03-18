import { BrowserWindow, shell } from 'electron'
import { join } from 'path'

const CORS_HEADERS = {
  'Access-Control-Allow-Origin': ['*'],
  'Access-Control-Allow-Headers': ['Authorization', 'Content-Type'],
  'Access-Control-Allow-Methods': ['GET', 'POST', 'OPTIONS'],
  'Access-Control-Max-Age': ['86400'],
}

// createMainWindow creates the primary application window
export function createMainWindow(): BrowserWindow {
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    titleBarStyle: 'hiddenInset',
    backgroundColor: '#f6f9fc',
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  })

  // Inject CORS headers for all responses from the local silo server.
  // OPTIONS preflight gets short-circuited with 200 so the browser passes the check.
  win.webContents.session.webRequest.onHeadersReceived(
    { urls: ['http://127.0.0.1:*/*'] },
    (details, callback) => {
      if (details.method === 'OPTIONS') {
        callback({ responseHeaders: CORS_HEADERS, statusLine: 'HTTP/1.1 200 OK' })
      } else {
        callback({ responseHeaders: { ...details.responseHeaders, ...CORS_HEADERS } })
      }
    }
  )

  win.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })

  if (process.env.ELECTRON_RENDERER_URL) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  } else {
    win.loadFile(join(__dirname, '../renderer/index.html'))
  }

  return win
}
