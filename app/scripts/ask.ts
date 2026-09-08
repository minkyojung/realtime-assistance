/**
 * 질문 하나를 직접 던져 본다 — 대본 없이.
 *
 *   pnpm tsx scripts/ask.ts "셀프호스팅에서 백업은 어떻게 하나요?"
 *   pnpm tsx scripts/ask.ts --nda "온프레미스 노드 구성은 어떻게 되나요?"
 *   pnpm tsx scripts/ask.ts --recruiting "연봉 협상은 언제 하나요?"
 *
 * 실시간 경로(감지 -> 의도 -> 판정)와 답변 경로(버튼)를 순서대로 돌린다.
 * 화면에서 카드를 보고 버튼을 누르는 것과 같은 일이며, 거치는 코드도 같다.
 */
import './_env'
import { pool } from '@/lib/db'
import { createSession, participantByRole, type Domain } from '@/lib/session'
import { saveUtterance, processUtterance } from '@/lib/pipeline'
import { answerQuestion } from '@/lib/answer'

const ICON = { direct: '🟢', conditional: '🟡', escalate: '🔴' } as const

async function main() {
  const args = process.argv.slice(2)
  const domain: Domain = args.includes('--recruiting') ? 'recruiting' : 'sales'
  const ndaSigned = args.includes('--nda')
  const text = args.filter((a) => !a.startsWith('--')).join(' ').trim()

  if (!text) {
    console.error('사용법: pnpm tsx scripts/ask.ts [--nda] [--recruiting] "질문"')
    process.exit(1)
  }

  const context = domain === 'sales'
    ? { nda_signed: ndaSigned, stage: 'discovery' }
    : { interview_round: 2, position_level: 'senior' }

  const { session } = await createSession({
    domain, counterpartOrg: domain === 'sales' ? 'Acme Corp' : '지원자', context,
  })
  const counterpart = await participantByRole(session.id, 'counterpart')

  console.log(`\n고객: ${text}`)
  console.log(`      (${domain} · ${domain === 'sales' ? `NDA ${ndaSigned ? '체결' : '미체결'}` : '2차 면접'})\n`)

  const utterance = await saveUtterance({
    sessionId: session.id, participantId: counterpart!.id, text,
  })

  // ① 실시간 경로 — 화면에 카드가 뜨기까지
  let questionId = ''
  for await (const ev of processUtterance(session, utterance)) {
    if (ev.type === 'question.detected') {
      console.log(`  감지  의도=${ev.intent}`)
      if (ev.normalized !== text) console.log(`        정규화: ${ev.normalized}`)
    }
    if (ev.type === 'question.verdict') {
      questionId = ev.questionId
      console.log(`  판정  ${ICON[ev.responseMode]} ${ev.headline}   (${ev.latencyMs}ms)`)
      if (ev.condition) console.log(`        ⚠ ${ev.condition}`)
    }
    if (ev.type === 'gap.created') console.log(`        ↳ 공백 기록 (${ev.reason})`)
  }
  if (!questionId) {
    console.log('  질문으로 감지되지 않았다. (물음표나 의문형 어미가 없으면 그냥 발화로 본다)')
    return pool.end()
  }

  // ② 답변 경로 — 버튼을 눌렀을 때
  console.log('\n  [Look it up] 누름')
  const t0 = Date.now()
  let firstDelta = 0
  let script = ''
  for await (const ev of answerQuestion(questionId)) {
    if (ev.type === 'question.verdict') {
      console.log(`  근거  ${ev.evidence.length}건` + (ev.evidence.length ? '' : '  (기준 미달 — 붙이지 않는다)'))
      for (const e of ev.evidence) {
        console.log(`        ${e.distance.toFixed(3)}  ${e.external_ref.padEnd(10)} ${(e.heading_path ?? '').slice(0, 62)}`)
        console.log(`               ${e.source_url}`)
      }
    }
    if (ev.type === 'question.delta' && !firstDelta) firstDelta = Date.now() - t0
    if (ev.type === 'question.done') script = ev.script
  }

  console.log(`\n  답    "${script.replace(/\n+/g, ' ')}"`)
  console.log(`        첫 글자 ${firstDelta}ms · 완료 ${Date.now() - t0}ms\n`)
  await pool.end()
}
main().catch((e) => { console.error(e); process.exit(1) })
