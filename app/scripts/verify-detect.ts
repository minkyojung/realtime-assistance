/** 질문 감지 + 의도 추출 검증. API 호출 없음. */
import { detectQuestion } from '@/lib/detect'
import { extractIntent, INTENT_POLICY } from '@/lib/intent'

type Case = { text: string; q: boolean; intent?: string; mark?: boolean }

const CASES: Case[] = [
  // 질문 — STT가 물음표를 붙여 준 경우
  { text: '온프레미스 배포 되나요?', q: true, intent: 'exists', mark: true },
  { text: '셀프호스팅에서 SAML SSO 설정 가능한가요?', q: true, intent: 'how', mark: true },
  { text: 'Edge Functions는 셀프호스팅에서 언제 지원되나요?', q: true, intent: 'when', mark: true },
  { text: 'SOC 2 인증도 받으셨나요?', q: true, intent: 'exists', mark: true },
  { text: '엔터프라이즈 플랜은 얼마인가요?', q: true, intent: 'price', mark: true },
  { text: '저번에 말한 그 버그 해결됐나요?', q: true, intent: 'status', mark: true },
  { text: '그거 되나요?', q: true, intent: 'exists', mark: true },

  // 비질문 — 어미 함정
  { text: '그건 확인해봐야 할 것 같은데요', q: false },
  { text: '언제 한번 자리 마련하죠', q: false },
  { text: '네, 보안 요건 말씀이시죠.', q: false },
  { text: '저희는 금융권이라 클라우드가 어려운데요.', q: false },
  { text: '내부 감사 때문에 데이터 반출이 안 됩니다.', q: false },
  { text: '자료는 메일로 보내드릴게요.', q: false },

  // 물음표가 없는 질문 — 보조 신호로 잡아야 함
  { text: '온프레미스도 지원하나요', q: true, intent: 'exists' },
  { text: '가격이 얼마', q: true, intent: 'price' },
]

let ok = 0, miss = 0, wrongIntent = 0
console.log('질문 감지\n')
console.log('  판정  기대  신호           의도      문장')
console.log('  ' + '-'.repeat(76))
for (const c of CASES) {
  const d = detectQuestion(c.text, c.mark)
  const i = d.isQuestion ? extractIntent(c.text) : '-'
  const hit = d.isQuestion === c.q
  const iHit = !c.intent || i === c.intent
  if (hit) ok++; else if (c.q) miss++
  if (hit && !iHit) wrongIntent++
  console.log(
    `  ${hit ? '✓' : '✗'}${d.isQuestion ? 'Q' : ' '}   ${c.q ? 'Q' : ' '}    ${d.signal.padEnd(14)} ${String(i).padEnd(9)} ${c.text}` +
    (c.intent && i !== c.intent ? `   ← 기대 ${c.intent}` : ''),
  )
}
const qs = CASES.filter(c => c.q).length
console.log(`\n  정확도 ${ok}/${CASES.length}   질문 재현율 ${qs - miss}/${qs}   의도 오분류 ${wrongIntent}`)

console.log('\n의도별 기본 정책')
for (const [k, v] of Object.entries(INTENT_POLICY)) {
  console.log(`  ${k.padEnd(7)} 확정답변 ${v.allowDirect ? '허용' : '금지'}   ${v.note}`)
}

console.log('\n핵심 쌍 — 같은 주제, 다른 의도')
for (const t of ['셀프호스팅에서 Edge Functions 쓸 수 있나요?',
                 '셀프호스팅에서 Edge Functions 언제 지원되나요?']) {
  const i = extractIntent(t)
  console.log(`  ${INTENT_POLICY[i].allowDirect ? '🟢' : '🔴'} ${i.padEnd(7)} ${t}`)
}
