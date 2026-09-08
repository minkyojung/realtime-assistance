import { NextRequest, NextResponse } from 'next/server'
import { createSession } from '@/lib/session'

export const runtime = 'nodejs'

/** POST /api/sessions — 미팅 시작. 화면 1-① */
export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => null)
  if (!body?.domain || !body?.counterpartOrg) {
    return NextResponse.json(
      { code: 'VALIDATION_FAILED', message: '필수 항목이 누락되었습니다',
        details: [{ field: 'counterpartOrg', message: '필수 항목입니다' }] },
      { status: 400 },
    )
  }
  const result = await createSession({
    domain: body.domain,
    counterpartOrg: body.counterpartOrg,
    context: body.context ?? {},
  })
  return NextResponse.json(result, { status: 201 })
}
