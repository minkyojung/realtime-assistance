/**
 * 지식 검색.
 *
 * 핵심 설계: 접근 통제는 LLM이 아니라 이 함수의 WHERE 절이 한다.
 * 세션 컨텍스트로 허용 등급을 정하고, 그보다 높은 등급의 청크는
 * 애초에 결과에 들어오지 않는다. 모델이 실수해도 유출이 불가능하다.
 */
import { query } from '@/lib/db'
import { embedOne, toVector } from '@/lib/embed'
import { allowedLevels, type AudienceLevel } from '@/lib/audience'

export type ChunkHit = {
  id: string
  source_type: string
  source_url: string
  external_ref: string
  heading_path: string | null
  content: string
  audience_level: AudienceLevel
  valid_from: string
  distance: number
}

export async function searchChunks(
  question: string,
  ctx: Record<string, unknown>,
  limit = 5,
): Promise<ChunkHit[]> {
  const levels = allowedLevels(ctx)
  const vec = toVector(await embedOne(question))

  // halfvec 캐스팅은 인덱스 정의와 동일해야 인덱스를 탄다.
  return query<ChunkHit>(
    `select id, source_type, source_url, external_ref, heading_path,
            content, audience_level, valid_from,
            (embedding::halfvec(3072) <=> $1::halfvec(3072)) as distance
       from knowledge_chunk
      where audience_level = any($2::audience_level[])
        and (valid_until is null or valid_until >= current_date)
      order by distance
      limit $3`,
    [vec, levels, limit],
  )
}
