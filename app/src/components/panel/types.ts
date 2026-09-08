/** 화면 1이 다루는 대화 항목. 발화와 제안이 한 흐름에 섞인다. */
export type ResponseMode = 'direct' | 'conditional' | 'escalate'

export type Evidence = {
  id: string
  source_type: string
  source_url: string
  external_ref: string
  heading_path: string | null
  audience_level: string
  valid_from: string
}

export type UtteranceItem = {
  kind: 'utterance'
  id: string
  role: 'host' | 'counterpart'
  text: string
  isFinal: boolean
}

export type SuggestionItem = {
  kind: 'suggestion'
  id: string            // questionId
  intent: string
  mode: ResponseMode | null   // null = 판정 대기 (스켈레톤)
  headline: string
  script: string
  condition: string | null
  evidence: Evidence[]
  latencyMs: number | null
  done: boolean
}

export type FeedItem = UtteranceItem | SuggestionItem
