/**
 * 지식 공백 — 답하지 못한 질문.
 *
 * 목록은 발생 빈도순으로 정렬한다. 전통적 사내 위키가 "무엇을 써야 할지
 * 모른다"는 이유로 비어 있는 반면, Relay는 대화가 우선순위를 정해 준다.
 * "이 12개에 답해 주세요. 각각 3회, 5회 물어봤습니다."
 */
import { query, queryOne } from '@/lib/db'
import { type ChunkHit } from '@/lib/search'
import { hypotheticalDocument } from '@/lib/hyde'
import { embedOne, toVector } from '@/lib/embed'

export type GapSummary = {
  id: string
  question_normalized: string
  intent: string
  reason: string
  status: string
  domain: string
  created_at: string
  occurrence_count: number
  first_seen_at: string
}

export type GapDetail = GapSummary & {
  raw_utterance: string
  counterpart_org: string
  occurrences: { session_id: string; counterpart_org: string; detected_at: string }[]
  evidence_candidates: ChunkHit[]
}

export async function listGaps(opts: {
  status?: string
  reason?: string
  domain?: string
  intent?: string
  sort?: 'frequency' | 'recent' | 'oldest'
  limit?: number
  offset?: number
} = {}) {
  const { status = 'open', sort = 'frequency', limit = 20, offset = 0 } = opts
  const order =
    sort === 'recent' ? 'g.created_at desc'
    : sort === 'oldest' ? 'g.created_at asc'
    : 'occurrence_count desc, g.created_at asc'

  const rows = await query<GapSummary>(
    `select distinct on (g.question_normalized, g.domain)
            g.id, g.question_normalized, g.intent::text, g.reason::text,
            g.status::text, g.domain::text, g.created_at,
            (select count(*)::int from knowledge_gap x
              where x.question_normalized = g.question_normalized
                and x.domain = g.domain) as occurrence_count,
            (select min(x.created_at) from knowledge_gap x
              where x.question_normalized = g.question_normalized
                and x.domain = g.domain) as first_seen_at
       from knowledge_gap g
      where ($1::gap_status is null or g.status = $1::gap_status)
        and ($2::gap_reason is null or g.reason = $2::gap_reason)
        and ($3::domain_type is null or g.domain = $3::domain_type)
        and ($4::question_intent is null or g.intent = $4::question_intent)
      order by g.question_normalized, g.domain, g.created_at asc`,
    [status || null, opts.reason || null, opts.domain || null, opts.intent || null],
  )

  // distinct on 은 정렬 컬럼이 고정되므로 애플리케이션에서 최종 정렬한다.
  const sorted = rows.sort((a, b) =>
    sort === 'recent' ? +new Date(b.created_at) - +new Date(a.created_at)
    : sort === 'oldest' ? +new Date(a.created_at) - +new Date(b.created_at)
    : b.occurrence_count - a.occurrence_count,
  )
  return { total: sorted.length, items: sorted.slice(offset, offset + limit) }
}

export async function getGap(id: string): Promise<GapDetail | null> {
  const gap = await queryOne<GapSummary & { session_id: string }>(
    `select g.id, g.session_id, g.question_normalized, g.intent::text, g.reason::text,
            g.status::text, g.domain::text, g.created_at,
            (select count(*)::int from knowledge_gap x
              where x.question_normalized = g.question_normalized
                and x.domain = g.domain) as occurrence_count,
            (select min(x.created_at) from knowledge_gap x
              where x.question_normalized = g.question_normalized
                and x.domain = g.domain) as first_seen_at
       from knowledge_gap g where g.id = $1`,
    [id],
  )
  if (!gap) return null

  const [utt] = await query<{ text: string; counterpart_org: string }>(
    `select u.text, s.counterpart_org
       from knowledge_gap g
       join question q on q.id = g.question_id
       join utterance u on u.id = q.utterance_id
       join session s on s.id = g.session_id
      where g.id = $1`,
    [id],
  )

  const occurrences = await query<{ session_id: string; counterpart_org: string; detected_at: string }>(
    `select g.session_id, s.counterpart_org, q.detected_at
       from knowledge_gap g
       join question q on q.id = g.question_id
       join session s on s.id = g.session_id
      where g.question_normalized = $1 and g.domain = $2::domain_type
      order by q.detected_at desc`,
    [gap.question_normalized, gap.domain],
  )

  // 승인자가 직접 검색하지 않아도 되도록 근거 후보를 미리 찾아 둔다.
  // 이때는 내부 자료까지 포함해서 본다 (승인자는 열람 권한이 있다).
  const evidence_candidates = await searchGapEvidence(gap.question_normalized, gap.intent)

  return { ...gap, raw_utterance: utt?.text ?? '', counterpart_org: utt?.counterpart_org ?? '', occurrences, evidence_candidates }
}

/**
 * 공백 상세의 근거 후보.
 *
 * 발화자 화면과 두 가지가 다르다.
 *  1. audience_level 을 제한하지 않는다. intent=when 이면 미확정 자료
 *     (열린 이슈·마일스톤)가 오히려 핵심 근거이고, 승인자는 열람 권한이 있다.
 *  2. HyDE 로 어휘 불일치를 보정한다. 실시간이 아니므로 생성 1초를 감당할 수 있다.
 */
async function searchGapEvidence(question: string, intent: string): Promise<ChunkHit[]> {
  const hypothetical = await hypotheticalDocument(question)
  const vec = toVector(await embedOne(hypothetical ?? question))
  const preferInternal = intent === 'when'
  return query<ChunkHit>(
    `select id, source_type, source_url, external_ref, heading_path,
            content, audience_level, valid_from,
            (embedding::halfvec(3072) <=> $1::halfvec(3072)) as distance
       from knowledge_chunk
      order by ${preferInternal ? `(audience_level = 'internal') desc,` : ''} distance
      limit 5`,
    [vec],
  )
}

export type ResolveInput = {
  domain: string
  question_pattern: string
  intent: string
  response_mode: string
  audience_level: string
  headline_template: string
  suggested_script: string
  condition?: string | null
  fallback_script?: string | null
  authority_role?: string | null
  chunk_ids?: string[]
  valid_from?: string
  valid_until?: string | null
}

/**
 * 공백을 답변 규칙으로 해소한다. 이 제품의 핵심 트랜잭션.
 *
 * 한 번의 호출로 아래가 원자적으로 수행된다.
 *   1. response_rule 생성
 *   2. response_rule.created_from = 공백 id
 *   3. knowledge_gap.resolved_by  = 규칙 id
 *   4. knowledge_gap.status = resolved
 *
 * 3·4의 양방향 연결이 "쓸수록 좋아진다"는 주장의 데이터 증거다.
 */
export async function resolveGap(gapId: string, input: ResolveInput) {
  const gap = await queryOne<{ id: string; status: string; question_normalized: string; domain: string }>(
    `select id, status::text, question_normalized, domain::text from knowledge_gap where id=$1`, [gapId],
  )
  if (!gap) return { error: 'NOT_FOUND' as const }
  if (gap.status !== 'open') return { error: 'CONFLICT' as const }

  const vec = toVector(await embedOne(input.question_pattern))

  const rule = await queryOne<{ id: string }>(
    `insert into response_rule
       (domain, question_pattern, pattern_embedding, intent, response_mode,
        audience_level, headline_template, suggested_script, condition,
        fallback_script, authority_role, created_from, valid_from, valid_until)
     values ($1::domain_type,$2,$3::vector,$4::question_intent,$5::response_mode,
             $6::audience_level,$7,$8,$9,$10,$11,$12,
             coalesce($13::date, current_date), $14::date)
     returning id`,
    [input.domain, input.question_pattern, vec, input.intent, input.response_mode,
     input.audience_level, input.headline_template, input.suggested_script,
     input.condition ?? null, input.fallback_script ?? null, input.authority_role ?? null,
     gapId, input.valid_from ?? null, input.valid_until ?? null],
  )

  if (input.chunk_ids?.length) {
    await query(
      `insert into response_rule_chunk (rule_id, chunk_id, note)
       select $1, unnest($2::uuid[]), '승인자가 선택한 근거'`,
      [rule!.id, input.chunk_ids],
    )
  }

  // 같은 질문의 공백을 모두 함께 해소한다. 5회 물어본 질문이면 5건이 정리된다.
  const resolved = await query<{ id: string }>(
    `update knowledge_gap
        set status='resolved', resolved_by=$1, resolved_at=now()
      where question_normalized=$2 and domain=$3::domain_type and status='open'
      returning id`,
    [rule!.id, gap.question_normalized, gap.domain],
  )

  return { ruleId: rule!.id, resolvedCount: resolved.length }
}

export async function rejectGap(gapId: string, reason: string) {
  return queryOne<{ id: string }>(
    `update knowledge_gap set status='rejected', rejection_reason=$2
      where id=$1 and status='open' returning id`,
    [gapId, reason],
  )
}
