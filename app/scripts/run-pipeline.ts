/** 파이프라인 전체 실행 — 대화 스크립트를 재생하며 판정을 출력한다. */
import './_env'
import { createSession, participantByRole } from '@/lib/session'
import { saveUtterance, processUtterance } from '@/lib/pipeline'
import { answerQuestion } from '@/lib/answer'
import { DEMO_SCRIPT } from '@/lib/scripted-source'
import { pool, query } from '@/lib/db'

const ICON = { direct: '🟢', conditional: '🟡', escalate: '🔴' } as const

async function main() {
  const { session } = await createSession({
    domain: 'sales',
    counterpartOrg: 'Acme Corp',
    context: { nda_signed: false, stage: 'discovery' },
  })
  const host = await participantByRole(session.id, 'host')
  const other = await participantByRole(session.id, 'counterpart')
  console.log(`세션 ${session.id.slice(0, 8)} · ${session.counterpart_org} · NDA 미체결\n`)

  const latencies: number[] = []
  for (const line of DEMO_SCRIPT) {
    const p = line.role === 'host' ? host! : other!
    const utt = await saveUtterance({
      sessionId: session.id, participantId: p.id,
      text: line.text, hasQuestionMark: line.questionMark,
    })

    let printed = false
    let script = ''
    let questionId = ''
    for await (const ev of processUtterance(session, utt)) {
      if (ev.type === 'utterance') {
        const who = line.role === 'host' ? '나  ' : '고객'
        console.log(`  ${who} ${line.text}`)
      }
      if (ev.type === 'question.verdict') {
        latencies.push(ev.latencyMs)
        questionId = ev.questionId
        console.log(`       ${ICON[ev.responseMode]} ${ev.headline}   (${ev.latencyMs}ms · 근거 ${ev.evidence.length}건)`)
        if (ev.condition) console.log(`          ⚠ ${ev.condition}`)
        printed = true
      }
      if (ev.type === 'gap.created') console.log(`          ↳ 공백 기록 (${ev.reason})`)
    }

    // 문구는 파이프라인이 만들지 않는다 — 화면에서 버튼을 눌러야 도는 경로다.
    // 여기서는 그 버튼을 대신 눌러 전체 흐름을 한 번에 본다.
    if (questionId) {
      for await (const ev of answerQuestion(questionId)) {
        if (ev.type === 'question.done') script = ev.script
      }
    }
    if (printed && script) {
      console.log(`          "${script.replace(/\n+/g, ' ').slice(0, 150)}"`)
    }
    console.log()
  }

  const stats = await query<{ response_mode: string; n: string }>(
    `select response_mode, count(*)::text n from question where session_id=$1 group by 1`,
    [session.id],
  )
  const gaps = await query<{ reason: string; n: string }>(
    `select reason, count(*)::text n from knowledge_gap where session_id=$1 group by 1`,
    [session.id],
  )
  console.log('='.repeat(64))
  console.log('판정 분포   ' + stats.map(s => `${ICON[s.response_mode as never]}${s.n}`).join('  '))
  console.log('공백 생성   ' + (gaps.map(g => `${g.reason}=${g.n}`).join(', ') || '없음'))
  console.log(`판정 지연   평균 ${Math.round(latencies.reduce((a,b)=>a+b,0)/latencies.length)}ms · 최대 ${Math.max(...latencies)}ms   (목표 1050ms)`)
  await pool.end()
}
main().catch(e => { console.error(e); process.exit(1) })
