/**
 * 세션 — 미팅 한 건.
 *
 * context 가 jsonb 인 것이 핵심 설계다. 세일즈는 {nda_signed, stage},
 * 채용은 {interview_round, position_level} 로 필드가 다르며,
 * 컬럼으로 고정하면 도메인 추가 시 마이그레이션이 발생해
 * "스키마 변경 없는 도메인 교체" 검증 조건이 깨진다.
 */
import { query, queryOne } from '@/lib/db'

export type Domain = 'sales' | 'recruiting'
export type ParticipantRole = 'host' | 'counterpart'

export type Session = {
  id: string
  domain: Domain
  counterpart_org: string
  context: Record<string, unknown>
  status: 'active' | 'ended'
  started_at: string
  ended_at: string | null
}

export type Participant = {
  id: string
  session_id: string
  role: ParticipantRole
  display_name: string | null
  speaker_label: string | null
}

export async function createSession(input: {
  domain: Domain
  counterpartOrg: string
  context?: Record<string, unknown>
}): Promise<{ session: Session; participants: Participant[] }> {
  const session = await queryOne<Session>(
    `insert into session (domain, counterpart_org, context)
     values ($1::domain_type, $2, $3::jsonb)
     returning *`,
    [input.domain, input.counterpartOrg, JSON.stringify(input.context ?? {})],
  )

  // MVP는 1:1이므로 host/counterpart 각 1명을 자동 생성한다.
  // 1:N 확장 시에는 counterpart 행만 늘어나고 스키마는 그대로다.
  const participants = await query<Participant>(
    `insert into participant (session_id, role, display_name)
     values ($1, 'host', '나'), ($1, 'counterpart', $2)
     returning *`,
    [session!.id, input.counterpartOrg],
  )
  return { session: session!, participants }
}

export async function getSession(id: string) {
  const session = await queryOne<Session>(`select * from session where id=$1`, [id])
  if (!session) return null
  const participants = await query<Participant>(
    `select * from participant where session_id=$1 order by role desc`, [id],
  )
  return { session, participants }
}

export async function updateContext(id: string, context: Record<string, unknown>) {
  return queryOne<Session>(
    `update session set context=$2::jsonb where id=$1 and status='active' returning *`,
    [id, JSON.stringify(context)],
  )
}

export async function endSession(id: string) {
  const session = await queryOne<Session>(
    `update session set status='ended', ended_at=now()
      where id=$1 and status='active' returning *`, [id],
  )
  if (!session) return null
  // 세션 내 미해소 공백 수 = 검토 큐로 확정되는 건수
  const [{ n }] = await query<{ n: string }>(
    `select count(*)::text n from knowledge_gap where session_id=$1 and status='open'`, [id],
  )
  return { session, confirmedGapCount: Number(n) }
}

/** 참가자 역할로 participant_id 를 찾는다. 스크립트 재생에서 사용. */
export async function participantByRole(sessionId: string, role: ParticipantRole) {
  return queryOne<Participant>(
    `select * from participant where session_id=$1 and role=$2 limit 1`,
    [sessionId, role],
  )
}
