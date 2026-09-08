/**
 * Phase 1b — 인입 결과 검증.
 *   1. 스파이크 1의 원형 질문 10개로 커버리지를 잰다.
 *   2. audience_level 필터가 실제로 차단하는지 확인한다.
 */
import './_env'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { searchChunks } from '@/lib/search'
import { pool, query } from '@/lib/db'

const NDA_NO = { nda_signed: false, stage: 'discovery' }
const NDA_YES = { nda_signed: true, stage: 'evaluation' }

async function main() {
  const qs = JSON.parse(
    readFileSync(join(process.cwd(), '..', 'spikes', 'questions.json'), 'utf8'),
  ).canonical as { id: string; text: string }[]

  console.log('1) 질문별 커버리지 (NDA 미체결 = public 만 허용)\n')
  console.log('   ID   근거  최근접  질문')
  console.log('   ' + '-'.repeat(74))
  let empty = 0
  for (const q of qs) {
    const hits = await searchChunks(q.text, NDA_NO, 5)
    const top = hits[0]
    if (!top || top.distance > 0.75) empty++
    const mark = !top ? ' ✗' : top.distance > 0.75 ? ' △' : ' ✓'
    console.log(
      `  ${mark}${q.id}  ${String(hits.length).padStart(3)}  ${top ? top.distance.toFixed(3) : '  —  '}  ${q.text}`,
    )
    if (top) console.log(`         └ ${top.source_type} ${top.external_ref}  ${(top.heading_path ?? '').slice(0, 62)}`)
  }
  console.log(`\n   근거 부족(△✗): ${empty}/${qs.length}`)

  console.log('\n2) audience_level 차단 검증')
  const probe = 'Edge Functions 셀프호스팅 지원 시점'
  const pub = await searchChunks(probe, NDA_NO, 20)
  const nda = await searchChunks(probe, NDA_YES, 20)
  const leaked = pub.filter((h) => h.audience_level !== 'public')
  console.log(`   질의: "${probe}"`)
  console.log(`   NDA 미체결 → ${pub.length}건, 등급 분포: ${[...new Set(pub.map(h => h.audience_level))].join(', ')}`)
  console.log(`   NDA 체결   → ${nda.length}건, 등급 분포: ${[...new Set(nda.map(h => h.audience_level))].join(', ')}`)
  console.log(`   ${leaked.length === 0 ? '✅ internal 유출 0건' : `❌ 유출 ${leaked.length}건`}`)

  const total = await query<{ n: string }>(
    `select count(*)::text n from knowledge_chunk where audience_level='internal'`,
  )
  console.log(`   (DB에 internal 청크는 ${total[0].n}건 존재하나 검색되지 않음)`)

  console.log('\n3) 인덱스 사용 확인')
  const plan = await query<{ 'QUERY PLAN': string }>(
    `explain select id from knowledge_chunk
      where audience_level='public'
      order by embedding::halfvec(3072) <=> (select embedding::halfvec(3072) from knowledge_chunk limit 1)
      limit 5`,
  )
  console.log('   ' + plan.map(p => p['QUERY PLAN']).join('\n   '))

  await pool.end()
}
main().catch((e) => { console.error(e); process.exit(1) })
