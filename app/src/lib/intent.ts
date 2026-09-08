/**
 * 질문 의도 추출 — 스파이크 1의 결론.
 *
 * 임베딩 유사도만으로는 "지원되나요"와 "언제 지원되나요"를 구분할 수 없다.
 * 실측에서 이 둘의 유사도가 0.859였고 전체 margin은 -0.53이었다.
 * 그래서 주제(임베딩)와 의도(규칙)를 별도 축으로 분리한다.
 *
 * intent 는 검색 WHERE 절 필터로 사용된다. 프롬프트가 아니다.
 */
export type Intent = 'exists' | 'status' | 'when' | 'price' | 'how' | 'other'

/**
 * 판정 순서가 중요하다. when 을 가장 먼저 본다.
 * "언제 지원되나요"는 exists 패턴에도 걸리지만 when 이어야 한다.
 */
const RULES: { intent: Intent; re: RegExp }[] = [
  // 시점·계획 — 확정 답변 매칭 금지 대상
  { intent: 'when', re: /(언제|예정|계획|로드맵|일정|출시|릴리즈\s*시점|나오나|나올|될\s*예정|지원\s*예정|추가\s*예정|언제쯤)/ },
  // 가격
  { intent: 'price', re: /(얼마|가격|비용|요금|과금|단가|할인|플랜\s*비용|무료|유료)/ },
  // 방법·절차
  { intent: 'how', re: /(어떻게|방법|설정|구성|절차|연동\s*방법|하는\s*법|가이드|문서\s*있)/ },
  // 현재 상태 — 이미 처리됐는지
  { intent: 'status', re: /(해결됐|고쳐졌|수정됐|반영됐|조치됐|패치|지금\s*상태|현재\s*상태)/ },
  // 존재·가능 여부
  { intent: 'exists', re: /(되나요|되는지|가능한가|가능한지|지원하|지원되|있나요|있는지|제공하|되죠|있죠)/ },
  // 존댓말 의문 어미 — "받으셨나요", "하셨나요"
  { intent: 'exists', re: /(으셨|하셨|되셨|받으셨|이신가|인가요)/ },
]

export function extractIntent(text: string): Intent {
  const t = text.trim()
  for (const r of RULES) if (r.re.test(t)) return r.intent
  // 어떤 규칙에도 안 걸렸지만 의문형 어미로 끝나면 '여부 질문'으로 본다.
  // 가장 흔한 유형이고, 안전은 audience_level 필터가 별도로 담당한다.
  if (/(나요|가요|까요)\s*[?？.!]?$/.test(t)) return 'exists'
  return 'other'
}

/**
 * 의도별 기본 정책.
 *
 * allowDirect 의 의미는 "승인된 규칙을 쓸 수 있는가"가 아니라
 * **"규칙이 없을 때 지식 청크만으로 확정처럼 답해도 되는가"** 이다.
 *
 * when 질문에 GitHub 원문(열린 이슈·마일스톤)을 근거로 자동 답변하면
 * 미확정 로드맵이 확정처럼 나간다. 그래서 규칙이 없으면 항상 승인 경로로 보낸다.
 * 반대로 승인자가 확정한 규칙이 있다면 그것은 이미 미확정 정보가 아니므로
 * intent 와 무관하게 사용한다. 그래야 학습 루프가 when 질문에서도 닫힌다.
 */
export const INTENT_POLICY: Record<Intent, { allowDirect: boolean; note: string }> = {
  exists: { allowDirect: true,  note: '머지된 PR·릴리즈는 확정 사실' },
  status: { allowDirect: true,  note: '완료 여부는 확정 가능' },
  when:   { allowDirect: false, note: '일정은 미확정. 항상 승인 경로' },
  price:  { allowDirect: true,  note: '승인된 규칙이 있을 때만' },
  how:    { allowDirect: true,  note: '문서 안내는 유출 위험 낮음' },
  other:  { allowDirect: false, note: '분류 실패 시 보수적으로' },
}
