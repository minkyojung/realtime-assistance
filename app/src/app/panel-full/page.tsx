import { PanelView } from '@/components/panel/panel-view'

/**
 * 화면 1 전체 구현 (대화형 레이아웃).
 *
 * /panel 은 병렬 작업 중이라 충돌을 피해 별도 경로로 둔다.
 * 두 작업이 합쳐지면 /panel 로 옮기고 이 경로는 삭제한다.
 */
export default function PanelFullPage() {
  return <PanelView />
}
