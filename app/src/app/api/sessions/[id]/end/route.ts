import { NextResponse } from 'next/server'
import { endSession } from '@/lib/session'

export const runtime = 'nodejs'

/** POST /api/sessions/{id}/end — 미해소 공백이 검토 큐로 확정된다. */
export async function POST(_: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const result = await endSession(id)
  if (!result) {
    return NextResponse.json({ code: 'CONFLICT', message: '이미 종료되었거나 없는 세션입니다' }, { status: 409 })
  }
  return NextResponse.json({ ...result.session, confirmed_gap_count: result.confirmedGapCount })
}
