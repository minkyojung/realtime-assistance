import type { NextConfig } from 'next'
import { config as loadEnv } from 'dotenv'
import { join } from 'node:path'

// .env 는 저장소 루트에 있다 (app/ 이 아니라).
// 쉘에서 source 하지 않아도 서버가 항상 같은 값을 읽도록 여기서 로드한다.
loadEnv({ path: join(process.cwd(), '..', '.env'), quiet: true })

const nextConfig: NextConfig = {
  // Next가 AGENTS.md / CLAUDE.md 를 자동 생성하지 않게 한다.
  // 이 저장소는 루트 문서 체계를 따로 관리한다.
  agentRules: false,
  // 화면 캡처에 dev 오버레이가 겹치지 않게 한다.
  devIndicators: false,
}

export default nextConfig
