/** API 명세 Swagger UI 렌더링 캡처. PDF 5장 삽입용. */
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
const OUT = '../wireframes/out'
mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1200, height: 1000 }, deviceScaleFactor: 2 })
await page.goto('http://localhost:3000/spec/swagger.html', { waitUntil: 'networkidle' })
await page.waitForSelector('.opblock-tag', { timeout: 20_000 })
await page.waitForTimeout(1500)

await page.screenshot({ path: `${OUT}/spec-api-overview.png` })
console.log('API 개요')

// 태그 전체 펼침
const count = await page.locator('.opblock-tag').count()
console.log(`태그 ${count}개`)
await page.screenshot({ path: `${OUT}/spec-api-full.png`, fullPage: true })
console.log('API 전체')

await browser.close()
