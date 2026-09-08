// 제출물 DBML을 Postgres DDL로 변환한다.
// ERD와 실제 DB가 갈라지지 않도록 스키마는 항상 여기서 생성한다.
import { exporter } from '@dbml/core'
import { readFileSync, writeFileSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

// cwd와 무관하게 저장소 루트를 기준으로 동작한다.
const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')

const HEADER = `-- =============================================================
--  Relay 스키마
--
--  이 파일은 제출물 ERD(판교_3반_Relay-DB.dbml)에서 생성되었다.
--  수정하지 말 것. DBML을 고치고 재생성한다.
--
--  재생성:  node scripts/dbml2sql.mjs
--  적용  :  psql "$DATABASE_URL" -f db/schema.sql
-- =============================================================

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()

`

// DBML은 표현식 인덱스를 표기할 수 없으므로 벡터 인덱스만 따로 붙인다.
// vector 타입은 HNSW 최대 2,000차원이라 3,072차원은 halfvec 캐스팅이 필요하다.
const VECTOR_INDEXES = `
-- ---------------------------------------------------------------
--  벡터 인덱스 (DBML로 표현할 수 없어 생성기에서 부착)
--
--  vector(3072) 는 HNSW 인덱스 상한(2,000차원)을 넘으므로
--  halfvec 로 캐스팅해 인덱싱한다. 검색 시에도 동일하게 캐스팅한다.
-- ---------------------------------------------------------------
DROP INDEX IF EXISTS "idx_chunk_embedding";
DROP INDEX IF EXISTS "idx_rule_embedding";

CREATE INDEX IF NOT EXISTS "idx_chunk_embedding"
  ON "knowledge_chunk" USING hnsw ((embedding::halfvec(3072)) halfvec_cosine_ops);

CREATE INDEX IF NOT EXISTS "idx_rule_embedding"
  ON "response_rule" USING hnsw ((pattern_embedding::halfvec(3072)) halfvec_cosine_ops);
`

const dbml = readFileSync(join(ROOT, '판교_3반_Relay-DB.dbml'), 'utf8')
const sql = exporter.export(dbml, 'postgres')
  // 생성된 btree 벡터 인덱스는 제거한다 (인덱스 튜플 8KB 상한 초과)
  .replace(/CREATE INDEX "idx_(chunk|rule)_(embedding|embedding)" ON[^;]+;\n?/g, '')
writeFileSync(join(ROOT, 'db/schema.sql'), HEADER + sql + VECTOR_INDEXES)
console.log('db/schema.sql 생성 완료')
