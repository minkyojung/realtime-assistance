import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

export default function PanelPage() {
  return (
    <main className="flex min-h-screen flex-col gap-4 p-4">
      <header className="flex items-center gap-2">
        <h1 className="text-sm font-semibold">Relay</h1>
        <Badge variant="outline">실시간 어시스트 패널</Badge>
      </header>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Phase 0 — 뼈대</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Next.js · shadcn · Postgres(pgvector) · Electron 셸이 연결되었습니다.
        </CardContent>
      </Card>
    </main>
  )
}
