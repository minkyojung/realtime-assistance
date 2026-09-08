-- =============================================================
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

CREATE TYPE "domain_type" AS ENUM (
  'sales',
  'recruiting'
);

CREATE TYPE "session_status" AS ENUM (
  'active',
  'ended'
);

CREATE TYPE "participant_role" AS ENUM (
  'host',
  'counterpart'
);

CREATE TYPE "question_intent" AS ENUM (
  'exists',
  'status',
  'when',
  'price',
  'how',
  'other'
);

CREATE TYPE "response_mode" AS ENUM (
  'direct',
  'conditional',
  'escalate'
);

CREATE TYPE "audience_level" AS ENUM (
  'public',
  'nda',
  'internal'
);

CREATE TYPE "detection_method" AS ENUM (
  'rule',
  'model'
);

CREATE TYPE "gap_reason" AS ENUM (
  'no_rule',
  'no_knowledge',
  'condition_unmet',
  'unconfirmed_timeline',
  'user_rejected'
);

CREATE TYPE "gap_status" AS ENUM (
  'open',
  'resolved',
  'rejected'
);

CREATE TYPE "source_type" AS ENUM (
  'release',
  'changelog',
  'merged_pr',
  'open_issue',
  'milestone'
);

CREATE TYPE "index_job_status" AS ENUM (
  'idle',
  'running',
  'failed'
);

CREATE TABLE "session" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "domain" domain_type NOT NULL,
  "counterpart_org" varchar(200) NOT NULL,
  "context" jsonb NOT NULL DEFAULT '{}',
  "status" session_status NOT NULL DEFAULT 'active',
  "started_at" timestamptz NOT NULL DEFAULT (now()),
  "ended_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "participant" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "session_id" uuid NOT NULL,
  "role" participant_role NOT NULL,
  "display_name" varchar(100),
  "speaker_label" varchar(10),
  "voice_profile_id" varchar(100),
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "utterance" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "session_id" uuid NOT NULL,
  "participant_id" uuid NOT NULL,
  "text" text NOT NULL,
  "is_final" boolean NOT NULL DEFAULT false,
  "has_question_mark" boolean NOT NULL DEFAULT false,
  "started_at" timestamptz NOT NULL,
  "ended_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "question" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "session_id" uuid NOT NULL,
  "utterance_id" uuid UNIQUE NOT NULL,
  "raw_text" text NOT NULL,
  "normalized_text" text NOT NULL,
  "intent" question_intent NOT NULL,
  "detected_by" detection_method NOT NULL,
  "response_mode" response_mode NOT NULL,
  "headline" text,
  "matched_rule_id" uuid,
  "latency_ms" integer,
  "detected_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "knowledge_source" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "provider" varchar(50) NOT NULL DEFAULT 'github',
  "repo" varchar(200) NOT NULL,
  "last_indexed_at" timestamptz,
  "job_status" index_job_status NOT NULL DEFAULT 'idle',
  "vocabulary" jsonb NOT NULL DEFAULT '[]',
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "knowledge_chunk" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "source_id" uuid NOT NULL,
  "source_type" source_type NOT NULL,
  "source_url" varchar(500) NOT NULL,
  "external_ref" varchar(100) NOT NULL,
  "heading_path" varchar(500),
  "content" text NOT NULL,
  "embedding" vector(3072),
  "audience_level" audience_level NOT NULL,
  "valid_from" date NOT NULL,
  "valid_until" date,
  "indexed_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "response_rule" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "domain" domain_type NOT NULL,
  "question_pattern" text NOT NULL,
  "pattern_embedding" vector(3072),
  "intent" question_intent NOT NULL,
  "response_mode" response_mode NOT NULL,
  "audience_level" audience_level NOT NULL,
  "headline_template" text NOT NULL,
  "suggested_script" text NOT NULL,
  "condition" text,
  "fallback_script" text,
  "authority_role" varchar(100),
  "created_from" uuid UNIQUE,
  "is_active" boolean NOT NULL DEFAULT true,
  "valid_from" date NOT NULL,
  "valid_until" date,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "response_rule_chunk" (
  "rule_id" uuid,
  "chunk_id" uuid,
  "note" text,
  PRIMARY KEY ("rule_id", "chunk_id")
);

CREATE TABLE "knowledge_gap" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "question_id" uuid UNIQUE NOT NULL,
  "session_id" uuid NOT NULL,
  "domain" domain_type NOT NULL,
  "question_normalized" text NOT NULL,
  "intent" question_intent NOT NULL,
  "reason" gap_reason NOT NULL,
  "status" gap_status NOT NULL DEFAULT 'open',
  "resolved_by" uuid,
  "resolved_at" timestamptz,
  "rejection_reason" text,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX "idx_session_domain_time" ON "session" ("domain", "started_at");

CREATE INDEX ON "session" ("status");

CREATE INDEX "idx_participant_session_role" ON "participant" ("session_id", "role");

CREATE INDEX "idx_utterance_session_time" ON "utterance" ("session_id", "ended_at");

CREATE INDEX ON "utterance" ("participant_id");

CREATE INDEX "idx_question_session_time" ON "question" ("session_id", "detected_at");

CREATE INDEX ON "question" ("matched_rule_id");

CREATE INDEX ON "question" ("intent");

CREATE INDEX "idx_question_normalized" ON "question" ("normalized_text");

CREATE UNIQUE INDEX "uq_source_repo" ON "knowledge_source" ("provider", "repo");

CREATE INDEX ON "knowledge_chunk" ("source_id");

CREATE INDEX ON "knowledge_chunk" ("audience_level");

CREATE INDEX ON "knowledge_chunk" ("source_type");

CREATE INDEX "idx_chunk_validity" ON "knowledge_chunk" ("valid_from", "valid_until");


CREATE INDEX "idx_rule_search" ON "response_rule" ("domain", "intent", "audience_level");

CREATE INDEX ON "response_rule" ("is_active");

CREATE INDEX ON "response_rule" ("valid_until");

CREATE INDEX ON "response_rule" ("created_from");


CREATE INDEX ON "response_rule_chunk" ("chunk_id");

CREATE INDEX "idx_gap_queue" ON "knowledge_gap" ("status", "created_at");

CREATE INDEX "idx_gap_frequency" ON "knowledge_gap" ("question_normalized", "domain");

CREATE INDEX ON "knowledge_gap" ("reason");

CREATE INDEX ON "knowledge_gap" ("resolved_by");

COMMENT ON TABLE "session" IS '미팅 한 건. 화면 1-① 세션 헤더의 원천.';

COMMENT ON COLUMN "session"."domain" IS '도메인. 채용으로 바꿔도 스키마 불변';

COMMENT ON COLUMN "session"."counterpart_org" IS '상대 조직명';

COMMENT ON COLUMN "session"."context" IS '도메인별 상황 변수. 컬럼이 아니라 jsonb인 것이 핵심 설계.
  세일즈 {"nda_signed": false, "stage": "discovery"}
  채용   {"interview_round": 2, "position_level": "senior"}
컬럼으로 고정하면 도메인 추가 시 마이그레이션이 발생하여
"스키마 변경 없는 도메인 교체" 검증 조건이 깨진다.
';

COMMENT ON TABLE "participant" IS '화자를 enum이 아니라 테이블로 둔 이유.
session 1건에 participant N건이므로 상대가 늘어나도 행만 늘고 스키마는 불변이다.
질문 감지 조건은 role != host 이므로 코드도 변하지 않는다.
';

COMMENT ON COLUMN "participant"."display_name" IS '화면 표시용';

COMMENT ON COLUMN "participant"."speaker_label" IS '1:N 확장용. MVP(1:1)에서는 항상 NULL.
상대가 여러 명이면 A, B, C 가 들어간다.
컬럼을 지금 만들어 두는 것이 "스키마 변경 없는 확장"의 근거다.
';

COMMENT ON COLUMN "participant"."voice_profile_id" IS '대면 환경에서 host 식별용 음성 등록 ID';

COMMENT ON TABLE "utterance" IS '발화 한 마디. 화면 1-④ 전사 스트림. 대명사 해소를 위해 host 발화도 저장한다.';

COMMENT ON COLUMN "utterance"."text" IS 'STT 전사. 어휘 프롬프트 주입 후 결과';

COMMENT ON COLUMN "utterance"."is_final" IS 'STT 인터림/최종 구분';

COMMENT ON COLUMN "utterance"."has_question_mark" IS 'STT가 예측한 물음표 유무. 질문 감지 1차 신호.
한국어 종결어미 모호성을 STT가 대신 해석해 준 결과이며
스모크 테스트에서 질문/비질문 5/5 정확히 분리됨.
';

COMMENT ON COLUMN "utterance"."ended_at" IS '턴 종료 시각. 지연 측정 기준점';

COMMENT ON TABLE "question" IS '감지된 질문과 판정 결과. 화면 1-② 판정 카드, 1-③ 히스토리의 원천.
커버리지 지표는 이 테이블 건수와 knowledge_gap 건수로 계산되므로
별도 집계 테이블이 필요하지 않다.
';

COMMENT ON COLUMN "question"."utterance_id" IS '1:0..1 관계. 모든 발화가 질문은 아니며,
질문으로 감지된 발화에만 이 행이 생긴다.
';

COMMENT ON COLUMN "question"."raw_text" IS '발화 원문';

COMMENT ON COLUMN "question"."normalized_text" IS '대명사·생략 해소 후 정규화된 질문.
"그거 되나요?" -> "Edge Functions 셀프호스팅 지원 여부"
';

COMMENT ON COLUMN "question"."intent" IS '스파이크 1에서 추가된 컬럼.
임베딩 유사도만으로는 "지원되나요"(0.86)와 "언제 지원되나요"를 구분할 수 없어
(하드네거티브 최대 유사도 0.859, margin -0.53) 의도를 별도 축으로 분리했다.
검색 시 WHERE intent = :intent 로 걸러진다.
';

COMMENT ON COLUMN "question"."headline" IS 'AI 생성 한 줄 답변. 스트리밍 우선 출력';

COMMENT ON COLUMN "question"."matched_rule_id" IS '매칭된 규칙. 없으면 gap 생성';

COMMENT ON COLUMN "question"."latency_ms" IS '턴 종료 -> 첫 토큰. 목표 1500ms';

COMMENT ON TABLE "knowledge_source" IS '연결된 GitHub 레포. 화면 3-④ 소스 관리.';

COMMENT ON COLUMN "knowledge_source"."repo" IS '예: supabase/supabase';

COMMENT ON COLUMN "knowledge_source"."vocabulary" IS '레포에서 추출한 고유명사 사전. STT 프롬프트로 주입된다.
["SAML", "SSO", "SOC 2", "Edge Functions", ...]
스모크 테스트에서 이 주입 없이는 SOC 2 -> "사기", SAML -> "생리수혜수" 로 전사되어
BM25 검색이 전면 실패했다. 지식 소스가 인식 정확도를 올리는 구조.
';

COMMENT ON TABLE "knowledge_chunk" IS 'Layer 1 — 출처이지 답이 아니다. 화면 1-②-1 근거 펼침, 2-③ 근거 후보.';

COMMENT ON COLUMN "knowledge_chunk"."source_type" IS '인입 시점에 audience_level을 강제 결정한다';

COMMENT ON COLUMN "knowledge_chunk"."source_url" IS 'GitHub 원문 링크. 화면에 근거로 노출';

COMMENT ON COLUMN "knowledge_chunk"."external_ref" IS '이슈/PR 번호, 릴리즈 태그';

COMMENT ON COLUMN "knowledge_chunk"."heading_path" IS '문서 구조 경로. "Self-Hosting > Auth > SAML"
검색 정확도에 결정적이므로 청킹 시 반드시 보존한다.
';

COMMENT ON COLUMN "knowledge_chunk"."content" IS '본문 조각. 200~400 토큰';

COMMENT ON COLUMN "knowledge_chunk"."embedding" IS 'OpenAI text-embedding-3-large.
스파이크 1 실측에서 3-small 대비 top1 정확도 66% -> 86%.
';

COMMENT ON COLUMN "knowledge_chunk"."audience_level" IS 'source_type 에서 파생되어 인입 파이프라인이 강제 지정한다.
LLM 프롬프트가 아니라 검색 WHERE 절에서 사용된다.
  release/changelog/merged_pr -> public
  open_issue/milestone        -> internal
';

COMMENT ON TABLE "response_rule" IS 'Layer 2 — 사람이 승인한 확정 답변과 말하는 방식.
화면 1-② 판정 카드, 2-④ 작성 폼, 3-③ 규칙 리스트.
';

COMMENT ON COLUMN "response_rule"."question_pattern" IS '매칭 기준 질문 패턴. 임베딩됨';

COMMENT ON COLUMN "response_rule"."intent" IS '이 규칙이 답할 수 있는 의도. WHERE 절 필터';

COMMENT ON COLUMN "response_rule"."headline_template" IS '한 줄 답변 템플릿';

COMMENT ON COLUMN "response_rule"."suggested_script" IS '실제로 입에 낼 문구. 이 제품이 파는 것.
"Enterprise 플랜에서 온프레미스를 지원합니다."
';

COMMENT ON COLUMN "response_rule"."condition" IS 'conditional 일 때 충족되어야 하는 조건';

COMMENT ON COLUMN "response_rule"."fallback_script" IS '조건 미충족 시 대신 할 말';

COMMENT ON COLUMN "response_rule"."authority_role" IS '승인권자. 세일즈=영업팀장 / 채용=인사팀';

COMMENT ON COLUMN "response_rule"."created_from" IS '이 규칙이 태어난 공백. knowledge_gap.resolved_by 와 양방향을 이룬다.
"쓸수록 좋아진다"는 주장의 데이터 증거이므로 반드시 유지한다.
';

COMMENT ON COLUMN "response_rule"."valid_until" IS '만료 임박 알림의 기준. 화면 3-②';

COMMENT ON TABLE "response_rule_chunk" IS '규칙과 근거 청크의 M:N. 한 규칙이 여러 GitHub 근거를 인용할 수 있다.';

COMMENT ON COLUMN "response_rule_chunk"."note" IS '이 근거를 채택한 이유';

COMMENT ON TABLE "knowledge_gap" IS '답하지 못한 질문. 일급 엔티티다.

빈도순으로 정렬되어 담당팀에게 노출되는 것이 핵심이다.
전통적 사내 위키는 "무엇을 써야 할지 모른다"는 이유로 비어 있지만
Relay는 대화가 우선순위를 정해 준다.
"이 12개에 답해 주세요. 각각 3회, 5회 물어봤습니다."

화면 2-② 큐 리스트, 2-③ 상세.
';

COMMENT ON COLUMN "knowledge_gap"."question_normalized" IS '빈도 집계 키. 동일 질문 반복을 묶는다';

COMMENT ON COLUMN "knowledge_gap"."resolved_by" IS '이 공백을 해소한 규칙. response_rule.created_from 과 양방향을 이룬다.

관계가 방향에 따라 다르다는 점이 중요하다.
  created_from  1:1  한 규칙은 하나의 공백에서 태어난다
  resolved_by   N:1  여러 공백이 하나의 규칙으로 함께 해소된다

같은 질문이 6회 반복돼 공백이 6건 쌓였다면, 승인자가 규칙 하나를
만드는 순간 6건이 모두 그 규칙을 가리키며 해소된다.
이것이 "한 번 답하면 반복 질문이 한꺼번에 사라진다"는 동작의 근거다.
';

COMMENT ON COLUMN "knowledge_gap"."rejection_reason" IS '기각 사유';

ALTER TABLE "participant" ADD FOREIGN KEY ("session_id") REFERENCES "session" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "utterance" ADD FOREIGN KEY ("session_id") REFERENCES "session" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "utterance" ADD FOREIGN KEY ("participant_id") REFERENCES "participant" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "question" ADD FOREIGN KEY ("session_id") REFERENCES "session" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "question" ADD FOREIGN KEY ("utterance_id") REFERENCES "utterance" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "question" ADD FOREIGN KEY ("matched_rule_id") REFERENCES "response_rule" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "knowledge_chunk" ADD FOREIGN KEY ("source_id") REFERENCES "knowledge_source" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "response_rule" ADD FOREIGN KEY ("created_from") REFERENCES "knowledge_gap" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "response_rule_chunk" ADD FOREIGN KEY ("rule_id") REFERENCES "response_rule" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "response_rule_chunk" ADD FOREIGN KEY ("chunk_id") REFERENCES "knowledge_chunk" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "knowledge_gap" ADD FOREIGN KEY ("question_id") REFERENCES "question" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "knowledge_gap" ADD FOREIGN KEY ("session_id") REFERENCES "session" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "knowledge_gap" ADD FOREIGN KEY ("resolved_by") REFERENCES "response_rule" ("id") DEFERRABLE INITIALLY IMMEDIATE;

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
