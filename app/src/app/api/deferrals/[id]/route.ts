import { NextResponse } from 'next/server'
import { getGap } from '@/lib/gaps'

export const runtime = 'nodejs'

/** GET /api/deferrals/{id} — 화면 2-③ 상세 + 근거 후보 */
export async function GET(_: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const gap = await getGap(id)
  if (!gap) {
    return NextResponse.json({ code: 'NOT_FOUND', message: '공백을 찾을 수 없습니다' }, { status: 404 })
  }
  return NextResponse.json(gap)
}
