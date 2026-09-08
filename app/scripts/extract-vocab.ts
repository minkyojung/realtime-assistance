/**
 * 어휘 사전 추출 — STT 프롬프트 주입용.
 *
 * 스모크 테스트에서 이 사전 없이는 한국어 전사가 이렇게 깨졌다.
 *   SOC 2 -> "사기"   SAML -> "생리수혜수"   Edge Functions -> "매직 펑션즈"
 * 고유명사가 깨지면 BM25 검색이 전면 실패하므로, 지식 소스에서
 * 고유명사를 추출해 세션 시작 시 STT에 넘긴다.
 */
import './_env'
import { pool, query } from '@/lib/db'

// 영어 일반어. 고유명사로 오인하기 쉬운 문장 첫 단어들.
const STOP = new Set([
  'The','This','That','These','Those','A','An','And','Or','But','If','When','What',
  'Why','How','We','You','It','In','On','At','To','For','From','With','By','As','Is',
  'Are','Was','Were','Be','Been','Add','Fix','Update','Remove','Use','Make','Set',
  'New','Now','Also','Not','No','Yes','All','Some','Any','Can','Will','Should','Would',
  'Feat','Chore','Docs','Refactor','Test','Bug','Issue','PR','Note','Problem','Solution',
  'Changes','Summary','Description','Steps','Before','After','Please','Thanks','I',
  'Developer','Update','Day','Week','Month','Here','There','Our','Your','Their',
])

const PATTERNS: RegExp[] = [
  /\b[A-Z]{2,}(?:\s?\d(?:\.\d)?)?\b/g,          // SAML, SSO, SOC 2, HIPAA, ISO 27001
  /\b[A-Z][a-z]+(?:[A-Z][a-z]+)+\b/g,           // PascalCase: EdgeFunctions
  /\b[A-Z][a-z]+\s+(?:Functions?|Auth|Storage|Realtime|Vault|Studio|Buckets?)\b/g,
  /\bpg[a-z_]+\b/g,                              // pgvector, pgbouncer
]

async function main() {
  const rows = await query<{ content: string }>(
    `select content from knowledge_chunk`,
  )
  const freq = new Map<string, number>()
  for (const r of rows) {
    for (const re of PATTERNS) {
      for (const m of r.content.match(re) ?? []) {
        const t = m.trim()
        if (t.length < 3 || STOP.has(t)) continue
        freq.set(t, (freq.get(t) ?? 0) + 1)
      }
    }
  }

  // 2회 이상 등장한 것만. 오탈자·1회성 노이즈 제거.
  const terms = [...freq.entries()]
    .filter(([, n]) => n >= 2)
    .sort((a, b) => b[1] - a[1])
    .map(([t]) => t)

  const source = await query<{ id: string; repo: string }>(
    `select id, repo from knowledge_source limit 1`,
  )
  await query(
    `update knowledge_source set vocabulary=$1::jsonb where id=$2`,
    [JSON.stringify(terms), source[0].id],
  )

  console.log(`어휘 ${terms.length}개 추출 → ${source[0].repo}`)
  console.log('\n상위 30개')
  console.log('  ' + terms.slice(0, 30).join(', '))
  const key = ['SAML','SSO','SOC 2','HIPAA','RLS','JWT','GDPR','ISO 27001','PITR']
  console.log('\n핵심 용어 포함 여부')
  for (const k of key) console.log(`  ${terms.includes(k) ? '✅' : '❌'} ${k}`)
  await pool.end()
}
main().catch((e) => { console.error(e); process.exit(1) })
