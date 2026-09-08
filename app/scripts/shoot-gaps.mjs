/** 화면 2 캡처. */
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
const OUT = '../wireframes/out'
mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1280, height: 820 }, deviceScaleFactor: 2 })
await page.goto('http://localhost:3000/gaps', { waitUntil: 'networkidle' })
await page.waitForTimeout(1200)
await page.screenshot({ path: `${OUT}/mvp-06-gaps-list.png` })
console.log('06 공백 큐 목록')

await page.locator('aside button').first().click()
await page.waitForTimeout(2500)
await page.screenshot({ path: `${OUT}/mvp-07-gaps-detail.png` })
console.log('07 공백 상세 + 근거 후보')

await browser.close()
