/** 파이프라인 전체 실행 — 대화 스크립트를 재생하며 판정을 출력한다. */
import 'dotenv/config'
import { createSession, participantByRole } from '@/lib/session'
import { saveUtterance, processUtterance } from '@/lib/pipeline'
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
    for await (const ev of processUtterance(session, utt)) {
      if (ev.type === 'utterance') {
        const who = line.role === 'host' ? '나  ' : '고객'
        console.log(`  ${who} ${line.text}`)
      }
      if (ev.type === 'question.verdict') {
        latencies.push(ev.latencyMs)
        console.log(`       ${ICON[ev.responseMode]} ${ev.headline}   (${ev.latencyMs}ms · 근거 ${ev.evidence.length}건)`)
        if (ev.condition) console.log(`          ⚠ ${ev.condition}`)
        printed = true
      }
      if (ev.type === 'gap.created') console.log(`          ↳ 공백 기록 (${ev.reason})`)
      if (ev.type === 'question.done') script = ev.script
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
