import { Badge } from '@/components/ui/badge'
import type { Evidence } from './types'

/**
 * 출처 한 줄.
 *
 * 미팅 중에 필요한 것은 근거를 '읽는' 것이 아니라 '확인'하는 것이다.
 * 펼침(Collapsible)은 스크롤을 밀어 대화 흐름을 놓치게 하므로 쓰지 않는다.
 * 출처 종류와 시점을 항상 노출하고, 더 보고 싶으면 원문으로 이동한다.
 */
const LABEL: Record<string, string> = {
  release: '릴리즈',
  changelog: '변경이력',
  merged_pr: 'PR',
  open_issue: '이슈',
  milestone: '마일스톤',
}

export function SourceBadge({ evidence }: { evidence: Evidence[] }) {
  const top = evidence[0]
  if (!top) return null
  const rest = evidence.length - 1
  return (
    <a
      href={top.source_url}
      target="_blank"
      rel="noreferrer"
      className="no-drag inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-xs text-muted-foreground hover:text-foreground"
    >
      <Badge variant="outline" className="whitespace-nowrap font-normal">
        {LABEL[top.source_type] ?? top.source_type} {top.external_ref}
      </Badge>
      <span>{top.valid_from?.slice(0, 7)}</span>
      {rest > 0 && <span>· 외 {rest}건</span>}
      <span aria-hidden>↗</span>
    </a>
  )
}
