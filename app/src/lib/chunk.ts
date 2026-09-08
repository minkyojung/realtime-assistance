/**
 * GitHub 원문을 검색 단위 조각으로 자른다.
 *
 * 설계 근거
 *  - 사실 검색이 목적이므로 조각은 작게 잡는다 (약 200~400 토큰).
 *  - `heading_path`(문서 구조 경로)를 반드시 보존한다. 조각만 봐도 맥락을
 *    알 수 있어야 검색 정확도가 유지된다.
 *  - 제목은 모든 조각에 붙인다. PR·이슈는 제목에 핵심 정보가 몰려 있다.
 */

export type RawDoc = {
  sourceType: 'release' | 'changelog' | 'merged_pr' | 'open_issue' | 'milestone'
  externalRef: string
  sourceUrl: string
  title: string
  body: string
  validFrom: string        // YYYY-MM-DD
  labels?: string[]
}

export type Chunk = {
  headingPath: string
  content: string
}

/** 대략 1토큰 ≈ 4자로 보고 1,200자를 상한으로 둔다. */
const MAX_CHARS = 1_200
const MIN_CHARS = 40

/** 검색에 도움이 안 되는 마크다운 잡음을 제거한다. */
function clean(md: string): string {
  return md
    .replace(/<!--[\s\S]*?-->/g, '')            // HTML 주석
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')       // 이미지
    .replace(/```[\s\S]*?```/g, ' [코드 생략] ') // 코드 블록
    .replace(/<\/?[a-z][^>]*>/gi, '')           // 인라인 HTML
    .replace(/\r/g, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/** 한 섹션을 길이 상한에 맞춰 문단 단위로 쪼갠다. */
function splitBySize(text: string): string[] {
  const paras = text.split(/\n\n+/).map((p) => p.trim()).filter(Boolean)
  const out: string[] = []
  let buf = ''

  for (const p of paras) {
    // 문단 하나가 상한을 넘으면 문장 단위로 다시 자른다.
    if (p.length > MAX_CHARS) {
      if (buf) { out.push(buf); buf = '' }
      let sent = ''
      for (const s of p.split(/(?<=[.!?。])\s+/)) {
        if ((sent + s).length > MAX_CHARS) { if (sent) out.push(sent); sent = s }
        else sent = sent ? `${sent} ${s}` : s
      }
      if (sent) out.push(sent)
      continue
    }
    if ((buf + '\n\n' + p).length > MAX_CHARS) { out.push(buf); buf = p }
    else buf = buf ? `${buf}\n\n${p}` : p
  }
  if (buf) out.push(buf)
  return out
}

export function chunkDoc(doc: RawDoc): Chunk[] {
  const body = clean(doc.body ?? '')
  const chunks: Chunk[] = []

  // 제목만 있고 본문이 없는 문서도 검색 대상이다 (PR 제목이 곧 사실인 경우가 많다).
  if (body.length < MIN_CHARS) {
    return [{ headingPath: doc.title, content: `${doc.title}\n\n${body}`.trim() }]
  }

  // 마크다운 헤딩을 기준으로 섹션을 나누고 계층 경로를 추적한다.
  const lines = body.split('\n')
  const stack: string[] = []
  let section: string[] = []
  const flush = () => {
    const text = section.join('\n').trim()
    section = []
    if (text.length < MIN_CHARS) return
    // 헤딩 레벨이 건너뛰면 스택에 빈 칸이 생기므로 걸러낸다.
    const path = [doc.title, ...stack.filter(Boolean)].join(' > ')
    for (const part of splitBySize(text)) {
      chunks.push({ headingPath: path, content: `${path}\n\n${part}` })
    }
  }

  for (const line of lines) {
    const h = /^(#{1,4})\s+(.*)$/.exec(line)
    if (h) {
      flush()
      stack.length = h[1].length - 1
      stack[h[1].length - 1] = h[2].trim()
    } else {
      section.push(line)
    }
  }
  flush()

  return chunks.length
    ? chunks
    : [{ headingPath: doc.title, content: `${doc.title}\n\n${body}`.slice(0, MAX_CHARS) }]
}
