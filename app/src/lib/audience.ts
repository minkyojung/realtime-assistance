/**
 * GitHub 소스 종류 → 공개 등급 매핑.
 *
 * 이 매핑이 **인입 파이프라인의 규칙**이라는 점이 핵심이다.
 * LLM 프롬프트로 "이건 말하면 안 돼"라고 지시하는 대신,
 * 애초에 등급을 박아 두고 검색 WHERE 절에서 걸러낸다.
 * 모델이 실수해도 상위 등급 정보는 검색 결과에 들어올 수 없다.
 */
export type SourceType =
  | 'release' | 'changelog' | 'merged_pr' | 'open_issue' | 'milestone'

export type AudienceLevel = 'public' | 'nda' | 'internal'

export const AUDIENCE_BY_SOURCE: Record<SourceType, AudienceLevel> = {
  release: 'public',      // 확정 사실
  changelog: 'public',    // 확정 사실
  merged_pr: 'public',    // 확정 사실
  open_issue: 'internal', // 미확정 — 승인 후에만
  milestone: 'internal',  // 미확정 일정 — 가장 위험
}

/** 세션 컨텍스트에서 허용 등급 목록을 만든다. */
export function allowedLevels(ctx: Record<string, unknown>): AudienceLevel[] {
  return ctx.nda_signed === true
    ? ['public', 'nda']
    : ['public']
}
