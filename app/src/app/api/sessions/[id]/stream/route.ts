import { getSession, participantByRole } from '@/lib/session'
import { saveUtterance, processUtterance, type PipelineEvent } from '@/lib/pipeline'
import { DEMO_SCRIPT, RECRUITING_SCRIPT, type ScriptLine } from '@/lib/scripted-source'

export const runtime = 'nodejs'

/**
 * GET /api/sessions/{id}/stream — 화면 1의 실시간 채널 (SSE)
 *
 * 설계 문서에서는 WebSocket이지만 단방향 푸시만 필요하므로 SSE로 구현한다.
 * 이벤트 종류와 순서는 동일하다.
 *
 * 이 채널로 나가는 스트리밍은 하나뿐이다 — `utterance.partial`(말하는 동안
 * 자막이 채워짐, STT 중간 결과 대응).
 *
 * 제안 문구(`question.delta`)는 여기로 오지 않는다. 사용자가 카드의 버튼을 눌렀을 때
 * `POST /api/questions/{id}/answer` 가 같은 형식으로 흘려보낸다.
 */

/** ScriptedSource: 한 줄을 글자 단위로 흘려보내 실제 발화처럼 보이게 한다. */
async function* speak(line: ScriptLine): AsyncGenerator<string> {
  const step = 3
  for (let i = step; i < line.text.length; i += step) {
    await new Promise((r) => setTimeout(r, 45))
    yield line.text.slice(0, i)
  }
}

export async function GET(req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const found = await getSession(id)
  if (!found) return new Response('세션을 찾을 수 없습니다', { status: 404 })

  const script =
    found.session.domain === 'recruiting' ? RECRUITING_SCRIPT : DEMO_SCRIPT
  const host = await participantByRole(id, 'host')
  const other = await participantByRole(id, 'counterpart')

  const encoder = new TextEncoder()
  const stream = new ReadableStream({
    async start(controller) {
      const send = (e: PipelineEvent | { type: string; [k: string]: unknown }) =>
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(e)}\n\n`))

      try {
        for (const [i, line] of script.entries()) {
          const p = line.role === 'host' ? host! : other!
          const tempId = `tmp-${i}`

          // ① 말하는 동안 — 자막이 채워진다
          send({ type: 'utterance.partial', id: tempId, role: line.role, text: '' })
          for await (const partial of speak(line)) {
            send({ type: 'utterance.partial', id: tempId, role: line.role, text: partial })
          }

          // 턴 종료 — 여기부터 지연 측정 대상
          const utt = await saveUtterance({
            sessionId: id, participantId: p.id,
            text: line.text, hasQuestionMark: line.questionMark,
          })
          send({ type: 'utterance.final', tempId, utterance: { ...utt, role: line.role } })

          // ② 파이프라인 — 판정과 제안 문구
          for await (const ev of processUtterance(found.session, utt)) {
            if (ev.type === 'utterance') continue // 위에서 이미 보냄
            send(ev)
          }

          await new Promise((r) => setTimeout(r, line.gapMs ?? 700))
        }
        send({ type: 'script.done' })
      } catch (e) {
        send({ type: 'error', message: e instanceof Error ? e.message : String(e) })
      } finally {
        controller.close()
      }
    },
  })

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/event-stream; charset=utf-8',
      'Cache-Control': 'no-cache, no-transform',
      Connection: 'keep-alive',
    },
  })
}
