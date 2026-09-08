/**
 * 3막 루프 검증 — 이 제품의 존재 이유.
 *
 *   1막  질문 -> 답 없음 -> 🔴 + 공백 생성
 *   2막  승인자가 공백을 규칙으로 확정
 *   3막  같은 질문 -> 🟢 즉답
 *
 * 그리고 response_rule.created_from <-> knowledge_gap.resolved_by
 * 양방향 연결이 실제로 생성되는지 확인한다.
 */
import './_env'
import { createSession, participantByRole } from '@/lib/session'
import { saveUtterance, processUtterance } from '@/lib/pipeline'
import { answerQuestion } from '@/lib/answer'
import { resolveGap, listGaps } from '@/lib/gaps'
import { pool, query, queryOne } from '@/lib/db'

const ICON = { direct: '🟢', conditional: '🟡', escalate: '🔴' } as const
const Q = 'Edge Functions는 셀프호스팅에서 언제 지원되나요?'
const CTX = { nda_signed: false, stage: 'discovery' }

/** 질문 하나를 던지고 판정을 받는다. */
async function ask(org: string) {
  const { session } = await createSession({ domain: 'sales', counterpartOrg: org, context: CTX })
  const p = await participantByRole(session.id, 'counterpart')
  const utt = await saveUtterance({
    sessionId: session.id, participantId: p!.id, text: Q, hasQuestionMark: true,
  })
  let out: { mode: string; headline: string; ms: number; script: string } | null = null
  let questionId = ''
  for await (const ev of processUtterance(session, utt)) {
    if (ev.type === 'question.verdict') {
      out = { mode: ev.responseMode, headline: ev.headline, ms: ev.latencyMs, script: '' }
      questionId = ev.questionId
    }
  }
  // 문구는 버튼을 눌러야 나온다. 검증 스크립트가 그 버튼을 대신 누른다.
  if (out && questionId) {
    for await (const ev of answerQuestion(questionId)) {
      if (ev.type === 'question.done') out.script = ev.script
    }
  }
  return out!
}

async function main() {
  // ── 1막 ────────────────────────────────────────────────
  console.log('1막 — 미팅 중, 답을 못 한다\n')
  const before = await ask('Acme Corp')
  console.log(`  고객: ${Q}`)
  console.log(`        ${ICON[before.mode as never]} ${before.headline}  (${before.ms}ms)`)
  console.log(`        "${before.script.replace(/\n+/g, ' ').slice(0, 110)}…"`)

  const { items } = await listGaps({ intent: 'when' })
  const gap = items[0]
  console.log(`\n        ↳ 공백 큐에 등록됨 · ${gap.occurrence_count}회 발생 · ${gap.reason}`)

  // ── 2막 ────────────────────────────────────────────────
  console.log('\n\n2막 — 승인자가 답을 채운다\n')
  const evidence = await query<{ id: string }>(
    `select id from knowledge_chunk where external_ref='#10319' limit 1`,
  )
  const resolved = await resolveGap(gap.id, {
    domain: 'sales',
    question_pattern: gap.question_normalized,
    intent: 'when',
    response_mode: 'conditional',
    audience_level: 'public',
    headline_template: '셀프호스팅 Edge Functions 미지원',
    suggested_script:
      '현재 셀프호스팅 환경에서는 Edge Functions를 지원하지 않습니다. ' +
      '클라우드 환경에서는 정상 제공되며, 셀프호스팅 지원은 검토 중이나 일정은 확정되지 않았습니다.',
    condition: '구체적 지원 시점은 대외 공지 전까지 언급 불가',
    fallback_script: '셀프호스팅 지원 여부는 확인 후 별도로 안내드리겠습니다.',
    authority_role: '제품팀 리드',
    chunk_ids: evidence.map((e) => e.id),
  })
  console.log(`  규칙 생성: ${'ruleId' in resolved ? resolved.ruleId.slice(0, 8) : resolved.error}`)
  console.log(`  함께 해소된 공백: ${'resolvedCount' in resolved ? resolved.resolvedCount : 0}건`)

  // 양방향 FK 확인
  const link = await queryOne<{ rule_id: string; gap_id: string; rule_created_from: string; gap_resolved_by: string }>(
    `select r.id as rule_id, g.id as gap_id,
            r.created_from as rule_created_from, g.resolved_by as gap_resolved_by
       from response_rule r
       join knowledge_gap g on g.resolved_by = r.id
      where r.created_from = g.id limit 1`,
  )
  console.log('\n  양방향 연결 확인')
  console.log(`    response_rule.created_from  -> ${link?.rule_created_from?.slice(0, 8)}  (공백)`)
  console.log(`    knowledge_gap.resolved_by   -> ${link?.gap_resolved_by?.slice(0, 8)}  (규칙)`)
  console.log(`    ${link ? '✅ 서로를 가리킨다' : '❌ 연결 없음'}`)

  // ── 3막 ────────────────────────────────────────────────
  console.log('\n\n3막 — 다음 미팅, 같은 질문\n')
  const after = await ask('Globex Inc')
  console.log(`  고객: ${Q}`)
  console.log(`        ${ICON[after.mode as never]} ${after.headline}  (${after.ms}ms)`)
  console.log(`        "${after.script.replace(/\n+/g, ' ').slice(0, 110)}…"`)

  // ── 결과 ───────────────────────────────────────────────
  console.log('\n' + '='.repeat(66))
  console.log(`전환   ${ICON[before.mode as never]} ${before.mode}  ->  ${ICON[after.mode as never]} ${after.mode}`)

  const cov = await queryOne<{ questions: string; gaps: string; resolved: string }>(
    `select (select count(*) from question)::text questions,
            (select count(*) from knowledge_gap where status='open')::text gaps,
            (select count(*) from knowledge_gap where status='resolved')::text resolved`,
  )
  const q = Number(cov!.questions), g = Number(cov!.gaps)
  console.log(`커버리지  질문 ${q}건 · 미해소 공백 ${g}건 · 해소됨 ${cov!.resolved}건`)
  console.log(`          = ${((1 - g / q) * 100).toFixed(0)}%`)
  await pool.end()
}
main().catch(e => { console.error(e); process.exit(1) })
