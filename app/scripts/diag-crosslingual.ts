/** 근거 검색 실패가 교차언어 문제인지, 군말 문제인지, 코퍼스 공백인지 가른다. */
import 'dotenv/config'
import { pool, query } from '@/lib/db'
import { embedOne, toVector } from '@/lib/embed'

const PROBES = [
  ['원본(군말 포함)', '그럼 온프레미스 배포도 되나요?'],
  ['군말 제거',       '온프레미스 배포 가능 여부'],
  ['한국어 다른 표현', '셀프호스팅 배포 가능 여부'],
  ['영어 직역',       'on-premise deployment'],
  ['코퍼스 용어',     'self-hosting deployment'],
  ['대조군(잘 되는 것)', '셀프호스팅 SAML SSO 설정'],
]

async function main() {
  for (const [label, q] of PROBES) {
    const vec = toVector(await embedOne(q))
    const rows = await query<{ source_type: string; external_ref: string; heading_path: string; distance: number }>(
      `select source_type, external_ref, coalesce(heading_path,'') as heading_path,
              (embedding::halfvec(3072) <=> $1::halfvec(3072)) as distance
         from knowledge_chunk order by distance limit 3`, [vec],
    )
    console.log(`\n[${label}]  "${q}"`)
    for (const r of rows) {
      console.log(`   ${r.distance.toFixed(3)}  ${r.source_type.padEnd(11)} ${r.external_ref.padEnd(9)} ${r.heading_path.slice(0, 62)}`)
    }
  }

  // 코퍼스에 온프레미스 관련 내용이 실제로 있는지 키워드로 확인
  console.log('\n' + '='.repeat(70))
  console.log('코퍼스 키워드 존재 확인 (BM25가 잡을 수 있는 것)')
  for (const kw of ['on-prem', 'self-host', 'self host', 'airgap', 'air-gap', '온프레미스']) {
    const [{ n }] = await query<{ n: string }>(
      `select count(*)::text n from knowledge_chunk where content ilike $1`, [`%${kw}%`],
    )
    console.log(`   ${kw.padEnd(14)} ${n.padStart(4)}건`)
  }
  await pool.end()
}
main().catch(e => { console.error(e); process.exit(1) })
