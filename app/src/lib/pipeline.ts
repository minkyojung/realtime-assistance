/**
 * 발화 처리 파이프라인 — 실시간 경로 전체.
 *
 *   발화 저장
 *     -> 상대 발화인가?        (host 발화는 문맥 저장만)
 *     -> 질문인가?             (물음표 주신호 + 어미 보조)
 *     -> 질문 정규화            (직전 발화로 대명사 해소)
 *     -> 의도 추출              (규칙, 0ms)
 *     -> 판정                   (규칙·근거 병렬 검색, 임베딩 1회)
 *     -> headline 즉시 확정      (모델 없음)
 *     -> script 스트리밍         (모델 1회)
 *     -> 매칭 실패 시 공백 기록
 *
 * 무거운 모델 호출은 script 생성 1회뿐이다.
 */
import { query, queryOne } from '@/lib/db'
import { detectQuestion } from '@/lib/detect'
import { extractIntent, type Intent } from '@/lib/intent'
import { decide, type ResponseMode } from '@/lib/verdict'
import { buildHeadline } from '@/lib/headline'
import { streamAnswer } from '@/lib/generate'
import type { ChunkHit } from '@/lib/search'
import type { Session } from '@/lib/session'

export type UtteranceRow = {
  id: string
  session_id: string
  participant_id: string
  role: 'host' | 'counterpart'
  text: string
  has_question_mark: boolean
  started_at: string
  ended_at: string
}

/** 파이프라인이 화면으로 밀어 보내는 이벤트. */
export type PipelineEvent =
  | { type: 'utterance'; utterance: UtteranceRow }
  | { type: 'question.detected'; questionId: string; normalized: string; intent: Intent }
  | { type: 'question.verdict'; questionId: string; responseMode: ResponseMode
      ; headline: string; condition: string | null; evidence: ChunkHit[]; latencyMs: number }
  | { type: 'question.delta'; questionId: string; delta: string }
  | { type: 'question.done'; questionId: string; script: string; totalMs: number }
  | { type: 'gap.created'; questionId: string; gapId: string; reason: string }

export async function saveUtterance(input: {
  sessionId: string
  participantId: string
  text: string
  hasQuestionMark?: boolean
  startedAt?: string
  endedAt?: string
}): Promise<UtteranceRow> {
  const now = new Date().toISOString()
  const row = await queryOne<UtteranceRow>(
    `insert into utterance
       (session_id, participant_id, text, is_final, has_question_mark, started_at, ended_at)
     values ($1,$2,$3,true,$4,$5,$6)
     returning *,
       (select role from participant where id=$2) as role`,
    [input.sessionId, input.participantId, input.text,
     input.hasQuestionMark ?? /[?？]\s*$/.test(input.text),
     input.startedAt ?? now, input.endedAt ?? now],
  )
  return row!
}

/**
 * 대명사·생략 해소.
 * "그거 되나요?" 는 직전 발화 없이는 검색이 불가능하다.
 * MVP에서는 직전 3개 발화를 앞에 붙여 임베딩 문맥으로 쓴다.
 * (LLM 정규화는 추가 호출이 되어 실시간 경로 예산을 넘는다)
 */
async function normalize(sessionId: string, text: string): Promise<string> {
  const t = text.trim()
  const hasPronoun = /(그거|그건|그게|이거|저거|그 부분|그쪽|아까|말씀하신)/.test(t)
  if (!hasPronoun) return t

  const prev = await query<{ text: string }>(
    `select text from utterance
      where session_id=$1 order by ended_at desc offset 1 limit 3`,
    [sessionId],
  )
  if (prev.length === 0) return t
  return `${prev.map((p) => p.text).reverse().join(' ')} ${t}`
}

/** 상대 발화 하나를 끝까지 처리하며 이벤트를 흘려보낸다. */
export async function* processUtterance(
  session: Session,
  utt: UtteranceRow,
): AsyncGenerator<PipelineEvent> {
  yield { type: 'utterance', utterance: utt }

  // host 발화는 문맥으로만 저장하고 질문 감지를 하지 않는다.
  if (utt.role === 'host') return

  const det = detectQuestion(utt.text, utt.has_question_mark)
  if (!det.isQuestion) return

  const t0 = Date.now()
  const normalized = await normalize(session.id, utt.text)
  const intent = extractIntent(normalized)

  const q = await queryOne<{ id: string }>(
    `insert into question
       (session_id, utterance_id, raw_text, normalized_text, intent,
        detected_by, response_mode)
     values ($1,$2,$3,$4,$5::question_intent,$6::detection_method,'escalate')
     returning id`,
    [session.id, utt.id, utt.text, normalized, intent,
     det.signal === 'question_mark' ? 'rule' : 'rule'],
  )
  const questionId = q!.id
  yield { type: 'question.detected', questionId, normalized, intent }

  // 판정 — 규칙·근거 병렬 검색
  const d = await decide(normalized, intent, session.context)
  const headline = buildHeadline(d.responseMode, normalized, d.rule?.headline_template, d.gapReason)
  const latencyMs = Date.now() - t0

  await query(
    `update question
        set response_mode=$2::response_mode, headline=$3,
            matched_rule_id=$4, latency_ms=$5
      where id=$1`,
    [questionId, d.responseMode, headline, d.matchedRuleId, latencyMs],
  )

  yield {
    type: 'question.verdict',
    questionId, responseMode: d.responseMode, headline,
    condition: d.condition, evidence: d.evidence, latencyMs,
  }

  // 답하지 못했으면 공백을 남긴다. 이 기록이 다음 답의 재료가 된다.
  if (d.gapReason) {
    const gap = await queryOne<{ id: string }>(
      `insert into knowledge_gap
         (question_id, session_id, domain, question_normalized, intent, reason)
       values ($1,$2,$3::domain_type,$4,$5::question_intent,$6::gap_reason)
       returning id`,
      [questionId, session.id, session.domain, normalized, intent, d.gapReason],
    )
    yield { type: 'gap.created', questionId, gapId: gap!.id, reason: d.gapReason }
  }

  // 문구 생성 — 무거운 모델 호출은 여기 한 번뿐이다.
  let script = ''
  for await (const delta of streamAnswer({
    question: normalized, intent, mode: d.responseMode,
    ctx: session.context, evidence: d.evidence, rule: d.rule,
  })) {
    script += delta
    yield { type: 'question.delta', questionId, delta }
  }
  yield { type: 'question.done', questionId, script: script.trim(), totalMs: Date.now() - t0 }
}
