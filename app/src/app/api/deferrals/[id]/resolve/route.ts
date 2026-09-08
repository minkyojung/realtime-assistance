import { NextRequest, NextResponse } from 'next/server'
import { resolveGap } from '@/lib/gaps'

export const runtime = 'nodejs'

const REQUIRED = [
  'domain', 'question_pattern', 'intent', 'response_mode',
  'audience_level', 'headline_template', 'suggested_script',
] as const

/**
 * POST /api/deferrals/{id}/resolve — 화면 2-④ [저장하고 해소]
 * 이 제품의 핵심 트랜잭션. 규칙 생성과 공백 해소가 양방향 FK로 묶인다.
 */
export async function POST(req: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const body = await req.json().catch(() => null)

  const missing = REQUIRED.filter((k) => !body?.[k])
  if (missing.length) {
    return NextResponse.json(
      { code: 'VALIDATION_FAILED', message: '필수 항목이 누락되었습니다',
        details: missing.map((field) => ({ field, message: '필수 항목입니다' })) },
      { status: 400 },
    )
  }
  if (body.response_mode === 'conditional' && !body.condition) {
    return NextResponse.json(
      { code: 'VALIDATION_FAILED', message: '조건부 규칙에는 조건이 필요합니다',
        details: [{ field: 'condition', message: '조건부일 때 필수입니다' }] },
      { status: 400 },
    )
  }

  const result = await resolveGap(id, body)
  if ('error' in result) {
    return result.error === 'NOT_FOUND'
      ? NextResponse.json({ code: 'NOT_FOUND', message: '공백을 찾을 수 없습니다' }, { status: 404 })
      : NextResponse.json({ code: 'CONFLICT', message: '이미 처리된 공백입니다' }, { status: 409 })
  }
  return NextResponse.json(result, { status: 201 })
}
