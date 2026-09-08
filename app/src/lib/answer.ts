/**
 * 답변 문구 생성 — **사용자가 카드의 버튼을 눌렀을 때만 돈다.**
 *
 * 실시간 경로(`pipeline.ts`)와 나뉘는 이유는 시간이다. 자동 생성은 대화를 끊지
 * 않으려고 1.5초 안에 검색까지 끝내야 해서 어휘 불일치를 고칠 수 없었고, 실제로
 * "온프레미스 배포 되나요?" 에 MCP 클라이언트 문서가 근거로 붙었다(거리 0.747).
 * 누른 뒤에는 몇 초를 써도 되므로 여기서는 HyDE 로 다시 찾는다.
 *
 *   규칙이 있다  승인된 문구를 그대로 (모델 없음, 실측 1~4ms)
 *                그 뒤에 HyDE 재검색 결과를 출처로 덧붙인다 — 답은 안 기다린다
 *   규칙이 없다  HyDE(~1s) -> 재검색 -> 근거 하한 통과분으로 생성 (실측 4.0s)
 *
 * 나가는 이벤트는 세션 스트림의 것과 같다(`question.verdict` · `.delta` · `.done`).
 * 이름을 새로 만들지 않는 이유 — 페이로드가 같은데 타입만 늘리면 화면 리듀서가
 * 두 벌이 되고, 웹·Swift 양쪽이 그만큼 갈라진다.
 */
import { queryOne } from '@/lib/db'
import { embedOne } from '@/lib/embed'
import { hypotheticalDocument } from '@/lib/hyde'
import { searchChunks, type ChunkHit } from '@/lib/search'
import { streamAnswer } from '@/lib/generate'
import { conditionMet, EVIDENCE_MAX_DISTANCE, type ResponseMode } from '@/lib/verdict'
import type { PipelineEvent } from '@/lib/pipeline'

/** 재검색 후보 수. 하한(`EVIDENCE_MAX_DISTANCE`)으로 거른 뒤 상위 3건만 쓴다.
 *  실시간 경로(3건)보다 넓게 잡는 이유는 HyDE 로 벡터가 바뀌어 순위도 바뀌기 때문이다. */
const DEEP_SEARCH_LIMIT = 10

type QuestionRow = {
  id: string
  normalized_text: string
  intent: string
  response_mode: ResponseMode
  headline: string | null
  condition: string | null
  matched_rule_id: string | null
  latency_ms: number | null
  context: Record<string, unknown>
}

type RuleRow = {
  suggested_script: string
  condition: string | null
  fallback_script: string | null
  headline_template: string
}

export class QuestionNotFound extends Error {}

/** 질문 하나의 답을 만들며 이벤트를 흘려보낸다. */
export async function* answerQuestion(questionId: string): AsyncGenerator<PipelineEvent> {
  const t0 = Date.now()

  const q = await queryOne<QuestionRow>(
    `select q.id, q.normalized_text, q.intent::text as intent,
            q.response_mode::text as response_mode, q.headline,
            r.condition, q.matched_rule_id, q.latency_ms, s.context
       from question q
       join session s on s.id = q.session_id
       left join response_rule r on r.id = q.matched_rule_id
      where q.id = $1`,
    [questionId],
  )
  if (!q) throw new QuestionNotFound(questionId)

  // ① 승인된 규칙이 있으면 그 문구가 답이다. 답을 먼저 내보내고 출처를 뒤에 붙인다.
  //
  //    모델로 다듬지 않는다. 승인자가 확정한 문장이 글자 그대로 나가는 편이
  //    안전하고, 다듬을 이유가 있었다면 승인 단계에서 다듬었을 것이다.
  if (q.matched_rule_id) {
    const rule = await queryOne<RuleRow>(
      `select suggested_script, condition, fallback_script, headline_template
         from response_rule where id = $1`,
      [q.matched_rule_id],
    )
    if (rule) {
      // 조건이 안 맞으면 대체 문구다. `decide()` 가 판정할 때 쓴 것과 같은 함수라
      // 여기서 다른 결론이 나올 수 없다.
      const met = conditionMet(rule.condition, q.context)
      const script = (met ? rule.suggested_script : rule.fallback_script) ?? rule.suggested_script

      yield { type: 'question.delta', questionId, delta: script }
      yield { type: 'question.done', questionId, script: script.trim(), totalMs: Date.now() - t0 }

      // 답은 이미 나갔다. 출처는 뒤이어 채운다.
      //
      // 승인 문구 자체의 근거는 승인자이지 문서가 아니지만, 화면에 출처가 아예
      // 없으면 사용자가 확인할 방법도 없어진다. 재검색은 몇 초가 걸리므로
      // **답을 붙잡아 두지 않고** 나중에 조용히 덧붙인다 — 기다림은 0초 그대로다.
      yield await refreshedVerdict(questionId, q)
      return
    }
  }

  // ② 규칙이 없다 — 다시 찾고, 그 근거로 문구를 만든다.
  const verdict = await refreshedVerdict(questionId, q)
  yield verdict
  yield* generate(questionId, q, verdict.evidence, t0)
}

/** HyDE 로 다시 찾은 근거를 실어 판정을 한 번 더 보낸다.
 *
 *  **판정 자체는 바꾸지 않는다.** 근거를 더 잘 찾았다는 것이 "답해도 된다"는 뜻은
 *  아니다. 그 결정은 승인자의 몫이고, 여기서 뒤집으면 승인 절차가 무의미해진다.
 *  바뀌는 것은 `evidence` 하나뿐이다. */
async function refreshedVerdict(questionId: string, q: QuestionRow) {
  // HyDE: 질문에 대한 '가상의 답변 문서'를 만들어 그것으로 검색한다. 생성 과정에서
  // 문서가 실제로 쓰는 용어가 나온다. 실측 0.747 -> 0.637 (온프레미스 질문).
  // 생성이 실패하면 원 질문으로 검색한다 — 근거 품질만 떨어지고 화면은 돈다.
  const hypothetical = await hypotheticalDocument(q.normalized_text)
  const vector = await embedOne(hypothetical ?? q.normalized_text)
  const hits = await searchChunks(q.normalized_text, q.context, DEEP_SEARCH_LIMIT, vector)

  return {
    type: 'question.verdict' as const,
    questionId,
    responseMode: q.response_mode,
    headline: q.headline ?? '',
    condition: q.condition,
    evidence: hits.filter((e) => e.distance <= EVIDENCE_MAX_DISTANCE).slice(0, 3),
    latencyMs: q.latency_ms ?? 0,
  }
}

/** 근거가 없으면 없는 대로 말하게 한다 — `streamAnswer` 의 escalate 프롬프트가
 *  "추측하지 말 것"과 "회신 시점을 제시할 것"을 이미 지시하고 있다. */
async function* generate(
  questionId: string,
  q: QuestionRow,
  evidence: ChunkHit[],
  t0: number,
): AsyncGenerator<PipelineEvent> {
  let script = ''
  for await (const delta of streamAnswer({
    question: q.normalized_text,
    intent: q.intent,
    mode: q.response_mode,
    ctx: q.context,
    evidence,
    rule: null,
  })) {
    script += delta
    yield { type: 'question.delta', questionId, delta }
  }
  yield { type: 'question.done', questionId, script: script.trim(), totalMs: Date.now() - t0 }
}
