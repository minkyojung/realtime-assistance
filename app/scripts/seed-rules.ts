/**
 * 시드 답변 규칙 — 데모에서 세 가지 판정이 모두 나오게 한다.
 *
 * 실제 운영에서는 이 행들이 knowledge_gap 해소(2막)를 통해 생기지만,
 * 첫 데모에는 사전 승인된 규칙이 몇 개 있어야 3막 루프를 보여줄 수 있다.
 */
import 'dotenv/config'
import { pool, query, queryOne } from '@/lib/db'
import { embed, toVector } from '@/lib/embed'
import type { Intent } from '@/lib/intent'

type Seed = {
  pattern: string
  intent: Intent
  mode: 'direct' | 'conditional' | 'escalate'
  audience: 'public' | 'nda' | 'internal'
  headline: string
  script: string
  condition?: string
  fallback?: string
  authority?: string
}

const SEEDS: Seed[] = [
  {
    pattern: '셀프호스팅 환경에서 SAML SSO 설정 가능 여부',
    intent: 'how',
    mode: 'direct',
    audience: 'public',
    headline: '셀프호스팅에서 SAML SSO 지원',
    script:
      'SAML 2.0 기반 SSO를 지원하며, 셀프호스팅 환경에서도 설정하실 수 있습니다. ' +
      '공식 설정 가이드가 제공되고 있어 IdP 연동은 어렵지 않습니다.',
    authority: '제품팀 리드',
  },
  {
    pattern: 'SSO 로그인 지원 여부',
    intent: 'exists',
    mode: 'direct',
    audience: 'public',
    headline: 'SSO 로그인 지원',
    script:
      'SSO 로그인을 지원합니다. SAML 2.0 기반이며 IdP-initiated 로그인도 가능합니다.',
    authority: '제품팀 리드',
  },
  {
    // 조건부 — NDA 미체결이면 상세 구성을 말할 수 없다
    pattern: '온프레미스 배포 가능 여부',
    intent: 'exists',
    mode: 'conditional',
    audience: 'public',
    headline: 'Enterprise 플랜에서 온프레미스 지원',
    script:
      'Enterprise 플랜에서 온프레미스 배포를 지원합니다. ' +
      '구체적인 구성과 요구 사양은 NDA 체결 후 상세히 안내드릴 수 있습니다.',
    condition: 'NDA 체결 시에만 노드 수·아키텍처 상세 언급 가능',
    fallback:
      'Enterprise 플랜에서 온프레미스를 지원합니다. 상세 구성은 NDA 체결 후 안내드리겠습니다.',
    authority: '영업팀장',
  },
  {
    pattern: 'SOC 2 인증 보유 여부',
    intent: 'exists',
    mode: 'direct',
    audience: 'public',
    headline: 'SOC 2 및 ISO 27001 인증 보유',
    script:
      'SOC 2 인증을 보유하고 있으며 ISO 27001 인증도 취득했습니다. ' +
      '보고서는 요청해 주시면 절차에 따라 공유해 드릴 수 있습니다.',
    authority: '보안팀',
  },
]

async function main() {
  await query(`delete from response_rule_chunk`)
  await query(`update knowledge_gap set resolved_by=null`)
  await query(`delete from response_rule`)

  const vectors = await embed(SEEDS.map((s) => s.pattern))

  for (const [i, s] of SEEDS.entries()) {
    const rule = await queryOne<{ id: string }>(
      `insert into response_rule
         (domain, question_pattern, pattern_embedding, intent, response_mode,
          audience_level, headline_template, suggested_script, condition,
          fallback_script, authority_role, valid_from, valid_until)
       values ('sales', $1, $2::vector, $3::question_intent, $4::response_mode,
               $5::audience_level, $6, $7, $8, $9, $10,
               current_date - 30, current_date + 90)
       returning id`,
      [s.pattern, toVector(vectors[i]), s.intent, s.mode, s.audience,
       s.headline, s.script, s.condition ?? null, s.fallback ?? null, s.authority ?? null],
    )

    // 규칙의 근거로 관련 청크를 연결한다 (M:N)
    await query(
      `insert into response_rule_chunk (rule_id, chunk_id, note)
       select $1, id, '시드 규칙 자동 연결'
         from knowledge_chunk
        where audience_level='public'
        order by embedding::halfvec(3072) <=> $2::halfvec(3072)
        limit 2`,
      [rule!.id, toVector(vectors[i])],
    )
  }

  const rows = await query<{ response_mode: string; intent: string; question_pattern: string }>(
    `select response_mode, intent, question_pattern from response_rule order by response_mode`,
  )
  console.log(`답변 규칙 ${rows.length}건 생성\n`)
  for (const r of rows) {
    const icon = { direct: '🟢', conditional: '🟡', escalate: '🔴' }[r.response_mode as never]
    console.log(`  ${icon} ${r.response_mode.padEnd(12)} ${r.intent.padEnd(7)} ${r.question_pattern}`)
  }
  const [{ n }] = await query<{ n: string }>(`select count(*)::text n from response_rule_chunk`)
  console.log(`\n규칙-근거 연결 ${n}건 (M:N)`)
  await pool.end()
}
main().catch((e) => { console.error(e); process.exit(1) })
