import { NextRequest, NextResponse } from 'next/server'
import { listGaps } from '@/lib/gaps'

export const runtime = 'nodejs'

/** GET /api/deferrals — 화면 2-② 큐 리스트. 기본 정렬은 발생 빈도순. */
export async function GET(req: NextRequest) {
  const p = req.nextUrl.searchParams
  const sort = p.get('sort') as 'frequency' | 'recent' | 'oldest' | null
  if (sort && !['frequency', 'recent', 'oldest'].includes(sort)) {
    return NextResponse.json({ code: 'VALIDATION_FAILED', message: 'sort 값이 올바르지 않습니다' }, { status: 400 })
  }
  const result = await listGaps({
    status: p.get('status') ?? 'open',
    reason: p.get('reason') ?? undefined,
    domain: p.get('domain') ?? undefined,
    intent: p.get('intent') ?? undefined,
    sort: sort ?? 'frequency',
    limit: Number(p.get('limit') ?? 20),
    offset: Number(p.get('offset') ?? 0),
  })
  return NextResponse.json({ ...result, limit: Number(p.get('limit') ?? 20), offset: Number(p.get('offset') ?? 0) })
}
