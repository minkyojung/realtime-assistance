import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { SourceBadge } from './source-badge'
import type { SuggestionItem } from './types'

/**
 * 제안 카드.
 *
 * 점선 테두리로 "내가 실제로 한 말"(실선 버블)과 구분한다.
 * 판정 3색은 왼쪽 테두리로만 표현하며, shadcn 컴포넌트 파일은 수정하지 않는다.
 */
const MODE = {
  direct:      { label: '바로 답변 가능', border: 'border-l-green-600',  dot: 'bg-green-600' },
  conditional: { label: '조건부 답변',    border: 'border-l-amber-500',  dot: 'bg-amber-500' },
  escalate:    { label: '확인 필요',      border: 'border-l-red-600',    dot: 'bg-red-600' },
} as const

export function SuggestionCard({
  item,
  onReject,
}: {
  item: SuggestionItem
  onReject?: (questionId: string) => void
}) {
  // 판정 전 — 질문은 감지됐고 검색 중
  if (!item.mode) {
    return (
      <Card className="ml-auto w-[92%] border-dashed">
        <CardContent className="space-y-2 py-4">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-4/5" />
        </CardContent>
      </Card>
    )
  }

  const m = MODE[item.mode]
  return (
    <Card
      className={cn(
        // 점선 테두리 = 제안. 실선 버블(실제로 한 말)과 구분한다.
        'ml-auto w-[92%] gap-0 border-dashed py-4',
        // 왼쪽 색상 바만 실선으로 두어 판정 색이 또렷하게 보이게 한다.
        'border-l-4 [border-left-style:solid]',
        m.border,
      )}
    >
      <CardHeader className="px-4 pb-2">
        <div className="flex items-center gap-2">
          <span className={cn('size-2 rounded-full', m.dot)} />
          <span className="text-xs font-medium text-muted-foreground">{m.label}</span>
          <Badge variant="outline" className="ml-auto font-normal text-[10px]">
            이렇게 말하세요
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="space-y-2 px-4 pb-2">
        <p className="text-sm font-semibold leading-snug">{item.headline}</p>

        {item.script ? (
          <p className="text-sm leading-relaxed text-foreground/90">
            {item.script}
            {!item.done && <span className="ml-0.5 animate-pulse">▋</span>}
          </p>
        ) : (
          <Skeleton className="h-3 w-full" />
        )}

        {item.condition && (
          <Alert className="py-2">
            <AlertDescription className="text-xs">⚠ {item.condition}</AlertDescription>
          </Alert>
        )}
      </CardContent>

      <CardFooter className="flex-wrap items-center gap-x-2 gap-y-1 px-4 pt-1">
        <SourceBadge evidence={item.evidence} />
        <div className="ml-auto flex items-center gap-1">
          {item.latencyMs !== null && (
            <span className="text-[10px] text-muted-foreground">{item.latencyMs}ms</span>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="no-drag h-6 px-2 text-[11px] text-muted-foreground"
            onClick={() => onReject?.(item.id)}
          >
            답변이 틀렸어요
          </Button>
        </div>
      </CardFooter>
    </Card>
  )
}
