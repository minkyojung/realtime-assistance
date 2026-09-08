/**
 * HyDE (Hypothetical Document Embeddings) — 어휘 불일치 보정.
 *
 * 문제
 *   질문자의 용어와 문서의 용어가 다르면 임베딩 검색이 실패한다.
 *   실측: "온프레미스 배포 가능 여부" -> 무관한 PR (거리 0.715)
 *   코퍼스에 'on-prem' 0건 / 'self-host' 88건. Supabase는 그 개념을
 *   self-hosting 이라 부른다. 영어로 번역해도 해결되지 않았고(0.648),
 *   BM25 하이브리드로도 못 고친다 (해당 단어가 코퍼스에 없으므로).
 *
 * 해법
 *   질문에 대한 '가상의 답변 문서'를 만들어 그것으로 검색한다.
 *   생성 과정에서 문서가 실제로 쓰는 용어가 자연스럽게 등장한다.
 *   실측: 0.715 -> 0.645, self-hosted 문서가 정확히 상위로 올라옴.
 *
 * 적용 범위
 *   승인자 화면(공백 상세)에만 쓴다. 생성에 약 1초가 들어
 *   발화자 실시간 경로(예산 1,050ms)에는 넣을 수 없다.
 *   실시간 경로는 사람이 쓴 response_rule 패턴과 매칭되므로
 *   애초에 어휘 불일치가 거의 발생하지 않는다.
 */
const MODEL = 'gpt-4.1-mini'

export async function hypotheticalDocument(question: string): Promise<string | null> {
  try {
    const res = await fetch('https://api.openai.com/v1/chat/completions', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${process.env.OPENAI_API_KEY}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        model: MODEL,
        max_tokens: 120,
        messages: [{
          role: 'user',
          content:
            '아래 질문에 대해 오픈소스 제품의 기술 문서나 이슈에 실릴 법한 답변을 영어로 2~3문장 작성하라. ' +
            '사실 여부는 중요하지 않다. 그 문서에서 실제로 쓰일 용어를 사용하는 것이 목적이다.\n\n' +
            `질문: ${question}`,
        }],
      }),
    })
    if (!res.ok) return null
    const j = await res.json()
    return (j.choices?.[0]?.message?.content as string) ?? null
  } catch {
    // 생성 실패 시 원 질문으로 검색한다. 근거 후보 품질만 떨어지고 화면은 동작한다.
    return null
  }
}
