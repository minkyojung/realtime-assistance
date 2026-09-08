/**
 * OpenAI 임베딩.
 *
 * 모델은 스파이크 1 실측으로 확정했다.
 *   3-small  top1 정확도 66%
 *   3-large  top1 정확도 86%   ← 채택
 * 근거: docs/06-스파이크1-결과.md
 */
export const EMBED_MODEL = 'text-embedding-3-large'
export const EMBED_DIM = 3072

const ENDPOINT = 'https://api.openai.com/v1/embeddings'

export async function embed(texts: string[]): Promise<number[][]> {
  if (texts.length === 0) return []
  const res = await fetch(ENDPOINT, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${process.env.OPENAI_API_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ model: EMBED_MODEL, input: texts }),
  })
  if (!res.ok) throw new Error(`임베딩 실패 ${res.status}: ${await res.text()}`)
  const json = (await res.json()) as { data: { embedding: number[] }[] }
  return json.data.map((d) => d.embedding)
}

/** 단건 임베딩. */
export async function embedOne(text: string): Promise<number[]> {
  return (await embed([text]))[0]
}

/** pgvector 리터럴 표기로 변환한다. */
export function toVector(v: number[]): string {
  return `[${v.join(',')}]`
}
