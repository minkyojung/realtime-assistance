// Package host 는 터미널 안에 앱들을 담는 껍데기다.
//
// 두 줄만 갖는다 — 입력창과 상태줄. 그 위는 전부 앱의 것이다.
// 파는 것은 화면이 아니라 조작법이다. docs/07-호스트-계약.md
package host

import (
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 입력창은 하나지만 하는 일이 둘이다. 무엇을 하는 중인지 화면이 말해야 한다.
//
//	기본       Ask AI — 자연어 요청. 목록을 건드리지 않는다
//	shift+tab  Search — 지금 보고 있는 것을 즉시 거른다 (ctrl+f 로도 들어간다)
//	/          Ask AI 에 `/` 로 시작하면 명령 — 로컬에서 바로 실행
//
// 모드는 화면을 바꾸지 않고 "친 글자의 뜻"을 바꾼다. 그래서 안 보이면
// 알아낼 방법이 없다. **입력창 자신이 말한다** — 커서 바로 앞의 글자와
// 테두리 색이 지금 어느 모드인지다. 아래에 줄을 따로 두지 않는 이유는,
// 모드를 말할 가장 좋은 자리가 글자를 치는 그 자리이기 때문이다.
//
// 창 제목. 터미널 탭에 뜬다.
const windowTitle = "npm run dev"

type inputMode int

const (
	modePrompt inputMode = iota
	modeSearch
)

type Model struct {
	apps    []app.App
	current int

	input    textarea.Model
	mode     inputMode
	showHelp bool
	notice   string

	// 팔레트·도움말에서 고른 줄.
	pick int

	// 홈에 있는가. 앱을 보고 있지 않다는 뜻이다.
	//
	// current 를 지우지 않는 이유는, 그것이 "마지막으로 본 앱"으로 계속
	// 쓸모가 있기 때문이다 — esc 로 홈에 와도 그 앱은 살아서 재생 중이고,
	// 상태줄이 그것을 계속 말한다. home.go
	home bool

	// 로그 — 호스트의 세 번째 자산. 입력의 짝이다.
	log        []logEntry
	logOpen    bool
	detailOpen bool
	routing    bool            // 라우터의 답을 기다리는 중
	routeSeq   int             // 취소된 요청의 늦은 답을 버리는 데 쓴다
	pending    map[string]bool // 답을 기다리는 앱
	spinner    spinner.Model

	w, h int
	send func(tea.Msg)
}

func New(apps ...app.App) Model {
	ta := textarea.New()
	ta.SetHeight(1)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	// 커서는 터미널의 실제 커서다. 한글 조합(IME)은 앱이 아니라 터미널이
	// 그 자리에 그리므로, 가짜 커서를 쓰면 조합 중인 글자가 엉뚱한 데
	// 뜨고 커서는 음절이 확정된 뒤에야 따라온다. View 가 위치를 보고한다.
	ta.SetVirtualCursor(false)
	styleInput(&ta)
	ta.Focus() // 입력창은 늘 활성이다. 타이핑이 언제나 먼저 온다.

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(style.ColBrand)

	m := Model{apps: apps, input: ta, spinner: sp, pending: map[string]bool{}, home: true}
	(&m).applyMode()
	return m
}

func (m Model) app() app.App { return m.apps[m.current] }

func (m Model) Init() tea.Cmd {
	// textarea.Blink 를 걸지 않는다. 실제 커서는 터미널이 깜빡인다.
	cmds := []tea.Cmd{m.spinner.Tick}
	for _, a := range m.apps {
		cmds = append(cmds, a.Init(m.push))
	}
	// 홈에서 시작하면 아직 아무 앱도 보고 있지 않다. 포커스는 앱에
	// 들어갈 때 준다 — 안 그러면 안 보는 동안 카메라 불이 켜진다.
	if m.home {
		return tea.Batch(cmds...)
	}
	first, cmd := m.apps[m.current].Update(app.FocusMsg{})
	m.apps[m.current] = first
	return tea.Batch(append(cmds, cmd)...)
}

// push 는 앱이 이벤트 루프 밖에서 메시지를 넣는 통로다.
// 폴링이 아니라 밀어 넣는 앱(채팅 등)에 필요하다.
func (m Model) push(msg tea.Msg) {
	if m.send != nil {
		m.send(msg)
	}
}

// SetSend 는 tea.Program 이 만들어진 뒤 주입한다.
func (m *Model) SetSend(f func(tea.Msg)) { m.send = f }

// LeaveHome 은 홈을 건너뛰고 곧장 앱 안에서 시작한다.
// 골든과 화면 테스트가 앱 화면을 보기 위해 쓴다.
func (m *Model) LeaveHome() { m.home = false }

// commanding — 입력이 `/` 로 시작하면 명령 모드다. 별도 상태를 두지 않는다.
func (m Model) commanding() bool {
	return m.mode == modePrompt && strings.HasPrefix(m.input.Value(), "/")
}

// overlaying — 본문 자리를 호스트가 잠시 빌려 쓰는 중인가.
func (m Model) overlaying() bool { return m.showHelp || m.commanding() }

// pickCount — ↑↓ 로 고를 것이 지금 화면에 몇 개 있는가.
//
// 팔레트와 도움말뿐이다. 홈에는 고를 목록이 없다(home.go) — 앱이 하나여서
// 목록을 걷어냈고, 그래서 홈의 ↑↓ 는 아무 데도 가지 않는다.
func (m Model) pickCount() int {
	if m.overlaying() {
		return m.overlayCount()
	}
	return 0
}

func (m Model) picking() bool { return m.pickCount() > 0 }

// 두 프롬프트는 폭이 같다(6글자 + 공백 둘). 모드를 바꿔도 글자가 시작하는
// 칸이 그대로여서 화면이 흔들리지 않는다.
const (
	promptAsk    = "Ask AI  "
	promptSearch = "Search  "
)

func (m *Model) applyMode() {
	styles := m.input.Styles()
	color := style.ColBrand // 프라이머리는 Ask AI 의 것이다
	if m.mode == modeSearch {
		m.input.Prompt = promptSearch
		m.input.Placeholder = "Filter what you are looking at"
		color = style.ColDim
	} else {
		m.input.Prompt = promptAsk
		m.input.Placeholder = "Ask for anything    /  commands     ?  help"
	}
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(color)
	styles.Blurred.Prompt = styles.Focused.Prompt
	m.input.SetStyles(styles)
	// textarea 는 프롬프트 폭을 빼서 글자 자리를 잡는다. 프롬프트를 바꾼
	// 뒤에 다시 불러야 한다.
	m.input.SetWidth(m.inputWidth())
}

// 입력창은 테두리 안에 들어간다. 양옆 선 두 칸과 안여백 두 칸을 뺀다.
func (m Model) inputWidth() int { return style.Max(style.ContentWidth(m.w)-4, 10) }

// 테두리 색도 모드를 말한다. 프라이머리(ColBrand)는 글자가 쓰고, 선은 한 단계
// 짙은 톤을 쓴다 — 브랜드 색이 화면에서 제일 큰 덩어리가 되면 안 된다.
func (m Model) inputBox(w int) string {
	color := style.ColBrandDeep
	if m.mode == modeSearch {
		color = style.ColRule
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		Width(w). // lipgloss 의 Width 는 테두리와 안여백을 포함한 전체 폭이다
		Render(m.input.View())
}

// setMode — 두 모드를 오간다.
//
// 들어갈 때도 나올 때도 입력을 비우고 필터를 다시 건다. 남겨 두면 친 글자가
// 다른 뜻으로 읽히고("조용한 거"가 검색어가 된다), 목록도 왜 걸러졌는지
// 설명되지 않은 채 남는다.
func (m *Model) setMode(mode inputMode) {
	m.mode = mode
	m.showHelp = false
	m.input.Reset()
	m.applyMode()
	m.apps[m.current] = m.app().Filter("")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(m.inputWidth())
		return m.forward(app.ResizeMsg{
			Width:  style.ContentWidth(m.w),
			Height: m.bodyHeight(),
		})

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case app.SayMsg:
		return m.handleSay(msg), nil

	case routedMsg:
		// 취소하고 다시 물어본 사이에 옛 답이 올 수 있다. 번호로 거른다.
		if msg.seq != m.routeSeq {
			return m, nil
		}
		return m.deliver(msg)

	case switchAppMsg:
		// 화면만 갈아끼운다. 다른 앱은 계속 살아 있다.
		if msg.index >= 0 && msg.index < len(m.apps) {
			// 나가는 앱에게 먼저 알린다. 장치를 잡고 있는 앱은 여기서 놓는다.
			// 홈에서 들어가는 길이면 나가는 앱이 없다.
			var blurCmd tea.Cmd
			if !m.home {
				var blur app.App
				blur, blurCmd = m.app().Update(app.BlurMsg{})
				m.apps[m.current] = blur
			}

			m.home = false
			m.current = msg.index
			m.mode = modePrompt
			m.showHelp = false
			(&m).applyMode()

			focus, focusCmd := m.app().Update(app.FocusMsg{})
			m.apps[m.current] = focus

			mm, sizeCmd := m.forward(app.ResizeMsg{
				Width:  style.ContentWidth(m.w),
				Height: m.bodyHeight(),
			})
			return mm, tea.Batch(blurCmd, focusCmd, sizeCmd)
		}
		return m, nil

	case showHelpMsg:
		m.showHelp, m.pick = true, 0
		return m, nil

	case showCostMsg:
		// 비용은 앱이 쓴 것이므로 앱의 상태줄에 이미 있다.
		// 여기서는 그것을 잠시 앞으로 끌어낸다.
		m.notice = m.app().Status()
		return m, nil

	case tea.KeyPressMsg:
		if handled, mm, cmd := m.handleKey(msg); handled {
			return mm, cmd
		}
		// 호스트가 안 쓰는 키는 입력창이 먼저 본다. 타이핑이 언제나 먼저다.
		before := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != before {
			m.pick = 0
			if m.mode == modeSearch {
				m.apps[m.current] = m.app().Filter(m.input.Value())
			}
			return m, cmd
		}
		// 홈에서는 넘길 앱이 없다. 보이지도 않는 앱이 방향키를 먹으면
		// 돌아갔을 때 엉뚱한 곳에 커서가 가 있다.
		if m.home {
			return m, cmd
		}
		// 입력창이 안 먹은 키만 앱에게 간다 (방향키·tab 등).
		next, appCmd := m.app().Update(msg)
		m.apps[m.current] = next
		return m, tea.Batch(cmd, appCmd)
	}
	return m.forward(msg)
}

// dispatch 는 문장을 어느 앱에게 줄지 정해 보낸다.
//
// 라우터를 부르지 않는 경우가 대부분이다 — 앱이 하나이거나, 사용자가
// @ 로 지정했으면 답이 이미 정해져 있다.
func (m Model) dispatch(prompt string) (tea.Model, tea.Cmd) {
	if name, rest := m.mention(prompt); name != "" {
		return m.ask([]string{name}, rest)
	}
	if len(m.apps) == 1 {
		return m.ask([]string{m.app().Name()}, prompt)
	}
	// 홈에서는 기댈 "지금 보는 앱"이 없다. 그것을 넘기면 애매한 문장이
	// 마지막에 본 앱으로 계속 쏠린다.
	current := ""
	if !m.home {
		current = m.app().Name()
	}
	m.routing = true
	m.routeSeq++
	return m, tea.Batch(
		cmdRoute(m.routeSeq, prompt, m.specs(), current),
		m.spinner.Tick,
	)
}

// deliver 는 라우터의 결과를 받아 앱들에게 넘긴다.
func (m Model) deliver(msg routedMsg) (tea.Model, tea.Cmd) {
	m.routing = false
	names := msg.apps
	if msg.err != nil || len(names) == 0 {
		// 홈에는 기댈 곳이 없다. 모르면 모른다고 말하고 홈에 머문다.
		// 여기서 아무 앱이나 열면 고르지도 않은 화면이 튀어나온다.
		if m.home {
			m.log = append(m.log, logEntry{
				who: "host", text: "어느 앱의 일인지 모르겠습니다", err: true,
			})
			return m, nil
		}
		// 앱 안에서는 지금 보고 있는 앱에게 준다. 틀려도 망하지 않는다 —
		// 그 앱이 못 하겠다고 로그에 남기고 끝이다.
		names = []string{m.app().Name()}
	}
	return m.ask(names, msg.prompt)
}

// ask 는 지목된 앱들에게 동시에 묻는다.
//
// 순서를 보장하지 않는다. "틀고 꺼줘"는 순서 의존이 없고, 순서가 필요한
// 요청("A 하고 나서 B")은 지금 범위 밖이다. 결과는 도착하는 대로 로그에 쌓인다.
func (m Model) ask(names []string, prompt string) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.spinner.Tick}
	enter := -1
	for _, name := range names {
		for i, a := range m.apps {
			if a.Name() != name {
				continue
			}
			if enter < 0 {
				enter = i
			}
			m.pending[name] = true
			next, cmd := a.Update(app.AskMsg{Prompt: prompt})
			m.apps[i] = next
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	// 홈에서 문장을 쳤고 앱이 지목됐으면 그 앱으로 들어간다. 목적지를
	// 고른 것과 같다 — 답이 그 화면에 나올 텐데 홈에 남아 있을 이유가 없다.
	//
	// 여럿이 지목되면 첫 번째로 간다. 나머지는 배경에서 답하고, 그 답은
	// 로그와 상태줄에 남는다 — 원래 그렇게 도는 구조다.
	if m.home && enter >= 0 {
		cmds = append(cmds, switchTo(enter)(""))
	}
	return m, tea.Batch(cmds...)
}

// forward 는 메시지를 지금 앱에게 넘긴다.
// 타입 단언이 없다 — App.Update 가 App 을 돌려주기 때문이다.
func (m Model) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.app().Update(msg)
	m.apps[m.current] = next
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (bool, tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return true, m, tea.Quit

	case "?":
		if m.input.Value() == "" {
			m.showHelp, m.pick = true, 0
			return true, m, nil
		}

	case "ctrl+o":
		// 가장 최근 응답이 무슨 일을 했는지 펼친다.
		m.detailOpen = !m.detailOpen
		return true, m, nil

	case "ctrl+j":
		// 앱의 화면을 한 칸 넘긴다. 앱 하나가 본문을 여러 가지로 나눠 쓸 때
		// (음악은 가사·대화·이력) 손으로 넘길 길이다.
		//
		// 원래는 로그를 펼치는 키였다. 앱이 자기 대화를 본문에 펼쳐 보여주게
		// 되면서 그 자리가 비었고, "무엇을 보여줘"라는 뜻은 그대로 남았다.
		// 호스트 로그의 상세는 ctrl+o 가 계속 맡는다.
		//
		// 홈에는 넘길 화면이 없다. 안 보이는 앱의 무대를 넘기면, 돌아갔을 때
		// 고르지도 않은 화면이 서 있다.
		if m.home {
			return true, m, nil
		}
		mm, cmd := m.forward(app.CycleMsg{})
		return true, mm, cmd

	case "ctrl+f":
		(&m).setMode(modeSearch)
		return true, m, nil

	case "shift+tab":
		// 모드를 바꾼다. 둘뿐이므로 한 키로 왕복한다.
		//
		// 앱에게 넘기지 않는다 — 앱의 tab(다음 섹션)에는 짝이 없어졌고,
		// 먼 섹션에는 `/` 로 곧장 간다.
		if m.home {
			m.notice = "Modes work inside the app"
			return true, m, nil
		}
		if m.mode == modeSearch {
			(&m).setMode(modePrompt)
		} else {
			(&m).setMode(modeSearch)
		}
		return true, m, nil

	case "esc":
		// 한 단계씩 물러난다 — 요청 → 도움말 → 검색 → 입력 비우기 → 홈 → 종료.
		//
		// 방금 시킨 일이 아직 돌고 있으면 그것이 첫 칸이다. 선곡이 7초라
		// 그 사이에 잘못 물어본 것을 알아채는데, 지금까지는 기다리는 수밖에
		// 없었다. ctrl+c 는 손대지 않는다 — 그것은 터미널의 탈출구다.
		if m.routing || len(m.pending) > 0 {
			return true, m.cancelPending(), nil
		}
		if m.showHelp {
			m.showHelp = false
			return true, m, nil
		}
		if m.detailOpen {
			m.detailOpen = false
			return true, m, nil
		}
		if m.logOpen {
			m.logOpen = false
			return true, m, nil
		}
		m.notice = ""
		if m.mode == modeSearch {
			(&m).setMode(modePrompt)
			return true, m, nil
		}
		if m.input.Value() != "" {
			m.input.Reset()
			return true, m, nil
		}
		if !m.home {
			// 앱이 자기 안에 물러날 단계를 갖고 있으면 그것이 먼저다.
			// 파고든 목록에서 esc 를 눌렀는데 앱 밖으로 튕겨 나가면 안 된다.
			if next, ok := m.app().Back(); ok {
				m.apps[m.current] = next
				return true, m, nil
			}
			// 앱에서 물러나면 홈이다. 앱은 계속 살아 있고 화면만 떠난다.
			// current 는 그대로 두므로 다시 enter 면 방금 있던 곳이다.
			blur, blurCmd := m.app().Update(app.BlurMsg{})
			m.apps[m.current] = blur
			m.home = true
			return true, m, blurCmd
		}
		return true, m, tea.Quit

	case "up", "ctrl+p":
		if m.picking() {
			m.pick = style.Max(m.pick-1, 0)
			return true, m, nil
		}
	case "down", "ctrl+n":
		if m.picking() {
			m.pick = style.Min(m.pick+1, m.pickCount()-1)
			return true, m, nil
		}

	case "enter":
		mm, cmd := m.handleEnter()
		return true, mm, cmd
	}
	return false, m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	if m.commanding() {
		return m.runCommand()
	}
	// 홈에서 빈 입력에 enter 면 앱으로 들어간다. 친 것이 있으면 그것이
	// 먼저다 — 홈에서도 문장을 던질 수 있어야 한다.
	//
	// 갈 곳은 마지막으로 본 앱이다. 앱이 하나면 언제나 그 앱이고,
	// 여럿이던 시절에도 esc 로 나온 그 자리로 돌아가는 것이 맞았다.
	if m.home && strings.TrimSpace(m.input.Value()) == "" {
		return m, switchTo(m.current)("")
	}
	// 검색 중에는 고른 것을 앱이 처리한다. 프롬프트일 때만 요청으로 보낸다.
	if m.mode == modePrompt {
		if prompt := strings.TrimSpace(m.input.Value()); prompt != "" {
			m.input.Reset()
			m.notice = ""
			m.showHelp = false
			// 한 말은 곧바로 로그에 남는다. 답을 기다리는 동안에도 보인다.
			m.log = append(m.log, logEntry{text: prompt})
			return m.dispatch(prompt)
		}
	}
	return m.forward(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// 프레임 여백. 커서 위치를 보고할 때 이만큼 밀어야 한다.
const framePad = 1

// padTo — 본문을 받은 높이만큼 빈 줄로 채운다.
//
// 앱이 준 높이를 다 쓰지 않는 일이 흔하다. 음악의 목록은 열네 줄에서
// 멈추므로(maxListRows) 창이 길면 그만큼 남는다. 그대로 두면 입력창이
// 화면 한가운데에 떠서, 창 높이에 따라 손이 가는 자리가 달라진다.
//
// **입력창과 상태줄은 언제나 맨 아래다.** 홈이 이미 그렇게 그리고 있었고
// (home.go), 그것을 호스트의 규칙으로 올린다. 앱이 높이를 넘겨 그리면
// 자르지 않는다 — 잘라서 감추느니 밀려나는 편이 눈에 띈다.
func padTo(body string, h int) string {
	if n := strings.Count(body, "\n") + 1; n < h {
		return body + strings.Repeat("\n", h-n)
	}
	return body
}

func (m Model) bodyHeight() int {
	// 테두리 친 입력창3 + 상태줄1 + 위아래 여백2
	w := style.ContentWidth(m.w)
	h := m.h - 6 - len(m.overlayRows(w)) - len(m.logRows(w))
	return style.Max(h, 3)
}

func (m Model) View() tea.View {
	if m.w == 0 || m.h == 0 {
		return tea.NewView("")
	}
	w := style.ContentWidth(m.w)
	bodyH := m.bodyHeight()

	var b strings.Builder
	// 본문은 무엇을 하든 그대로다. 팔레트는 입력창 아래에 붙는다.
	body := m.viewHome(w, bodyH)
	if !m.home {
		body = m.app().View(w, bodyH)
	}
	b.WriteString(padTo(body, bodyH))
	// 로그는 입력창 바로 위, 팔레트는 바로 아래. 둘 다 본문을 밀어내지 않는다.
	for _, r := range m.logRows(w) {
		b.WriteString("\n")
		b.WriteString(r)
	}
	b.WriteString("\n")
	// 입력창이 몇째 줄에 놓이는지는 지금 센다. 본문 높이로 계산하면
	// 앱이 받은 높이를 다 안 쓸 때(musicapp 의 maxListRows) 어긋난다.
	inputRow := strings.Count(b.String(), "\n")
	b.WriteString(m.inputBox(w))
	for _, r := range m.overlayRows(w) {
		b.WriteString("\n")
		b.WriteString(r)
	}
	b.WriteString("\n")
	b.WriteString(m.viewStatus(w))

	v := tea.NewView(lipgloss.NewStyle().Padding(framePad, framePad).Render(b.String()))
	v.AltScreen = true
	// 실제 커서를 입력창의 글자 자리에 둔다. 터미널이 한글 조합을 그리는
	// 자리가 여기다 — 안 알려주면 조합 중인 글자가 엉뚱한 데 뜬다.
	if c := m.input.Cursor(); c != nil {
		// 테두리 왼쪽 선과 안여백 두 칸, 윗선 한 줄만큼 더 민다.
		c.Position.X += framePad + 2
		c.Position.Y += framePad + inputRow + 1
		v.Cursor = c
	}
	// 어깨너머로 제일 먼저 보이는 자리다. 스플래시보다 노출이 크다.
	// 이 껍데기의 컨셉대로라면 여기가 가장 정직하지 않아야 한다.
	v.WindowTitle = windowTitle
	return v
}

// 상태줄 — 지금 앱이 말하는 것과, 배경 앱들이 말하는 것을 모은다.
func (m Model) viewStatus(w int) string {
	// 왼쪽은 언제나 "앞에 나온 것"이다. 홈에서도 마찬가지다 — 화면은
	// 떠나 있어도 그 앱은 살아서 재생 중이고, 상태줄이 그것을 계속 말한다.
	front := m.current
	left := m.apps[front].Status()
	if m.notice != "" {
		left = style.BrandSoft.Render("· ") + style.Dim.Render(style.Truncate(m.notice, w-2))
	}

	// 배경 앱은 이름과 배지만 내놓는다. 맥 메뉴바와 같다.
	var bg []string
	for i, a := range m.apps {
		if i == front {
			continue
		}
		s := a.Name()
		if n := a.Badge(); n > 0 {
			s += " " + style.Tokens(n)
		}
		bg = append(bg, s)
	}
	if len(bg) == 0 {
		return style.Truncate(left, w)
	}
	return style.Row(left, style.Faint.Render(strings.Join(bg, " · ")), w)
}

// textarea 기본 스타일은 배경이 검게 깔린다. 나머지 화면과 어긋나므로
// 배경을 비우고 전경색만 팔레트에 맞춘다.
func styleInput(ta *textarea.Model) {
	styles := ta.Styles()
	for _, st := range []*textarea.StyleState{&styles.Focused, &styles.Blurred} {
		st.Base = lipgloss.NewStyle()
		st.CursorLine = lipgloss.NewStyle()
		st.EndOfBuffer = lipgloss.NewStyle()
		st.Text = lipgloss.NewStyle().Foreground(style.ColFg)
		st.Prompt = lipgloss.NewStyle().Foreground(style.ColBrand)
		st.Placeholder = lipgloss.NewStyle().Foreground(style.ColFaint)
	}
	styles.Cursor.Color = style.ColBrand
	ta.SetStyles(styles)
}
