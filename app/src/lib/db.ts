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
