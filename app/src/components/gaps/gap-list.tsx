'use client'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { REASON_LABEL, type GapSummary } from './types'

/**
 * 공백 큐 목록 — 발생 빈도순.
 * 승인자에게 "무엇부터 답해야 하는지"를 시스템이 알려 주는 자리다.
 */
export function GapList({
  items,
  selectedId,
  onSelect,
}: {
  items: GapSummary[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
  if (items.length === 0) {
    return (
      <div className="p-8 text-center text-sm text-muted-foreground">
        검토할 공백이 없습니다.
      </div>
    )
  }
  return (
    <ul className="divide-y">
      {items.map((g) => (
        <li key={g.id}>
          <button
            onClick={() => onSelect(g.id)}
            className={cn(
              'w-full px-4 py-3 text-left transition-colors hover:bg-muted/50',
              selectedId === g.id && 'bg-muted',
            )}
          >
            <div className="flex items-start gap-3">
              <span className="mt-0.5 min-w-[2.5rem] text-center text-sm font-semibold tabular-nums">
                {g.occurrence_count}회
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm">{g.question_normalized}</p>
                <div className="mt-1 flex items-center gap-1.5">
                  <Badge variant="outline" className="font-normal text-[10px]">
                    {REASON_LABEL[g.reason] ?? g.reason}
                  </Badge>
                  <span className="text-[10px] text-muted-foreground">
                    최초 {g.first_seen_at?.slice(0, 10)}
                  </span>
                </div>
              </div>
            </div>
          </button>
        </li>
      ))}
    </ul>
  )
}
