/**
 * 답변 문구 생성 — headline 우선 스트리밍.
 *
 * 지연 목표상 첫 줄(headline)이 가장 먼저 나와야 한다.
 * 그래서 모델에게 headline 을 첫 줄에 단독으로 출력하도록 지시하고,
 * 스트림을 줄 단위로 갈라 첫 줄이 완성되는 즉시 화면에 보낸다.
 */
import type { ChunkHit } from '@/lib/search'
import type { ResponseMode } from '@/lib/verdict'

const MODEL = 'gpt-5.6-terra'
/** headline은 모델 없이 만들고 script만 생성하므로 추론을 최소화한다. */
const REASONING_EFFORT = 'low'

export type GenInput = {
  question: string
  intent: string
  mode: ResponseMode
  ctx: Record<string, unknown>
  evidence: ChunkHit[]
  rule?: { headline_template: string; suggested_script: string; condition: string | null } | null
}

function prompt(i: GenInput): string {
  const ctxLine = Object.entries(i.ctx).map(([k, v]) => `${k}=${v}`).join(', ')
  const ev = i.evidence
    .map((e, n) => `[${n + 1}] (${e.source_type} ${e.external_ref}) ${e.heading_path ?? ''}\n${e.content.slice(0, 700)}`)
    .join('\n\n')

  if (i.mode === 'escalate') {
    return `당신은 B2B 세일즈 담당자의 실시간 보조 도구다.
고객이 방금 질문했는데 확정된 답변이 없다. 담당자가 지금 입으로 낼 문장을 만들어라.

규칙
- 고객에게 할 말만 출력할 것. 제목·요약·머리말을 붙이지 말 것.
- 2~3문장.
  * 모른다고 말하지 말고, 확인 후 회신하겠다고 말할 것
  * 회신 시점을 구체적으로 제시할 것
  * 다음 미팅 재료가 될 역질문을 하나 덧붙일 것
- 추측하지 말 것. 근거에 없는 사실을 만들지 말 것.

질문: ${i.question}
상황: ${ctxLine}
${ev ? `참고 자료(고객에게 그대로 말하면 안 됨):\n${ev}` : '참고 자료 없음'}`
  }

  return `당신은 B2B 세일즈 담당자의 실시간 보조 도구다.
승인된 답변 규칙이 있다. 담당자가 지금 입으로 낼 문장을 만들어라.

규칙
- 고객에게 할 말만 출력할 것. 제목·요약·머리말을 붙이지 말 것.
- 2~3문장. 승인된 문구를 기반으로 자연스럽게 다듬을 것.
- 승인된 문구에 없는 수치·일정·약속을 추가하지 말 것.
${i.mode === 'conditional' ? '- 조건이 충족되지 않았으므로 조건에서 금지한 내용은 언급하지 말 것.' : ''}

질문: ${i.question}
상황: ${ctxLine}
승인된 요약: ${i.rule?.headline_template ?? ''}
승인된 문구: ${i.rule?.suggested_script ?? ''}
${i.rule?.condition ? `조건: ${i.rule.condition}` : ''}
${ev ? `근거:\n${ev}` : ''}`
}

/** 토큰 스트림을 그대로 흘려보낸다. 호출 측에서 첫 줄을 갈라 쓴다. */
export async function* streamAnswer(input: GenInput): AsyncGenerator<string> {
  const res = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${process.env.OPENAI_API_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: MODEL,
      stream: true,
      reasoning_effort: REASONING_EFFORT,
      messages: [{ role: 'user', content: prompt(input) }],
    }),
  })
  if (!res.ok || !res.body) throw new Error(`생성 실패 ${res.status}: ${await res.text()}`)

  const reader = res.body.getReader()
  const dec = new TextDecoder()
  let buf = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += dec.decode(value, { stream: true })
    const lines = buf.split('\n')
    buf = lines.pop() ?? ''
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6).trim()
      if (data === '[DONE]') return
      try {
        const delta = JSON.parse(data).choices?.[0]?.delta?.content
        if (delta) yield delta as string
      } catch { /* 부분 청크는 건너뛴다 */ }
    }
  }
}

/** 스트림을 headline / script 로 가른다. 첫 빈 줄이 경계다. */
export function splitAnswer(text: string): { headline: string; script: string } {
  const idx = text.indexOf('\n\n')
  if (idx === -1) return { headline: text.trim(), script: '' }
  return {
    headline: text.slice(0, idx).trim(),
    script: text.slice(idx + 2).trim(),
  }
}
