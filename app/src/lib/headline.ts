/**
 * headline — 화면에 가장 먼저 뜨는 한 줄.
 *
 * 생성 모델로 만들면 첫 줄이 뜨기까지 3초가 걸린다(실측).
 * headline 은 요약일 뿐이므로 모델 없이 만든다.
 *   - 규칙이 매칭되면 승인된 headline_template 을 그대로 쓴다
 *   - 매칭이 없으면 정규화된 질문을 쓴다
 * 결과적으로 화면 첫 줄은 검색이 끝나는 즉시 뜬다.
 *
 * 모델은 실제로 입에 낼 문구(script) 생성에만 쓰고, 그건 뒤따라 스트리밍된다.
 */
import type { ResponseMode } from '@/lib/verdict'

const ESCALATE_PREFIX: Record<string, string> = {
  unconfirmed_timeline: '일정 확인 필요',
  no_knowledge: '자료 없음 — 확인 필요',
  no_rule: '확정 답변 없음',
  condition_unmet: '조건부 — 언급 주의',
  user_rejected: '재확인 필요',
}

export function buildHeadline(
  mode: ResponseMode,
  normalized: string,
  ruleHeadline?: string | null,
  gapReason?: string | null,
): string {
  if (mode !== 'escalate' && ruleHeadline) return ruleHeadline
  const prefix = (gapReason && ESCALATE_PREFIX[gapReason]) ?? '확인 필요'
  return `${prefix} · ${normalized}`
}
