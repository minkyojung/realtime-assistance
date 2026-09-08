/** 판정 + 생성 실측. 지연(TTFT/headline)이 예산 안에 드는지 본다. */
import 'dotenv/config'
import { detectQuestion } from '@/lib/detect'
import { extractIntent } from '@/lib/intent'
import { decide } from '@/lib/verdict'
import { streamAnswer } from '@/lib/generate'
import { buildHeadline } from '@/lib/headline'
import { pool } from '@/lib/db'

const CTX = { nda_signed: false, stage: 'discovery' }
const QUESTIONS = [
  '셀프호스팅에서 SAML SSO를 설정할 수 있나요?',
  'Edge Functions는 셀프호스팅에서 언제 지원되나요?',
  'SOC 2 인증도 받으셨나요?',
]

async function run(text: string) {
  const t0 = Date.now()
  const det = detectQuestion(text, true)
  const intent = extractIntent(text)
  const d = await decide(text, intent, CTX)
  const tDecide = Date.now() - t0

  // headline 은 모델 없이 즉시 만들어진다. 화면 첫 줄이 뜨는 시점.
  const headline = buildHeadline(d.responseMode, text, d.rule?.headline_template, d.gapReason)
  const tHeadline = Date.now() - t0

  let ttft = 0, acc = ''
  for await (const delta of streamAnswer({
    question: text, intent, mode: d.responseMode, ctx: CTX,
    evidence: d.evidence, rule: d.rule,
  })) {
    if (!ttft) ttft = Date.now() - t0
    acc += delta
  }
  const total = Date.now() - t0
  const script = acc.trim()

  const icon = { direct: '🟢', conditional: '🟡', escalate: '🔴' }[d.responseMode]
  console.log(`\n${icon} ${text}`)
  console.log(`   intent=${intent}  mode=${d.responseMode}  gap=${d.gapReason ?? '-'}  근거 ${d.evidence.length}건`)
  console.log(`   판정 ${tDecide}ms · headline ${tHeadline}ms · 첫토큰 ${ttft}ms · 완료 ${total}ms`)
  console.log(`   ▸ ${headline}`)
  console.log(`     ${script.replace(/\n/g, '\n     ').slice(0, 240)}`)
  if (d.evidence[0]) {
    console.log(`   📄 ${d.evidence[0].source_type} ${d.evidence[0].external_ref} (${d.evidence[0].audience_level})`)
  }
  return { tDecide, ttft, tHeadline, total }
}

async function main() {
  console.log('판정 + 생성 실측 (NDA 미체결)')
  const rs: Awaited<ReturnType<typeof run>>[] = []
  for (const q of QUESTIONS) rs.push(await run(q))

  console.log('\n' + '='.repeat(66))
  console.log('지연 요약 (턴 종료 시점 기준, STT 제외)')
  console.log('  단계          평균     최대     예산')
  const avg = (k: keyof typeof rs[0]) => Math.round(rs.reduce((a, r) => a + r[k], 0) / rs.length)
  const max = (k: keyof typeof rs[0]) => Math.max(...rs.map(r => r[k]))
  console.log(`  판정(검색)  ${String(avg('tDecide')).padStart(6)}ms ${String(max('tDecide')).padStart(7)}ms   200ms`)
  console.log(`  headline    ${String(avg('tHeadline')).padStart(6)}ms ${String(max('tHeadline')).padStart(7)}ms  1050ms  ← 화면 첫 줄`)
  console.log(`  script 첫토큰${String(avg('ttft')).padStart(5)}ms ${String(max('ttft')).padStart(7)}ms     (뒤따라옴)`)
  console.log(`\n  STT 450ms 가산 시 headline 도달: 평균 ${avg('tHeadline') + 450}ms / 최대 ${max('tHeadline') + 450}ms   목표 1500ms`)
  await pool.end()
}
main().catch(e => { console.error(e); process.exit(1) })
