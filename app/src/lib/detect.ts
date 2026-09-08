/**
 * 질문 감지 — 실시간 경로 1차 필터.
 *
 * 주 신호는 STT가 붙여 주는 물음표다.
 * 스모크 테스트(docs/07)에서 한국어 질문/비질문 8건이 부호로 정확히 갈렸고,
 * 특히 정규식이 오탐하기 쉬운 두 함정을 STT가 이미 처리했다.
 *   "그건 확인해봐야 할 것 같은데요"  -> 마침표
 *   "언제 한번 자리 마련하죠"         -> 마침표
 *
 * 어미 규칙은 물음표가 없을 때만 쓰는 보조 신호다.
 * 재현율(놓치지 않기)을 정밀도보다 우선한다. 놓치면 카드가 아예 안 뜨지만,
 * 잘못 통과시켜도 2차 모델이 걸러 주기 때문이다.
 */

/** 물음표 없이도 질문으로 볼 종결 어미. */
const INTERROGATIVE_ENDINGS = [
  /(나요|가요|까요|은가|는가|런가|len가)\s*[.!]?$/,
  /(인지|는지|을지|ㄹ지)\s*[.!]?$/,
  /(맞나|되나|있나|없나|어때|어떤가)\s*[.!]?$/,
]

/** 의문사가 있으면 질문일 가능성이 높다. */
const WH_WORDS = /(얼마|언제|어디|어떻게|어떤|무엇|뭐|왜|누가|누구|몇)/

/** 어미가 의문형처럼 보여도 질문이 아닌 표현. */
const NOT_QUESTION = [
  /같은데요\s*[.!]?$/,      // "확인해봐야 할 것 같은데요"
  /하죠\s*[.!]?$/,          // "언제 한번 자리 마련하죠"
  /하시죠\s*[.!]?$/,
  /(했|있|없|같)어요\s*[.!]?$/,
  /드릴게요\s*[.!]?$/,
]

export type Detection = {
  isQuestion: boolean
  /** 어떤 신호로 걸렸는지. DB의 detected_by 에 대응한다. */
  signal: 'question_mark' | 'ending' | 'wh_word' | 'none'
  confident: boolean
}

export function detectQuestion(text: string, hasQuestionMark?: boolean): Detection {
  const t = text.trim()

  // 1) 물음표 — 주 신호. STT 부호 예측 또는 텍스트에 포함된 것.
  if (hasQuestionMark || /[?？]\s*$/.test(t)) {
    return { isQuestion: true, signal: 'question_mark', confident: true }
  }

  // 2) 명백한 비질문 표현은 먼저 제외한다.
  if (NOT_QUESTION.some((re) => re.test(t))) {
    return { isQuestion: false, signal: 'none', confident: true }
  }

  // 3) 의문형 어미 — 보조 신호.
  if (INTERROGATIVE_ENDINGS.some((re) => re.test(t))) {
    return { isQuestion: true, signal: 'ending', confident: false }
  }

  // 4) 의문사만 있는 경우. 확신은 낮지만 놓치지 않는 쪽을 택한다.
  if (WH_WORDS.test(t) && t.length < 60) {
    return { isQuestion: true, signal: 'wh_word', confident: false }
  }

  return { isQuestion: false, signal: 'none', confident: true }
}
