import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

export default function PanelPage() {
  return (
    <main data-vibrancy className="flex min-h-screen flex-col">
      {/* 창 드래그 영역. macOS 신호등 버튼 자리를 비워 둔다. */}
      <header className="drag-region flex h-11 shrink-0 items-center gap-2 border-b pl-20 pr-4">
        <span className="text-sm font-semibold">Relay</span>
        <Badge variant="outline" className="no-drag">
          실시간 어시스트 패널
        </Badge>
      </header>

      <div className="flex flex-col gap-4 p-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Phase 0 — 뼈대</CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            Next.js · shadcn · Postgres(pgvector) · Electron 셸이 연결되었습니다.
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
