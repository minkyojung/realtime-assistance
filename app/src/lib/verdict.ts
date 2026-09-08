/**
 * 판정 — 실시간 경로의 유일한 무거운 모델 호출.
 *
 * 흐름
 *   1. intent 정책상 확정 답변이 금지면(when/other) 즉시 escalate
 *   2. response_rule 검색 (intent + audience_level 필터)
 *   3. 규칙이 있으면 conditional 여부 판단
 *   4. 없으면 knowledge_chunk 로 초안 생성 + knowledge_gap 기록
 *
 * headline 을 먼저 스트리밍해야 하므로 생성 프롬프트는 한 줄 요약을
 * 가장 먼저 뱉도록 지시한다.
 */
import { query } from '@/lib/db'
import { embedOne, toVector } from '@/lib/embed'
import { allowedLevels } from '@/lib/audience'
import { INTENT_POLICY, type Intent } from '@/lib/intent'
import { searchChunks, type ChunkHit } from '@/lib/search'

export type ResponseMode = 'direct' | 'conditional' | 'escalate'

export type RuleHit = {
  id: string
  question_pattern: string
  intent: Intent
  response_mode: ResponseMode
  audience_level: string
  headline_template: string
  suggested_script: string
  condition: string | null
  fallback_script: string | null
  distance: number
}

export type Verdict = {
  responseMode: ResponseMode
  headline: string
  suggestedScript: string
  condition: string | null
  matchedRuleId: string | null
  evidence: ChunkHit[]
  gapReason: string | null
}

/** 규칙 매칭 임계값. 스파이크1에서 단일 임계값만으로는 부족함이 확인되어
 *  intent 필터와 함께 사용한다. 이 값은 후보를 좁히는 용도이지
 *  단독 판정 근거가 아니다. */
const RULE_MAX_DISTANCE = 0.45

/** 근거 채택 임계값.
 *
 *  `searchChunks` 는 상위 k건을 무조건 돌려준다 — 코퍼스에 답이 없어도
 *  "제일 덜 먼" 청크가 나온다. 그걸 그대로 근거로 붙이면 온프레미스 질문에
 *  MCP 클라이언트 문서가 출처로 달린다(실측 0.747). 근거를 눌러본 사람이
 *  엉뚱한 문서를 보는 순간 답 자체의 신뢰가 깨지므로, 먼 청크는 버리고
 *  근거 없음으로 간다.
 *
 *  값의 근거 — 실측에서 관련 근거는 0.51~0.61, 무관한 근거는 0.75 이상에
 *  몰렸다. `scripts/verify-search.ts` 도 0.75 초과를 "근거 부족"으로 센다.
 *  그 사이에서 관련 쪽에 여유를 두고 잡았다. */
export const EVIDENCE_MAX_DISTANCE = 0.65

export async function searchRules(
  question: string,
  intent: Intent,
  ctx: Record<string, unknown>,
  limit = 3,
  vector?: number[],
): Promise<RuleHit[]> {
  const vec = toVector(vector ?? (await embedOne(question)))
  return query<RuleHit>(
    `select id, question_pattern, intent, response_mode, audience_level,
            headline_template, suggested_script, condition, fallback_script,
            (pattern_embedding::halfvec(3072) <=> $1::halfvec(3072)) as distance
       from response_rule
      where is_active
        and intent = $2::question_intent
        and audience_level = any($3::audience_level[])
        and valid_from <= current_date
        and (valid_until is null or valid_until >= current_date)
      order by distance
      limit $4`,
    [vec, intent, allowedLevels(ctx), limit],
  )
}

/** 조건 문자열이 세션 컨텍스트에서 충족되는지 본다.
 *  MVP에서는 nda_signed 만 다룬다. */
export function conditionMet(condition: string | null, ctx: Record<string, unknown>): boolean {
  if (!condition) return true
  if (/NDA/i.test(condition)) return ctx.nda_signed === true
  return true
}

export async function decide(
  normalized: string,
  intent: Intent,
  ctx: Record<string, unknown>,
): Promise<Omit<Verdict, 'headline' | 'suggestedScript'> & {
  rule: RuleHit | null
}> {
  const policy = INTENT_POLICY[intent]

  // 임베딩은 한 번만 계산해 규칙·근거 검색이 공유한다.
  // 각각 호출하면 실측에서 판정 단계가 700ms를 넘었다.
  const vector = await embedOne(normalized)

  // 1. 규칙 검색 + 근거 검색을 병렬로. 1.5초 예산에서 순차는 불가능하다.
  const [rules, allEvidence] = await Promise.all([
    searchRules(normalized, intent, ctx, 3, vector),
    searchChunks(normalized, ctx, 3, vector),
  ])

  const rule = rules[0] && rules[0].distance <= RULE_MAX_DISTANCE ? rules[0] : null
  // 규칙에 임계값이 있는 것과 같은 이유로 근거에도 하한을 둔다.
  const evidence = allEvidence.filter((e) => e.distance <= EVIDENCE_MAX_DISTANCE)

  // 2. 규칙이 없을 때만 의도 정책이 작동한다.
  //
  //    intent 정책의 의미는 "승인된 규칙을 쓰지 말라"가 아니라
  //    "규칙 없이 지식 청크만으로 확정처럼 답하지 말라"이다.
  //    승인자가 명시적으로 확정한 답변은 미확정 정보가 아니므로 사용한다.
  //    이 구분이 없으면 when 질문은 아무리 답을 채워도 영원히 🔴에 머물러
  //    학습 루프가 닫히지 않는다. when 이야말로 가장 자주 나오는 질문이다.
  if (!rule) {
    return {
      responseMode: 'escalate',
      condition: null,
      matchedRuleId: null,
      evidence,
      gapReason: !policy.allowDirect
        ? (intent === 'when' ? 'unconfirmed_timeline' : 'no_rule')
        : (evidence.length === 0 ? 'no_knowledge' : 'no_rule'),
      rule: null,
    }
  }

  // 4. 조건부 규칙의 조건 충족 여부
  const met = conditionMet(rule.condition, ctx)
  return {
    responseMode: met ? rule.response_mode : 'conditional',
    condition: rule.condition,
    matchedRuleId: rule.id,
    evidence,
    gapReason: met ? null : 'condition_unmet',
    rule,
  }
}
