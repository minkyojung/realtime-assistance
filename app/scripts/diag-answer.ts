/**
 * 답변 경로 실측 — 버튼을 눌렀을 때 무슨 일이 일어나는가.
 *
 * 가장 최근 세션의 질문들에 대해 `answerQuestion()` 을 그대로 돌린다.
 *   · 규칙이 있는 질문  승인 문구를 그대로 → 대기 0초
 *   · 규칙이 없는 질문  HyDE 재검색 → 생성 (예산 5초)
 *
 * 먼저 `pnpm tsx scripts/run-pipeline.ts` 로 세션을 하나 만들어 둬야 한다.
 */
import './_env'
import { pool, query } from '@/lib/db'
import { answerQuestion } from '@/lib/answer'

async function main() {
  const questions = await query<{
    id: string; normalized_text: string; matched_rule_id: string | null
  }>(
    `select id, normalized_text, matched_rule_id from question
      where session_id = (select id from session order by started_at desc limit 1)
      order by detected_at`,
  )
  if (questions.length === 0) {
    console.log('질문이 없다. 먼저 pnpm tsx scripts/run-pipeline.ts 를 돌려라.')
    return pool.end()
  }

  for (const q of questions) {
    const path = q.matched_rule_id ? '규칙 있음 → 승인 문구' : '규칙 없음 → HyDE 재검색'
    console.log(`\n■ ${q.normalized_text}   [${path}]`)

    const t0 = Date.now()
    let firstDelta = 0
    let script = ''
    for await (const ev of answerQuestion(q.id)) {
      if (ev.type === 'question.verdict') {
        console.log(`   근거 ${ev.evidence.length}건: ` +
          ev.evidence.map((e) => `${e.external_ref}(${e.distance.toFixed(3)})`).join(' '))
      }
      if (ev.type === 'question.delta' && !firstDelta) firstDelta = Date.now() - t0
      if (ev.type === 'question.done') script = ev.script
    }

    console.log(`   첫 글자 ${firstDelta}ms · 완료 ${Date.now() - t0}ms   (예산 5,000ms)`)
    console.log(`   "${script.replace(/\n+/g, ' ').slice(0, 100)}…"`)
  }
  await pool.end()
}
main().catch((e) => { console.error(e); process.exit(1) })
