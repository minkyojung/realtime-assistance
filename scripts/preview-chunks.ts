/** 임베딩 전에 청킹 결과만 확인한다. API 호출 없음. */
import { buildDocs } from './build-docs'
import { chunkDoc } from '../app/src/lib/chunk'
import { AUDIENCE_BY_SOURCE } from '../app/src/lib/audience'

const docs = buildDocs()
const rows = docs.flatMap((d) =>
  chunkDoc(d).map((c) => ({ doc: d, chunk: c })),
)

const byType = new Map<string, { docs: number; chunks: number }>()
for (const d of docs) {
  const e = byType.get(d.sourceType) ?? { docs: 0, chunks: 0 }
  e.docs++; byType.set(d.sourceType, e)
}
for (const { doc } of rows) byType.get(doc.sourceType)!.chunks++

console.log('종류별 문서/청크 수')
console.log('  타입          문서   청크   공개등급')
for (const [t, v] of [...byType].sort()) {
  console.log(`  ${t.padEnd(12)}${String(v.docs).padStart(5)}${String(v.chunks).padStart(7)}   ${AUDIENCE_BY_SOURCE[t as never]}`)
}
const chars = rows.reduce((a, r) => a + r.chunk.content.length, 0)
console.log(`\n총 문서 ${docs.length} / 총 청크 ${rows.length}`)
console.log(`평균 청크 길이 ${Math.round(chars / rows.length)}자`)
console.log(`예상 임베딩 토큰 약 ${Math.round(chars / 4 / 1000)}K → 비용 약 $${(chars / 4 / 1e6 * 0.13).toFixed(3)}`)

console.log('\n샘플 3건')
for (const { doc, chunk } of [rows[0], rows[Math.floor(rows.length / 2)], rows[rows.length - 1]]) {
  console.log(`\n  [${doc.sourceType} ${doc.externalRef}] ${AUDIENCE_BY_SOURCE[doc.sourceType]}`)
  console.log(`  path: ${chunk.headingPath.slice(0, 90)}`)
  console.log(`  ${chunk.content.replace(/\n/g, ' ').slice(0, 150)}...`)
}
