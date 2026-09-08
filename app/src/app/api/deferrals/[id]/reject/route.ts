import { NextRequest, NextResponse } from 'next/server'
import { rejectGap } from '@/lib/gaps'

export const runtime = 'nodejs'

/** POST /api/deferrals/{id}/reject — 화면 2-④ [기각] */
export async function POST(req: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const body = await req.json().catch(() => null)
  if (!body?.rejection_reason) {
    return NextResponse.json(
      { code: 'VALIDATION_FAILED', message: '기각 사유가 필요합니다',
        details: [{ field: 'rejection_reason', message: '필수 항목입니다' }] },
      { status: 400 },
    )
  }
  const done = await rejectGap(id, body.rejection_reason)
  if (!done) {
    return NextResponse.json({ code: 'CONFLICT', message: '이미 처리되었거나 없는 공백입니다' }, { status: 409 })
  }
  return NextResponse.json(done)
}
