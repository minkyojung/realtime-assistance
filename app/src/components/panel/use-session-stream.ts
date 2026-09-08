'use client'

import { useCallback, useRef, useState } from 'react'
import type { FeedItem, SuggestionItem, UtteranceItem } from './types'

/**
 * SSE 구독 → 대화 피드 상태.
 *
 * 발화와 제안이 하나의 배열에 시간순으로 섞인다.
 * 서버 이벤트를 그대로 반영하며, 클라이언트에서 판정을 다시 계산하지 않는다.
 */
export type SessionInfo = {
  id: string
  counterpart_org: string
  domain: string
  context: Record<string, unknown>
}

export function useSessionStream() {
  const [session, setSession] = useState<SessionInfo | null>(null)
  const [feed, setFeed] = useState<FeedItem[]>([])
  const [running, setRunning] = useState(false)
  const esRef = useRef<EventSource | null>(null)

  const start = useCallback(async (domain: 'sales' | 'recruiting' = 'sales') => {
    setFeed([])
    setRunning(true)

    const res = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(
        domain === 'sales'
          ? { domain, counterpartOrg: 'Acme Corp', context: { nda_signed: false, stage: 'discovery' } }
          : { domain, counterpartOrg: '지원자 김OO', context: { interview_round: 2, position_level: 'senior' } },
      ),
    })
    const { session: s } = await res.json()
    setSession(s)

    const es = new EventSource(`/api/sessions/${s.id}/stream`)
    esRef.current = es

    es.onmessage = (msg) => {
      const e = JSON.parse(msg.data)
      setFeed((prev) => {
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

          case 'script.done':
            es.close()
            setRunning(false)
            return next

          default:
            return next
        }
      })
    }

    es.onerror = () => { es.close(); setRunning(false) }
  }, [])

  const stop = useCallback(() => {
    esRef.current?.close()
    setRunning(false)
  }, [])

  return { session, feed, running, start, stop }
}
