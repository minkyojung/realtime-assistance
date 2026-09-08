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
    // macOS 네이티브 vibrancy(NSVisualEffectView). 창 뒤 화면이 시스템 블러로
    // 비치려면 Electron 쪽 배경이 완전 투명이어야 하고, 렌더러(/panel)의
    // html/body 배경도 transparent 여야 한다(globals.css 참고).
    // 애플의 Liquid Glass(NSGlassEffectView)는 Electron이 아직 노출하지 않는다.
    ...(process.platform === 'darwin'
      ? {
          transparent: true,
          backgroundColor: '#00000000',
          vibrancy: 'hud',
          visualEffectState: 'active',
          // NSPanel(nonactivating). 통화 중에 패널을 클릭해도 Zoom·브라우저의
          // 포커스를 뺏지 않는다. 이게 HUD 패널의 핵심이고 재질은 부수적이다.
          type: 'panel',
          alwaysOnTop: true,
        }
      : { backgroundColor: '#ffffff' }),
    webPreferences: { contextIsolation: true, nodeIntegration: false },
  })

  if (process.platform === 'darwin') {
    // 'floating': 일반 창보다는 위, 시스템 UI보다는 아래.
    win.setAlwaysOnTop(true, 'floating')
    // 다른 Space로 넘어가거나 상대가 풀스크린이어도 계속 따라온다.
    win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreenScreen: true })
    // Mission Control 썸네일에는 안 잡히게 한다.
    win.setHiddenInMissionControl(true)
  }

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
