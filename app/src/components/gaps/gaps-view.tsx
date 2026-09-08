'use client'

import { useCallback, useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { GapList } from './gap-list'
import { GapDetailPanel } from './gap-detail'
import type { GapDetail, GapSummary } from './types'

/**
 * 화면 2 — 지식 공백 큐.
 *
 * 3막 루프의 2막. 발화자가 답하지 못한 질문이 여기 쌓이고,
 * 승인자가 규칙으로 확정하면 다음 미팅부터 즉답이 된다.
 */
export function GapsView() {
  const [items, setItems] = useState<GapSummary[]>([])
  const [total, setTotal] = useState(0)
  const [selected, setSelected] = useState<GapDetail | null>(null)
  const [status, setStatus] = useState('open')
  const [sort, setSort] = useState('frequency')
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    const res = await fetch(`/api/deferrals?status=${status}&sort=${sort}`)
    const d = await res.json()
    setItems(d.items)
    setTotal(d.total)
    setLoading(false)
    return d.items as GapSummary[]
  }, [status, sort])

  useEffect(() => { void load() }, [load])

  async function select(id: string) {
    const res = await fetch(`/api/deferrals/${id}`)
    setSelected(await res.json())
  }

  async function afterResolve() {
    setSelected(null)
    const next = await load()
    if (next.length > 0) void select(next[0].id)
  }

  return (
    <div className="flex h-screen flex-col">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b px-5">
        <span className="font-semibold">Relay</span>
        <Separator orientation="vertical" className="h-4" />
        <h1 className="text-sm">지식 공백 큐</h1>
        <Badge variant="secondary" className="font-normal">검토 대기 {total}건</Badge>

        <div className="ml-auto flex items-center gap-2">
          <Select value={sort} onValueChange={(v) => v && setSort(v)}>
            <SelectTrigger className="h-8 w-[140px] text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="frequency">발생 빈도순</SelectItem>
              <SelectItem value="recent">최근순</SelectItem>
              <SelectItem value="oldest">오래된순</SelectItem>
            </SelectContent>
          </Select>
          <Select value={status} onValueChange={(v) => v && setStatus(v)}>
            <SelectTrigger className="h-8 w-[110px] text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="open">미해소</SelectItem>
              <SelectItem value="resolved">해소됨</SelectItem>
              <SelectItem value="rejected">기각</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-[340px_1fr]">
        <aside className="min-h-0 overflow-y-auto border-r">
          {loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">불러오는 중…</div>
          ) : (
            <GapList items={items} selectedId={selected?.id ?? null} onSelect={select} />
          )}
        </aside>

        <main className="min-h-0">
          {selected ? (
            <GapDetailPanel gap={selected} onResolved={afterResolve} />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              왼쪽에서 공백을 선택하세요
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
