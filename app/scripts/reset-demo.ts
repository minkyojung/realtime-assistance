/**
 * 데모 상태 초기화.
 * 대화 이력과 학습으로 생긴 규칙만 지운다.
 * 지식 청크(인입 결과)와 시드 규칙은 보존한다.
 *
 * response_rule.created_from <-> knowledge_gap.resolved_by 가 순환 참조라
 * 단순 삭제 순서로는 풀 수 없다. 두 제약 모두 DEFERRABLE 로 생성돼 있으므로
 * 트랜잭션 안에서 검사를 커밋 시점까지 미룬다.
 */
import 'dotenv/config'
import { pool, query, transaction } from '@/lib/db'

async function main() {
  const learned = await query<{ id: string }>(
    `select id from response_rule where created_from is not null`,
  )

  await transaction(async (q) => {
    await q(`SET CONSTRAINTS ALL DEFERRED`)
    await q(`delete from knowledge_gap`)
    await q(`delete from question`)
    await q(`delete from utterance`)
    await q(`delete from participant`)
    await q(`delete from session`)
    if (learned.length) {
      const ids = learned.map((r) => r.id)
      await q(`delete from response_rule_chunk where rule_id = any($1::uuid[])`, [ids])
      await q(`delete from response_rule where id = any($1::uuid[])`, [ids])
    }
  })

  const [c] = await query<{ chunks: string; rules: string }>(
    `select (select count(*) from knowledge_chunk)::text chunks,
            (select count(*) from response_rule)::text rules`,
  )
  console.log(`초기화 완료 — 학습 규칙 ${learned.length}건 삭제, 지식 청크 ${c.chunks}건 / 시드 규칙 ${c.rules}건 보존`)
  await pool.end()
}
main().catch((e) => { console.error(e); process.exit(1) })
