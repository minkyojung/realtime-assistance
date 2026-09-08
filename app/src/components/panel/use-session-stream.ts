'use client'

import { useCallback, useRef, useState } from 'react'
import { reduceFeed, type StreamEvent } from './reduce-feed'
import type { FeedItem } from './types'

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
      const e = JSON.parse(msg.data) as StreamEvent
      setFeed((prev) => reduceFeed(prev, e))
      if (e.type === 'script.done') {
        es.close()
        setRunning(false)
      }
    }

    es.onerror = () => { es.close(); setRunning(false) }
  }, [])

  const stop = useCallback(() => {
    esRef.current?.close()
    setRunning(false)
  }, [])

  return { session, feed, running, start, stop }
}
