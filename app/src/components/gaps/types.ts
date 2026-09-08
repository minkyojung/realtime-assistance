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

export type Evidence = {
  id: string
  source_type: string
  source_url: string
  external_ref: string
  heading_path: string | null
  content: string
  audience_level: string
  valid_from: string
}

export type GapDetail = GapSummary & {
  raw_utterance: string
  counterpart_org: string
  occurrences: { session_id: string; counterpart_org: string; detected_at: string }[]
  evidence_candidates: Evidence[]
}

export const REASON_LABEL: Record<string, string> = {
  no_rule: '규칙 없음',
  no_knowledge: '자료 없음',
  condition_unmet: '조건 미충족',
  unconfirmed_timeline: '미확정 일정',
  user_rejected: '답변 오류 신고',
}

export const INTENT_LABEL: Record<string, string> = {
  exists: '지원 여부', status: '현재 상태', when: '시점·계획',
  price: '가격', how: '방법', other: '기타',
}

export const SOURCE_LABEL: Record<string, string> = {
  release: '릴리즈', changelog: '변경이력', merged_pr: 'PR',
  open_issue: '이슈', milestone: '마일스톤',
}
