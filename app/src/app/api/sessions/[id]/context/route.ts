import { NextRequest, NextResponse } from 'next/server'
import { updateContext } from '@/lib/session'

export const runtime = 'nodejs'

/**
 * PATCH /api/sessions/{id}/context — 화면 1-① [컨텍스트]
 * 갱신 즉시 이후 검색의 audience_level 필터에 반영된다.
 */
export async function PATCH(req: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const body = await req.json().catch(() => null)
  if (!body?.context || typeof body.context !== 'object') {
    return NextResponse.json({ code: 'VALIDATION_FAILED', message: 'context가 필요합니다' }, { status: 400 })
  }
  const updated = await updateContext(id, body.context)
  if (!updated) {
    return NextResponse.json(
      { code: 'CONFLICT', message: '종료된 세션은 수정할 수 없습니다' }, { status: 409 },
    )
  }
  return NextResponse.json(updated)
}
