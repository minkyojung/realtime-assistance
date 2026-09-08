/**
 * 픽스처(.sse) → TS 리듀서 결과(.feed.json).
 *
 * Swift `FeedStoreTests` 의 기대값을 만든다. 리듀서를 옮긴 뒤
 * 같은 입력에 같은 출력이 나오는지 이 파일로 대조한다.
 *
 *   pnpm tsx scripts/dump-feed.ts ../macos/Fixtures/sales-demo.sse
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { reduceFeed, type StreamEvent } from '@/components/panel/reduce-feed'
import type { FeedItem } from '@/components/panel/types'

const src = process.argv[2]
if (!src) {
  console.error('사용법: tsx scripts/dump-feed.ts <fixture.sse>')
  process.exit(1)
}

const events = readFileSync(src, 'utf8')
  .split('\n')
  .filter((l) => l.startsWith('data: '))
  .map((l) => JSON.parse(l.slice(6)) as StreamEvent)

const feed = events.reduce<FeedItem[]>(reduceFeed, [])

const out = src.replace(/\.sse$/, '.feed.json')
writeFileSync(out, JSON.stringify(feed, null, 2) + '\n')
console.log(`${src}: 이벤트 ${events.length} → 피드 ${feed.length}항목 → ${out}`)
