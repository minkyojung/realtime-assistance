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
function conditionMet(condition: string | null, ctx: Record<string, unknown>): boolean {
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

  // 1. 의도 자체가 확정 답변 금지면 규칙 검색조차 하지 않는다.
  //    "언제 되나요"에 확정 답변을 매칭하지 않는 것이 이 시스템의 핵심 안전장치다.
  if (!policy.allowDirect) {
    const evidence = await searchChunks(normalized, ctx, 3, vector)
    return {
      responseMode: 'escalate',
      condition: null,
      matchedRuleId: null,
      evidence,
      gapReason: intent === 'when' ? 'unconfirmed_timeline' : 'no_rule',
      rule: null,
    }
  }

  // 2. 규칙 검색 + 근거 검색을 병렬로. 1.5초 예산에서 순차는 불가능하다.
  const [rules, evidence] = await Promise.all([
    searchRules(normalized, intent, ctx, 3, vector),
    searchChunks(normalized, ctx, 3, vector),
  ])

  const rule = rules[0] && rules[0].distance <= RULE_MAX_DISTANCE ? rules[0] : null

  // 3. 규칙 없음 -> 근거만으로 초안. 공백 기록.
  if (!rule) {
    return {
      responseMode: 'escalate',
      condition: null,
      matchedRuleId: null,
      evidence,
      gapReason: evidence.length === 0 ? 'no_knowledge' : 'no_rule',
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
