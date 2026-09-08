/** 패널 화면을 실제로 구동해 캡처한다. PDF 삽입용. */
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'

const OUT = '../wireframes/out'
mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 440, height: 900 }, deviceScaleFactor: 2 })
await page.goto('http://localhost:3000/panel', { waitUntil: 'networkidle' })

await page.screenshot({ path: `${OUT}/mvp-01-idle.png` })
console.log('01 대기 상태')

await page.getByRole('button', { name: /미팅 시작/ }).click()

// 판정이 3건 이상 뜰 때까지 기다린다
await page.waitForTimeout(28_000)
await page.screenshot({ path: `${OUT}/mvp-02-panel.png`, fullPage: true })
console.log('02 대화 진행')

await browser.close()
