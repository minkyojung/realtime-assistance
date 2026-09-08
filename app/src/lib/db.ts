import { Pool } from 'pg'

/**
 * Postgres 연결 풀.
 * 스키마는 제출물 ERD(판교_3반_Relay-DB.dbml)에서 생성된 db/schema.sql 을 따른다.
 */
declare global {
  // eslint-disable-next-line no-var
  var __relayPool: Pool | undefined
}

export const pool =
  global.__relayPool ??
  new Pool({ connectionString: process.env.DATABASE_URL, max: 10 })

if (process.env.NODE_ENV !== 'production') global.__relayPool = pool

export async function query<T = Record<string, unknown>>(
  text: string,
  params?: unknown[],
): Promise<T[]> {
  const res = await pool.query(text, params as never[])
  return res.rows as T[]
}

/** 단건 조회. 없으면 null. */
export async function queryOne<T = Record<string, unknown>>(
  text: string,
  params?: unknown[],
): Promise<T | null> {
  const rows = await query<T>(text, params)
  return rows[0] ?? null
}

/**
 * 트랜잭션. 여러 쓰기가 전부 성공하거나 전부 취소되어야 할 때 쓴다.
 * 예: 공백 해소는 규칙 생성과 공백 갱신이 함께 성립해야 하며,
 *     중간에 실패하면 어느 쪽도 가리키지 않는 고아 규칙이 남는다.
 */
export async function transaction<T>(
  fn: (q: <R = Record<string, unknown>>(text: string, params?: unknown[]) => Promise<R[]>) => Promise<T>,
): Promise<T> {
  const client = await pool.connect()
  try {
    await client.query('BEGIN')
    const q = async <R = Record<string, unknown>>(text: string, params?: unknown[]) => {
      const res = await client.query(text, params as never[])
      return res.rows as R[]
    }
    const out = await fn(q)
    await client.query('COMMIT')
    return out
  } catch (e) {
    await client.query('ROLLBACK')
    throw e
  } finally {
    client.release()
  }
}
