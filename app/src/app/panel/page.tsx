import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { SessionHeader } from './session-header'

export default function PanelPage() {
  return (
    <main data-vibrancy className="flex min-h-screen flex-col">
      {/* TODO: GET /sessions/{id} 연결 전까지는 고정값 */}
      <SessionHeader counterpartOrg="Acme Corp" stage="discovery" isCapturing />

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
