'use client'

import { useEffect, useRef } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { useSessionStream } from './use-session-stream'
import { UtteranceBubble } from './utterance-bubble'
import { SuggestionCard } from './suggestion-card'

/**
 * 화면 1 — 실시간 어시스트 패널.
 *
 * 대화형 레이아웃. 발화(실선)와 제안(점선)이 한 흐름에 시간순으로 섞인다.
 * 사용자는 아무것도 입력하지 않는다. 상대가 말을 끝내면 카드가 저절로 뜬다.
 */
export function PanelView() {
  const { session, feed, running, start } = useSessionStream()
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
  }, [feed])

  const ctx = session?.context ?? {}
  const ndaSigned = ctx.nda_signed === true

  return (
    // data-vibrancy: Electron 창의 NSVisualEffectView 재질을 덮지 않도록
    // globals.css 가 이 속성을 보고 body 배경을 투명하게 내린다.
    <div data-vibrancy className="flex h-screen flex-col">
      {/* ① 세션 헤더 — Electron 창 드래그 영역 */}
      <header className="drag-region flex h-12 shrink-0 items-center gap-2 border-b pl-20 pr-3">
        <span className="text-sm font-semibold">Relay</span>
        {session ? (
          <>
            <Separator orientation="vertical" className="h-4" />
            <span className="text-sm">{session.counterpart_org}</span>
            <Badge variant="secondary" className="font-normal">{session.domain}</Badge>
            <Badge variant="outline" className="font-normal">
              {ndaSigned ? 'NDA 체결' : 'NDA 미체결'}
            </Badge>
          </>
        ) : (
          <span className="text-xs text-muted-foreground">세션이 시작되지 않았습니다</span>
        )}
        <div className="ml-auto flex items-center gap-1">
          {running && (
            <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <span className="size-1.5 animate-pulse rounded-full bg-red-600" />
              녹음 중
            </span>
          )}
        </div>
      </header>

      {/* ②③ 대화 피드 — 발화와 제안이 한 흐름 */}
      <div className="flex-1 overflow-y-auto">
        <div className="flex flex-col gap-3 p-3">
          {feed.length === 0 && (
            <div className="mt-24 text-center text-sm text-muted-foreground">
              아직 감지된 대화가 없습니다.
              <br />
              아래에서 미팅을 시작하세요.
            </div>
          )}
          {feed.map((item) =>
            item.kind === 'utterance' ? (
              <UtteranceBubble key={item.id} item={item} />
            ) : (
              <SuggestionCard key={item.id} item={item} />
            ),
          )}
          <div ref={endRef} />
        </div>
      </div>

      {/* 하단 — ScriptedSource 재생 컨트롤 (실제 제품에서는 오디오 캡처 토글) */}
      <footer className="flex shrink-0 items-center gap-2 border-t p-3">
        <Button size="sm" disabled={running} onClick={() => start('sales')} className="no-drag">
          {running ? '진행 중…' : '미팅 시작 (세일즈)'}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={running}
          onClick={() => start('recruiting')}
          className="no-drag"
        >
          채용 도메인
        </Button>
        <span className="ml-auto text-[10px] text-muted-foreground">
          ScriptedSource · 오디오 미사용
        </span>
      </footer>
    </div>
  )
}
