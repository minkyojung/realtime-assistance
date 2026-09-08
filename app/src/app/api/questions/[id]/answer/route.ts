import { answerQuestion, QuestionNotFound } from '@/lib/answer'
import type { PipelineEvent } from '@/lib/pipeline'

export const runtime = 'nodejs'

/**
 * POST /api/questions/{id}/answer — 카드의 버튼이 부르는 곳.
 *
 * 세션 스트림(`/api/sessions/{id}/stream`)과 같은 형식의 SSE 를 돌려준다.
 * 이벤트 이름도 같다 — 화면은 어느 채널로 왔는지 알 필요가 없다.
 *
 * 왜 별도 요청인가 — 상대의 모든 질문에 답이 필요한 것은 아니고, 자동으로 만들면
 * 대화를 끊지 않으려고 1.5초 예산에 갇힌다. 누른 뒤에는 몇 초를 써도 되므로
 * 그 안에서 HyDE 로 다시 찾는다. 자세한 근거는 `lib/answer.ts`.
 */
export async function POST(_req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params

  const encoder = new TextEncoder()
  const stream = new ReadableStream({
    async start(controller) {
      const send = (e: PipelineEvent | { type: string; [k: string]: unknown }) =>
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(e)}\n\n`))

      try {
        for await (const event of answerQuestion(id)) send(event)
      } catch (e) {
        // 스트림이 이미 열렸으므로 상태 코드로는 알릴 수 없다. 화면이 읽는 것도
        // 본문의 `error` 이벤트다 (세션 스트림과 같은 처리).
        send({
          type: 'error',
          message: e instanceof QuestionNotFound
            ? '질문을 찾을 수 없습니다'
            : e instanceof Error ? e.message : String(e),
        })
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
