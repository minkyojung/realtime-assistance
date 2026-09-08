import { NextResponse } from 'next/server'
import { getSession } from '@/lib/session'
import { query } from '@/lib/db'

export const runtime = 'nodejs'

/** GET /api/sessions/{id} — 화면 1-① 초기 로드 */
export async function GET(_: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const found = await getSession(id)
  if (!found) {
    return NextResponse.json({ code: 'NOT_FOUND', message: '세션을 찾을 수 없습니다' }, { status: 404 })
  }
  const [counts] = await query<{ questions: string; gaps: string }>(
    `select (select count(*) from question where session_id=$1)::text as questions,
            (select count(*) from knowledge_gap where session_id=$1 and status='open')::text as gaps`,
    [id],
  )
  return NextResponse.json({
    ...found.session,
    participants: found.participants,
    question_count: Number(counts.questions),
    open_gap_count: Number(counts.gaps),
  })
}
