/** 패널 화면을 실제로 구동해 캡처한다. PDF 삽입용. */
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'

const OUT = '../wireframes/out'
mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 440, height: 900 }, deviceScaleFactor: 2 })
await page.goto('http://localhost:3000/panel-full', { waitUntil: 'networkidle' })

await page.screenshot({ path: `${OUT}/mvp-01-idle.png` })
console.log('01 대기 상태')

await page.getByRole('button', { name: /미팅 시작/ }).click()

// 첫 판정(초록)이 뜨는 지점
await page.getByText('바로 답변 가능').first().waitFor({ timeout: 30_000 })
await page.waitForTimeout(2_500)
await page.screenshot({ path: `${OUT}/mvp-02-direct.png` })
console.log('02 즉답 판정')

// 조건부(노랑)
await page.getByText('조건부 답변').first().waitFor({ timeout: 30_000 })
await page.waitForTimeout(2_500)
await page.screenshot({ path: `${OUT}/mvp-03-conditional.png` })
console.log('03 조건부 판정')

// 확인 필요(빨강) — intent=when 차단
await page.getByText('확인 필요').first().waitFor({ timeout: 30_000 })
await page.waitForTimeout(3_000)
await page.screenshot({ path: `${OUT}/mvp-04-escalate.png` })
console.log('04 확인 필요')

// 전체 흐름
await page.waitForTimeout(6_000)
await page.screenshot({ path: `${OUT}/mvp-05-full.png`, fullPage: true })
console.log('05 전체 대화')

await browser.close()
