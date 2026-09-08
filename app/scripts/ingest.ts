/**
 * 지식 인입 — GitHub 원문을 검색 가능한 상태로 DB에 적재한다.
 *
 *   원문 → 청킹 → audience_level 강제 부여 → 임베딩 → knowledge_chunk
 *
 * 실행: cd app && npx tsx ../scripts/ingest.ts
 */
import 'dotenv/config'
import { buildDocs } from './build-docs'
import { chunkDoc } from '@/lib/chunk'
import { AUDIENCE_BY_SOURCE } from '@/lib/audience'
import { embed, toVector, EMBED_MODEL } from '@/lib/embed'
import { pool, query, queryOne } from '@/lib/db'

const REPO = process.env.GITHUB_REPO ?? 'supabase/supabase'
const BATCH = 128

async function main() {
  const t0 = Date.now()

  // 1. 지식 소스 등록 (이미 있으면 재사용)
  let source = await queryOne<{ id: string }>(
    `select id from knowledge_source where provider='github' and repo=$1`, [REPO],
  )
  if (!source) {
    source = await queryOne<{ id: string }>(
      `insert into knowledge_source (provider, repo, job_status)
       values ('github', $1, 'running') returning id`, [REPO],
    )
    console.log(`지식 소스 등록: ${REPO}`)
  } else {
    await query(`update knowledge_source set job_status='running' where id=$1`, [source.id])
    await query(`delete from knowledge_chunk where source_id=$1`, [source.id])
    console.log(`기존 소스 재색인: ${REPO}`)
  }
  const sourceId = source!.id

  // 2. 청킹
  const docs = buildDocs()
  const rows = docs.flatMap((d) =>
    chunkDoc(d).map((c) => ({
      sourceType: d.sourceType,
      externalRef: d.externalRef,
      sourceUrl: d.sourceUrl,
      validFrom: d.validFrom,
      audienceLevel: AUDIENCE_BY_SOURCE[d.sourceType],
      headingPath: c.headingPath.slice(0, 500),
      content: c.content,
    })),
  )
  console.log(`문서 ${docs.length} → 청크 ${rows.length}`)

  // 3. 임베딩 + 적재
  console.log(`임베딩 (${EMBED_MODEL})`)
  let done = 0
  for (let i = 0; i < rows.length; i += BATCH) {
    const batch = rows.slice(i, i + BATCH)
    const vectors = await embed(batch.map((r) => r.content))

    const values: unknown[] = []
    const tuples = batch.map((r, k) => {
      const b = k * 8
      values.push(sourceId, r.sourceType, r.sourceUrl, r.externalRef,
                  r.headingPath, r.content, toVector(vectors[k]), r.audienceLevel)
      return `($${b + 1},$${b + 2},$${b + 3},$${b + 4},$${b + 5},$${b + 6},$${b + 7}::vector,$${b + 8},'${batch[k].validFrom}')`
    })
    await query(
      `insert into knowledge_chunk
        (source_id, source_type, source_url, external_ref, heading_path,
         content, embedding, audience_level, valid_from)
       values ${tuples.join(',')}`,
      values,
    )
    done += batch.length
    process.stdout.write(`\r  ${done}/${rows.length}`)
  }
  console.log()

  await query(
    `update knowledge_source set job_status='idle', last_indexed_at=now() where id=$1`,
    [sourceId],
  )

  // 4. 결과
  const stat = await query<{ audience_level: string; source_type: string; n: string }>(
    `select audience_level, source_type, count(*)::text as n
     from knowledge_chunk where source_id=$1
     group by 1,2 order by 1,2`, [sourceId],
  )
  console.log(`\n적재 결과 (${((Date.now() - t0) / 1000).toFixed(1)}초)`)
  for (const s of stat) {
    console.log(`  ${s.audience_level.padEnd(9)} ${s.source_type.padEnd(11)} ${s.n.padStart(5)}`)
  }
  await pool.end()
}

main().catch((e) => { console.error(e); process.exit(1) })
