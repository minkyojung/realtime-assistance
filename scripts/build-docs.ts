/**
 * spikes/data/*.json (GitHub 원문) → 표준 문서 형태로 변환한다.
 * 인입 파이프라인의 첫 단계이며, 여기서 source_type이 결정된다.
 */
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import type { RawDoc } from '../app/src/lib/chunk'

const DATA = join(process.cwd(), '..', 'spikes', 'data')
const read = (f: string) => JSON.parse(readFileSync(join(DATA, f), 'utf8'))
const day = (s?: string | null) => (s ?? new Date().toISOString()).slice(0, 10)

export function buildDocs(): RawDoc[] {
  const docs: RawDoc[] = []

  for (const r of read('releases.json')) {
    docs.push({
      sourceType: 'release',
      externalRef: r.tag,
      sourceUrl: r.url,
      title: r.name || r.tag,
      body: r.body ?? '',
      validFrom: day(r.published_at),
    })
  }

  for (const p of read('merged_prs.json')) {
    docs.push({
      sourceType: 'merged_pr',
      externalRef: `#${p.number}`,
      sourceUrl: p.url,
      title: p.title,
      body: p.body ?? '',
      validFrom: day(p.merged_at),
      labels: p.labels,
    })
  }

  for (const i of read('open_issues.json')) {
    docs.push({
      sourceType: 'open_issue',
      externalRef: `#${i.number}`,
      sourceUrl: i.url,
      title: i.title,
      body: i.body ?? '',
      validFrom: day(i.created_at),
      labels: i.labels,
    })
  }

  // 세일즈 키워드 검색 결과. 상태에 따라 확정/미확정이 갈린다.
  for (const s of read('sales_issues.json')) {
    docs.push({
      sourceType: s.is_pr
        ? (s.state === 'closed' ? 'merged_pr' : 'open_issue')
        : 'open_issue',
      externalRef: `#${s.number}`,
      sourceUrl: s.url,
      title: s.title,
      body: s.body ?? '',
      validFrom: day(null),
      labels: s.labels,
    })
  }

  // external_ref 중복 제거 (sales_issues가 다른 목록과 겹칠 수 있다)
  const seen = new Set<string>()
  return docs.filter((d) => {
    const k = `${d.sourceType}:${d.externalRef}`
    if (seen.has(k)) return false
    seen.add(k)
    return true
  })
}
