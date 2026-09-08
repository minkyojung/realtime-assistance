import type { Evidence, FeedItem, ResponseMode, SuggestionItem, UtteranceItem } from './types'

/**
 * 스트림이 내려보내는 이벤트. `/api/sessions/{id}/stream` 의 계약이며
 * Swift `RelayCore/Event.swift` 가 그대로 옮겨야 하는 원본이다.
 *
 * 케이싱이 섞여 있다. 최상위는 camelCase(`questionId`), `utterance` 안은
 * snake_case DB row 다. 한쪽 규칙으로 통일하려 들지 말 것.
 */
export type StreamEvent =
  | { type: 'utterance.partial'; id: string; role: UtteranceItem['role']; text: string }
  | { type: 'utterance.final'; tempId: string; utterance: { id: string; role: UtteranceItem['role']; text: string } }
  | { type: 'question.detected'; questionId: string; normalized: string; intent: string }
  | {
      type: 'question.verdict'; questionId: string; responseMode: ResponseMode
      headline: string; condition: string | null; evidence: Evidence[]; latencyMs: number
    }
  | { type: 'question.delta'; questionId: string; delta: string }
  | { type: 'question.done'; questionId: string; script: string; totalMs: number }
  | { type: 'gap.created'; questionId: string; gapId: string; reason: string }
  | { type: 'script.done' }
  | { type: 'error'; message: string }

/**
 * 이벤트 하나를 피드에 반영한다. 순수 함수 — 훅과 `scripts/dump-feed.ts` 가 함께 쓴다.
 *
 * 모르는 `type` 은 무시한다(스트림에 이벤트가 추가돼도 화면이 죽지 않게).
 */
export function reduceFeed(prev: FeedItem[], e: StreamEvent): FeedItem[] {
  const next = [...prev]

  switch (e.type) {
    // ① 말하는 동안 자막이 채워진다
    case 'utterance.partial': {
      const i = next.findIndex((x) => x.kind === 'utterance' && x.id === e.id)
      const item: UtteranceItem = { kind: 'utterance', id: e.id, role: e.role, text: e.text, isFinal: false }
      if (i >= 0) next[i] = item
      else next.push(item)
      return next
    }
    case 'utterance.final': {
      const i = next.findIndex((x) => x.kind === 'utterance' && x.id === e.tempId)
      const item: UtteranceItem = {
        kind: 'utterance', id: e.utterance.id, role: e.utterance.role,
        text: e.utterance.text, isFinal: true,
      }
      if (i >= 0) next[i] = item
      else next.push(item)
      return next
    }

    // ② 질문 감지 → 스켈레톤 카드
    case 'question.detected':
      next.push({
        kind: 'suggestion', id: e.questionId, intent: e.intent, mode: null,
        headline: '', script: '', condition: null, evidence: [],
        latencyMs: null, done: false,
      } satisfies SuggestionItem)
      return next

    // ③ 판정 도착 → 색과 headline 즉시 표시
    case 'question.verdict': {
      const i = next.findIndex((x) => x.kind === 'suggestion' && x.id === e.questionId)
      if (i < 0) return next
      const cur = next[i] as SuggestionItem
      next[i] = {
        ...cur, mode: e.responseMode, headline: e.headline,
        condition: e.condition, evidence: e.evidence, latencyMs: e.latencyMs,
      }
      return next
    }

    // ④ 제안 문구가 채워진다
    case 'question.delta': {
      const i = next.findIndex((x) => x.kind === 'suggestion' && x.id === e.questionId)
      if (i < 0) return next
      const cur = next[i] as SuggestionItem
      next[i] = { ...cur, script: cur.script + e.delta }
      return next
    }
    case 'question.done': {
      const i = next.findIndex((x) => x.kind === 'suggestion' && x.id === e.questionId)
      if (i < 0) return next
      next[i] = { ...(next[i] as SuggestionItem), script: e.script, done: true }
      return next
    }

    default:
      return next
  }
}
