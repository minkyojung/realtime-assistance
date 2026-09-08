'use client'

import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { INTENT_LABEL, REASON_LABEL, SOURCE_LABEL, type GapDetail } from './types'

/**
 * 공백 상세 + 규칙 작성 폼.
 *
 * 승인자가 GitHub을 직접 검색하지 않아도 되도록 근거 후보를 미리 제시한다.
 * 이것이 2막의 마찰을 줄이는 핵심이다.
 */
export function GapDetailPanel({
  gap,
  onResolved,
}: {
  gap: GapDetail
  onResolved: () => void
}) {
  const [mode, setMode] = useState('direct')
  const [audience, setAudience] = useState('public')
  const [headline, setHeadline] = useState('')
  const [script, setScript] = useState('')
  const [condition, setCondition] = useState('')
  const [authority, setAuthority] = useState('')
  const [picked, setPicked] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function submit() {
    setSaving(true)
    setError(null)
    const res = await fetch(`/api/deferrals/${gap.id}/resolve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        domain: gap.domain,
        question_pattern: gap.question_normalized,
        intent: gap.intent,
        response_mode: mode,
        audience_level: audience,
        headline_template: headline,
        suggested_script: script,
        condition: condition || null,
        fallback_script: mode === 'conditional' ? script : null,
        authority_role: authority || null,
        chunk_ids: picked,
        valid_until: null,
      }),
    })
    setSaving(false)
    if (!res.ok) {
      const e = await res.json().catch(() => null)
      setError(e?.message ?? '저장에 실패했습니다')
      return
    }
    onResolved()
  }

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      {/* 공백 요약 */}
      <div className="border-b p-4">
        <h2 className="text-base font-semibold leading-snug">{gap.question_normalized}</h2>
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          <Badge variant="secondary" className="font-normal">
            {gap.occurrence_count}회 발생
          </Badge>
          <Badge variant="outline" className="font-normal">
            {REASON_LABEL[gap.reason] ?? gap.reason}
          </Badge>
          <Badge variant="outline" className="font-normal">
            {INTENT_LABEL[gap.intent] ?? gap.intent}
          </Badge>
          <span className="text-xs text-muted-foreground">
            최초 {gap.first_seen_at?.slice(0, 10)}
          </span>
        </div>
        <p className="mt-3 rounded-md border bg-muted/40 px-3 py-2 text-sm">
          <span className="mr-2 text-xs text-muted-foreground">원문</span>
          {gap.raw_utterance}
        </p>
      </div>

      {/* 근거 후보 — 시스템이 미리 찾아 둔다 */}
      <div className="border-b p-4">
        <div className="mb-2 flex items-center gap-2">
          <h3 className="text-sm font-medium">관련 GitHub 근거</h3>
          <span className="text-xs text-muted-foreground">
            {gap.intent === 'when' && '미확정 자료 우선'}
          </span>
        </div>
        <ul className="space-y-1.5">
          {gap.evidence_candidates.map((e) => {
            const on = picked.includes(e.id)
            return (
              <li key={e.id}>
                <button
                  onClick={() => setPicked((p) => (on ? p.filter((x) => x !== e.id) : [...p, e.id]))}
                  className={cn(
                    'flex w-full items-start gap-2 rounded-md border px-3 py-2 text-left text-xs transition-colors',
                    on ? 'border-foreground bg-muted' : 'hover:bg-muted/50',
                  )}
                >
                  <span className="mt-0.5">{on ? '☑' : '☐'}</span>
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-1.5">
                      <Badge
                        variant={e.audience_level === 'internal' ? 'destructive' : 'outline'}
                        className="font-normal text-[10px]"
                      >
                        {e.audience_level === 'internal' ? '대외비' : '공개가능'}
                      </Badge>
                      <span className="text-muted-foreground">
                        {SOURCE_LABEL[e.source_type] ?? e.source_type} {e.external_ref}
                      </span>
                    </span>
                    <span className="mt-1 block truncate">{e.heading_path}</span>
                  </span>
                  <a
                    href={e.source_url}
                    target="_blank"
                    rel="noreferrer"
                    onClick={(ev) => ev.stopPropagation()}
                    className="shrink-0 text-muted-foreground hover:text-foreground"
                  >
                    ↗
                  </a>
                </button>
              </li>
            )
          })}
        </ul>
      </div>

      {/* 규칙 작성 폼 */}
      <div className="flex-1 space-y-3 p-4">
        <h3 className="text-sm font-medium">답변 규칙 작성</h3>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">응답 모드</Label>
            <Select value={mode} onValueChange={(v) => v && setMode(v)}>
              <SelectTrigger className="h-9"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="direct">즉답 가능</SelectItem>
                <SelectItem value="conditional">조건부</SelectItem>
                <SelectItem value="escalate">담당자 연결</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">공개 등급</Label>
            <Select value={audience} onValueChange={(v) => v && setAudience(v)}>
              <SelectTrigger className="h-9"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="public">공개 가능</SelectItem>
                <SelectItem value="nda">NDA 체결 후</SelectItem>
                <SelectItem value="internal">대외 비공개</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="space-y-1.5">
          <Label className="text-xs">한 줄 요약 (화면에 크게 표시됨)</Label>
          <Input
            value={headline}
            onChange={(e) => setHeadline(e.target.value)}
            placeholder="예: 셀프호스팅 Edge Functions 미지원"
          />
        </div>

        <div className="space-y-1.5">
          <Label className="text-xs">이렇게 말하세요 (실제로 입에 낼 문구)</Label>
          <Textarea
            rows={4}
            value={script}
            onChange={(e) => setScript(e.target.value)}
            placeholder="고객에게 그대로 말할 수 있는 문장으로 작성합니다."
          />
        </div>

        {mode === 'conditional' && (
          <div className="space-y-1.5">
            <Label className="text-xs">조건 (미충족 시 경고로 표시됨)</Label>
            <Input
              value={condition}
              onChange={(e) => setCondition(e.target.value)}
              placeholder="예: NDA 체결 시에만 상세 언급 가능"
            />
          </div>
        )}

        <div className="space-y-1.5">
          <Label className="text-xs">승인권자</Label>
          <Input
            value={authority}
            onChange={(e) => setAuthority(e.target.value)}
            placeholder="예: 제품팀 리드"
          />
        </div>

        {error && <p className="text-xs text-destructive">{error}</p>}

        <Separator />

        <div className="flex items-center gap-2">
          <Button onClick={submit} disabled={saving || !headline || !script}>
            {saving ? '저장 중…' : `저장하고 해소 (${gap.occurrence_count}건)`}
          </Button>
          <span className="text-xs text-muted-foreground">
            같은 질문의 공백이 함께 정리됩니다
          </span>
        </div>
      </div>
    </div>
  )
}
