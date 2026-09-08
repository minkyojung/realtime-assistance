/**
 * ScriptedSource — 오디오 대신 대화 스크립트를 재생한다.
 *
 * 설계 문서의 AudioSource 인터페이스 중 하나이며,
 * LiveAudioSource(실제 STT)와 파이프라인 이후 단계를 완전히 동일하게 탄다.
 * 데모 안정성 확보용이자, 오디오 없이 파이프라인을 재현 가능하게
 * 테스트하기 위해 어차피 필요한 구조다.
 */
export type ScriptLine = {
  role: 'host' | 'counterpart'
  text: string
  /** STT가 붙여 줄 물음표. 실제 STT에서는 전사 결과에 포함된다. */
  questionMark?: boolean
  /** 앞 발화와의 간격(ms). 재생 속도 조절용. */
  gapMs?: number
}

export const DEMO_SCRIPT: ScriptLine[] = [
  { role: 'counterpart', text: '안녕하세요, 자료 잘 봤습니다.' },
  { role: 'host', text: '네 감사합니다. 궁금하신 점 편하게 말씀해 주세요.' },
  { role: 'counterpart', text: '저희는 금융권이라 클라우드 반출이 어려운데요.' },
  { role: 'counterpart', text: '셀프호스팅 환경에서도 SAML SSO를 설정할 수 있나요?', questionMark: true },
  { role: 'host', text: '네, 가능합니다.' },
  { role: 'counterpart', text: '그럼 온프레미스 배포도 되나요?', questionMark: true },
  { role: 'counterpart', text: '아 그리고 SOC 2 인증도 받으셨나요?', questionMark: true },
  { role: 'counterpart', text: 'Edge Functions는 셀프호스팅에서 언제 지원되나요?', questionMark: true },
  { role: 'host', text: '그건 확인해봐야 할 것 같은데요.' },
]

/** 채용 도메인 — 스키마 변경 없이 도메인이 교체되는지 확인용. */
export const RECRUITING_SCRIPT: ScriptLine[] = [
  { role: 'counterpart', text: '지원서 잘 접수됐는지 확인차 연락드렸습니다.' },
  { role: 'counterpart', text: '이 포지션 연봉 밴드가 어떻게 되나요?', questionMark: true },
  { role: 'counterpart', text: '최종 결과는 언제쯤 나오나요?', questionMark: true },
  { role: 'host', text: '확인해서 안내드리겠습니다.' },
]
