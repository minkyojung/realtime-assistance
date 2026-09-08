// Relay 데스크톱 셸
//
// 앱 로직은 전부 Next.js에 있고, 이 파일은 /panel 을 사이드 패널 크기의
// 창으로 띄우는 역할만 한다. 오디오 캡처(마이크 + 시스템 오디오 2트랙)를
// 붙일 자리도 여기이며, MVP에서는 ScriptedSource로 대체한다.

const { app, BrowserWindow } = require('electron')

const URL = process.env.RELAY_URL ?? 'http://localhost:3000/panel'

/** dev 서버가 뜰 때까지 기다린다. 없으면 창이 오류 페이지로 뜬다. */
async function waitForServer(url, { timeoutMs = 30_000, intervalMs = 500 } = {}) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url, { method: 'HEAD' })
      if (res.ok || res.status === 404) return true
    } catch {
      // 아직 안 뜸
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return false
}

async function createWindow() {
  const win = new BrowserWindow({
    width: 440,
    height: 900,
    title: 'Relay',
    titleBarStyle: 'hiddenInset',
    backgroundColor: '#ffffff',
    webPreferences: { contextIsolation: true, nodeIntegration: false },
  })

  const ready = await waitForServer(URL)
  if (!ready) console.warn(`[relay] dev 서버 응답 없음: ${URL}`)
  await win.loadURL(URL)
}

app.whenReady().then(createWindow)

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) createWindow()
})
