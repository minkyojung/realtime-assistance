import { cn } from '@/lib/utils'
import type { UtteranceItem } from './types'

/**
 * 발화 버블 — 실제로 오간 말.
 * 제안 카드(점선)와 달리 실선/무테두리로 두어 사실과 제안을 구분한다.
 */
export function UtteranceBubble({ item }: { item: UtteranceItem }) {
  const isHost = item.role === 'host'
  return (
    <div className={cn('flex flex-col gap-0.5', isHost ? 'items-end' : 'items-start')}>
      <span className="px-1 text-[10px] text-muted-foreground">{isHost ? '나' : '고객'}</span>
      <div
        className={cn(
          'max-w-[85%] rounded-lg px-3 py-2 text-sm leading-relaxed',
          isHost ? 'bg-muted text-foreground' : 'border bg-background',
          !item.isFinal && 'text-muted-foreground',
        )}
      >
        {item.text}
        {!item.isFinal && <span className="ml-0.5 animate-pulse">▋</span>}
      </div>
    </div>
  )
}
