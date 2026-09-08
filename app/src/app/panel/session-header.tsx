'use client'

import { useEffect, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

type SessionHeaderProps = {
  /** `session.counterpart_org` */
  counterpartOrg: string
  /** `session.context.stage` — discovery / poc / negotiation … */
  stage: string
  /** `session.started_at` (ISO). 없으면 마운트 시각부터 센다. */
  startedAt?: string
  /** 오디오 캡처가 살아 있는지. 클라이언트 상태. */
  isCapturing: boolean
}

/** 경과 시간을 mm:ss 로, 한 시간이 넘으면 h:mm:ss 로 만든다. */
function formatElapsed(totalSeconds: number) {
  const seconds = String(totalSeconds % 60).padStart(2, '0')
  const minutes = Math.floor(totalSeconds / 60) % 60
  const hours = Math.floor(totalSeconds / 3600)
  if (hours > 0) return `${hours}:${String(minutes).padStart(2, '0')}:${seconds}`
  return `${String(minutes).padStart(2, '0')}:${seconds}`
}

/**
 * 화면 1 ① 세션 헤더.
 *
 * 통화 중에는 이 줄을 "읽지" 않는다. 녹음이 살아 있는지, 지금 누구와 있는지를
 * 힐끗 보고 판별하는 용도라 한 줄에 상태만 담고 조작 버튼은 두지 않는다.
 */
export function SessionHeader({
  counterpartOrg,
  stage,
  startedAt,
  isCapturing,
}: SessionHeaderProps) {
  const [elapsedSeconds, setElapsedSeconds] = useState(0)

  useEffect(() => {
    if (!isCapturing) return
    const base = startedAt ? Date.parse(startedAt) : Date.now()
    const tick = () =>
      setElapsedSeconds(Math.max(0, Math.floor((Date.now() - base) / 1000)))
    tick()
    const timer = setInterval(tick, 1000)
    return () => clearInterval(timer)
  }, [startedAt, isCapturing])

  return (
    <header className="drag-region flex h-10 shrink-0 items-center gap-2 border-b pr-3 pl-20">
      {/* 색만으로 상태를 알리지 않도록 텍스트를 함께 둔다. */}
      <span className="flex shrink-0 items-center gap-1.5">
        <span className="relative flex size-2">
          {isCapturing && (
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-red-500 opacity-60" />
          )}
          <span
            className={
              isCapturing
                ? 'relative inline-flex size-2 rounded-full bg-red-500'
                : 'relative inline-flex size-2 rounded-full bg-muted-foreground/50'
            }
          />
        </span>
        <span className="sr-only">{isCapturing ? '녹음 중' : '녹음 정지'}</span>
        <span className="font-mono text-xs tabular-nums text-muted-foreground">
          {formatElapsed(elapsedSeconds)}
        </span>
      </span>

      <Separator orientation="vertical" className="h-3.5 self-center bg-foreground/20" />

      <span className="truncate text-[13px] font-medium">{counterpartOrg}</span>
      <Badge variant="outline" className="shrink-0">
        {stage}
      </Badge>
    </header>
  )
}
