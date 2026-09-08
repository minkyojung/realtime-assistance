/** HyDE 실측 — 가상 답변으로 검색하면 어휘 불일치가 풀리는가. */
import './_env'
import { pool, query } from '@/lib/db'
import { embedOne, toVector } from '@/lib/embed'

const KEY = process.env.OPENAI_API_KEY!

async function hyde(question: string, model: string) {
  const t0 = Date.now()
  const res = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: { Authorization: `Bearer ${KEY}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model,
      ...(model.startsWith('gpt-5') ? { reasoning_effort: 'low' } : { max_tokens: 120 }),
      messages: [{
        role: 'user',
        content:
          '아래 질문에 대해 오픈소스 제품의 기술 문서에 실릴 법한 답변을 영어로 2~3문장 작성하라. ' +
          '사실 여부는 중요하지 않다. 문서에서 실제로 쓰일 용어를 사용하는 것이 목적이다.\n\n' +
          `질문: ${question}`,
      }],
    }),
  })
  const j = await res.json()
  if (!j.choices) return { text: `[실패] ${JSON.stringify(j).slice(0,120)}`, ms: Date.now() - t0 }
  return { text: j.choices[0].message.content as string, ms: Date.now() - t0 }
}

async function search(text: string) {
  const vec = toVector(await embedOne(text))
  return query<{ source_type: string; external_ref: string; heading_path: string; distance: number }>(
    `select source_type, external_ref, coalesce(heading_path,'') as heading_path,
            (embedding::halfvec(3072) <=> $1::halfvec(3072)) as distance
       from knowledge_chunk order by distance limit 3`, [vec])
}

async function main() {
  const Q = '그럼 온프레미스 배포도 되나요?'
  console.log(`질문: "${Q}"\n`)

  console.log('[기준] 질문 그대로 검색')
  for (const r of await search(Q)) {
    console.log(`   ${r.distance.toFixed(3)}  ${r.source_type.padEnd(11)} ${r.external_ref.padEnd(9)} ${r.heading_path.slice(0, 58)}`)
  }

  for (const model of ['gpt-4.1-nano', 'gpt-4.1-mini']) {
    const h = await hyde(Q, model)
    console.log(`\n[HyDE ${model}]  생성 ${h.ms}ms`)
    console.log(`   가상답변: ${h.text.replace(/\n/g, ' ').slice(0, 170)}…`)
    for (const r of await search(h.text)) {
      console.log(`   ${r.distance.toFixed(3)}  ${r.source_type.padEnd(11)} ${r.external_ref.padEnd(9)} ${r.heading_path.slice(0, 58)}`)
    }
  }
  await pool.end()
}
main().catch(e => { console.error(e); process.exit(1) })
