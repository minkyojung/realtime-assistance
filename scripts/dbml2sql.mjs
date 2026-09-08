// 제출물 DBML을 Postgres DDL로 변환한다.
// ERD와 실제 DB가 갈라지지 않도록 스키마는 항상 여기서 생성한다.
import { exporter } from '@dbml/core'
import { readFileSync, writeFileSync } from 'node:fs'

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

const dbml = readFileSync('판교_3반_Relay-DB.dbml', 'utf8')
writeFileSync('db/schema.sql', HEADER + exporter.export(dbml, 'postgres'))
console.log('db/schema.sql 생성 완료')
