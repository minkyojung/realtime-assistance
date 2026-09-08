import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  // Next가 AGENTS.md / CLAUDE.md 를 자동 생성하지 않게 한다.
  // 이 저장소는 루트 문서 체계를 따로 관리한다.
  agentRules: false,
  // 화면 캡처에 dev 오버레이가 겹치지 않게 한다.
  devIndicators: false,
}

export default nextConfig
