package sequence

import (
	"fmt"
	"math"
	"strconv"

	"github.com/itda-work/itda-doc/diagram"
)

// 그리기. 쓰는 요소·속성은 흐름도·간트·타임라인과 같은 허용 목록이다(svg·rect·path·line·circle·text·tspan·title —
// mdoc/security_test.go 가 고정한다). 화살촉은 marker 가 아니라 path(흐름도와 같다), 사람 모양(actor) 은 circle+line+path.
// 색은 클래스 → 토큰(base.css), 새 색 상수·새 토큰 없음. 클래스는 모두 sq- 접두사(앱 CSS 충돌 — sequence_test.go
// TestClassCollisions):
//
//	sq-par·sq-par-t              참여자 상자·글자(actor 선언은 사람 모양 sq-fig·sq-fig-h 를 얹는다)
//	sq-box·sq-box-t              참여자 무리(box) 띠·글자
//	sq-life                      생명선(점선)
//	sq-rect                      rect 띠(옅게)
//	sq-frame·sq-lab·sq-lab-t     묶음(loop·alt·…) 테두리·머리 칸·칸 글자
//	sq-div·sq-sec-t              구획 점선(else·and·option)·구획 글자
//	sq-msg(+sq-dt)·sq-head·sq-cross   메시지 선(점선)·채운 화살촉·가위표
//	sq-tbg·sq-msg-t              메시지 글 바탕(--paper 계열 — 생명선이 글자를 가로지르지 않게)·글자
//	sq-num·sq-num-t              autonumber 동그라미·숫자
//	sq-note·sq-note-t            메모 상자·글자
//	sq-act                       활성 막대
//	sq-title                     그림 안 제목
//
// 배치 상수(viewBox 단위, 글자 14 기준). 참여자 상자 폭은 고정(원본 conf.width=150)이고 긴 이름은 줄을 바꿔 상자
// 높이가 는다(원본은 폭이 늘어난다 — 이 판의 축소, 안내서 10장). 참여자 사이 간격은 그 사이를 지나는 메시지·메모 글
// 폭으로 넓힌다(원본 calculateActorMargins — 이웃 참여자 사이의 것만 세는 원본 계산 그대로).
const (
	padX      = 10.0  // 그림 좌우 여백
	padT      = 10.0  // 그림 위 여백
	actorW    = 150.0 // 참여자 상자 폭(고정)
	actorPad  = 10.0  // 상자 안 글자 좌우 여백(합)
	actorGap  = 50.0  // 참여자 사이 최소 간격(원본 actorMargin)
	rowPad    = 12.0  // 참여자 상자 안 글자 위아래 여백(합)
	glyphH    = 34.0  // 사람 모양 높이(actor 선언)
	glyphGap  = 6.0   // 사람 모양과 이름 사이
	boxGapX   = 12.0  // box 사이 추가 간격
	boxPad    = 8.0   // box 띠 안쪽 여백
	boxStripH = 26.0  // box 이름 줄 높이
	msgWrap   = 220.0 // 메시지 글 줄바꿈 폭
	msgMin    = 35.0  // 메시지 행 최소 높이(원본 messageMargin)
	noteWrap  = 200.0 // 메모 글 줄바꿈 폭
	notePad   = 10.0  // 메모 상자 안 여백(원본 noteMargin)
	noteMin   = 50.0  // 메모 상자 최소 폭(원본 conf.width)
	loopPad   = 10.0  // 묶음 여백(원본 boxMargin — 중첩마다 안쪽으로 깊이만큼)
	labelBoxW = 50.0  // 묶음 머리 칸 폭(원본 labelBoxWidth)
	labelBoxH = 20.0  // 묶음 머리 칸 높이(원본 labelBoxHeight)
	labelPad  = 6.0   // 머리 칸과 조건 글 사이
	condMaxW  = 240.0 // 조건 글 줄바꿈 폭(이 안에서 한 줄에 들어가면 묶음이 그 폭까지 넓어진다)
	actW      = 10.0  // 활성 막대 폭(원본 activationWidth)
	headL     = 11.0  // 채운 화살촉(->>) 길이 — 원본 arrowhead marker 비율(11×10)
	headW     = 5.0   // 채운 화살촉 반 너비
	crossS    = 4.0   // 가위표 반 크기
	selfDrop  = 24.0  // 자기 메시지의 아래 내려감
	numR      = 6.0   // autonumber 동그라미 반지름(원본 SEQUENCE_NUMBER_RADIUS)
	numSz     = 11.0  // autonumber 숫자 글자 크기(3 자리까지 — 원본 fontSize 사다리)
	lineH     = 18.0  // diagram.LineH — 줄 간격
	titleSz   = 15.0  // 그림 안 제목 글자 크기
)

// msgRow·noteRow·frameDraw·actDraw 는 걷는 동안 정한 그림 조각이다(걷기가 끝나면 전체 폭을 알아 좌표를 옮겨 쓴다).
type msgRow struct {
	from, to int
	ar       arrowKind
	lines    []string
	textW    float64
	startx   float64 // 화살표 시작(x 좌표는 center 공간 — 아래서 옮긴다)
	self     bool
	selfDX   float64
	num      string  // autonumber 숫자(끄면 "")
	numSz    float64 // 숫자 글자 크기(자릿수 사다리 — 원본 fontSize)
	line     int
	y0, y1   float64 // 글 위·아래
	arrowY   float64
	overX    float64 // 화살표 먼 쪽 끝(자기 메시지의 오른쪽)
}

type noteRow struct {
	lines []string
	x, y  float64
	w, h  float64
}

type sectionDraw struct {
	y     float64
	title string
	lines []string // 구획 글의 줄(대괄호를 감싼 채)
}

type frameDraw struct {
	kind     int
	name     string
	title    string
	lines    []string // 조건 글의 줄(대괄호를 감싼 채) — 비었으면 머리 칸만
	titleH   float64  // 머리 줄 높이(머리 칸·조건 글 줄 수에 맞춰 늘어난다)
	x1, x2   float64
	top, bot float64
	sections []sectionDraw
}

type actDraw struct {
	actor int
	level int
	x, y  float64
	h     float64
}

type layout struct {
	c          *chart
	n          int
	center     []float64
	actorLines [][]string
	actorH     float64 // 참여자 행 높이(통일)
	actorTop   float64
	mirrorTop  float64
	msgs       []*msgRow
	notes      []*noteRow
	frames     []*frameDraw
	acts       []*actDraw
	open       map[int][]*actDraw // 참여자별 열린 활성(만들어진 차례)
	boxLines   map[*box][]string  // box 이름의 줄(띠 폭에서 감는다 — 리뷰 1회차 [4])
	boxStrip   map[*box]float64   // box 이름 칸 높이
	stack      []*frameDraw       // 열린 묶음
	minX, maxX float64
	pos        float64
	seqNum     float64
	seqStep    float64
	seqVisible bool
}

func drawSVG(c *chart) (string, []Issue, error) {
	l, err := newLayout(c)
	if err != nil {
		return "", nil, err
	}
	return diagram.AriaRefs(l.emit()), nil, nil // aria-label 의 탭·줄바꿈은 문자 참조로(공통 — state 와 같다)
}

// newLayout 는 그림을 걷어 layout 을 만든다(그리기와 시험이 함께 쓴다 — 시험은 소유 관계(활성의 참여자·조건 글의
// 묶음)를 SVG 좌표와 구조체를 견줘 본다).
func newLayout(c *chart) (*layout, error) {
	l := &layout{c: c, n: len(c.Actors), open: map[int][]*actDraw{}, minX: math.MaxFloat64, maxX: -math.MaxFloat64, seqNum: 1, seqStep: 1}
	if err := l.place(); err != nil {
		return nil, err
	}
	return l, nil
}

// idx 는 참여자 ID 의 차례다.
func (l *layout) idx(id string) int {
	for i, a := range l.c.Actors {
		if a.ID == id {
			return i
		}
	}
	return 0 // 파서가 참조를 만들어 두었으니 오지 않는다
}

// gapNeed 는 이웃 참여자 사이(i 와 i+1)에 필요한 중심 간격이다(원본 getMaxMessageWidthPerActor·calculateActorMargins:
// 이웃 사이의 메시지·메모 글 폭만 센다 — 건너뛴 메시지는 간격을 넓히지 않는다).
func (l *layout) place() error {
	n := l.n
	need := make([]float64, n-1)
	for i := range need {
		need[i] = actorGap + actorW // 최소: 간격 + 상자 절반 둘(중심 간격)
	}
	add := func(gap int, w float64) {
		if gap >= 0 && gap < len(need) && w > need[gap] {
			need[gap] = w
		}
	}
	msgW := func(lines []string) float64 { return diagram.LinesW(lines) + 2*actorPad }
	for _, ev := range l.c.Events {
		switch ev.kind {
		case evMessage:
			f, t := l.idx(ev.from), l.idx(ev.to)
			lines := wrapMsg(ev.text)
			w := msgW(lines) + actorGap
			switch {
			case f == t: // 자기 메시지 — 양옆 간격에 절반씩
				add(f-1, w/2+actorGap)
				add(f, w/2+actorGap)
			case abs(f-t) == 1: // 이웃 사이 메시지만 간격을 넓힌다(원본과 같다)
				add(min(f, t), w)
			}
		case evNote:
			a := l.idx(ev.from)
			lines := diagram.WrapLabel(ev.text, noteWrap)
			w := msgW(lines)
			switch ev.placement {
			case rightOf:
				if a+1 < n {
					add(a, w+actorGap)
				}
			case leftOf:
				if a-1 >= 0 {
					add(a-1, w+actorGap)
				}
			case over:
				if ev.over2 == "" {
					if a-1 >= 0 {
						add(a-1, w/2+actorGap)
					}
					if a+1 < n {
						add(a, w/2+actorGap)
					}
				} else {
					b := l.idx(ev.over2)
					// 원본: 받는이(B) 의 왼쪽 간격과 보낸이(A) 의 오른쪽 간격에 절반씩(이웃이 아니어도 그 간격만)
					if b-1 >= 0 {
						add(b-1, w/2+actorGap)
					}
					if a+1 < n {
						add(a, w/2+actorGap)
					}
				}
			}
		}
	}
	// 참여자 중심 x — box 경계마다 간격을 더한다(원본이 box 사이에 boxMargin+box.margin 을 더하는 것과 같은 뜻).
	l.center = make([]float64, n)
	l.center[0] = actorW / 2
	for i := 1; i < n; i++ {
		g := need[i-1]
		if l.c.Actors[i-1].Box != l.c.Actors[i].Box {
			g += 2 * boxGapX
		}
		l.center[i] = l.center[i-1] + g
	}
	// 참여자 이름 줄·행 높이(행은 통일 — 원본이 행의 가장 큰 상자에 맞추는 것과 같다)
	l.actorLines = make([][]string, n)
	for i, a := range l.c.Actors {
		l.actorLines[i] = diagram.WrapLabel(a.Name, actorW-actorPad)
		h := float64(len(l.actorLines[i]))*lineH + rowPad
		if a.Actor {
			h += glyphH + glyphGap
		}
		l.actorH = math.Max(l.actorH, h)
	}
	l.actorH = math.Max(l.actorH, 36)

	// 세로 걷기
	top := padT
	if l.c.Title != "" {
		top += titleSz + 8
	}
	// box 이름 줄 — 띠 폭(참여자 범위 ± 여백) 안에서 감는다(리뷰 1회차 [4]: 긴 이름이 viewBox 밖으로 나가던 것).
	// 이름 줄 수만큼 띠의 이름 칸이 높아지고, 참여자 행은 가장 높은 띠만큼 아래로 내려간다.
	l.boxLines = map[*box][]string{}
	l.boxStrip = map[*box]float64{}
	maxStrip := 0.0
	for _, b := range l.c.Boxes {
		if b.Count == 0 || b.Name == "" {
			continue
		}
		left, right := l.boxRange(b)
		w := math.Max(right-left-2*boxPad, 100)
		lines := diagram.WrapLabel(b.Name, w)
		l.boxLines[b] = lines
		strip := float64(len(lines))*lineH + 2*boxPad
		l.boxStrip[b] = strip
		maxStrip = math.Max(maxStrip, strip)
	}
	if maxStrip == 0 {
		maxStrip = boxStripH // 이름 있는 box 가 없어도 빈 칸 높이는 유지
	}
	namedBox := false
	for _, b := range l.c.Boxes {
		if b.Count > 0 && b.Name != "" {
			namedBox = true
		}
	}
	l.actorTop = top
	if namedBox {
		l.actorTop += maxStrip
	}
	l.pos = l.actorTop + l.actorH + 10
	for _, ev := range l.c.Events {
		switch ev.kind {
		case evMessage:
			if err := l.walkMessage(ev); err != nil {
				return err
			}
		case evNote:
			l.walkNote(ev)
		case evActiveStart:
			l.walkActiveStart(ev)
		case evActiveEnd:
			l.walkActiveEnd(ev)
		case evAutonumber:
			// 원본(원Renderer 1270·1345~1380): 시작·단계는 value || previous — 0 도 「없음」으로 세던 값을 쓰고,
			// 표시와 카운터 증가는 분리다(off 구간·autonumber 앞의 메시지도 센다). 리뷰 1회차 [2].
			if ev.numOff {
				l.seqVisible = false
			} else {
				l.seqVisible = true
				if ev.haveStart && ev.numStart != 0 {
					l.seqNum = ev.numStart
				}
				if ev.haveStep && ev.numStep != 0 {
					l.seqStep = ev.numStep
				}
			}
		case evBlockStart:
			l.walkBlockStart(ev)
		case evBlockElse:
			l.walkSection(ev)
		case evBlockEnd:
			l.walkBlockEnd()
		}
	}
	// 닫히지 않은 활성은 막대를 그리지 않는다(원본도 ACTIVE_END 에서만 그린다).
	l.mirrorTop = l.pos + 20
	return nil
}

// actBounds 는 참여자의 활성 가장자리다(원본 activationBounds — 열린 활성의 바깥쪽). 없으면 중심 ±1.
func (l *layout) actBounds(i int) (float64, float64) {
	c := l.center[i]
	acts := l.open[i]
	if len(acts) == 0 {
		return c - 1, c + 1
	}
	left, right := math.MaxFloat64, -math.MaxFloat64
	for _, a := range acts {
		left = math.Min(left, a.x)
		right = math.Max(right, a.x+actW)
	}
	return left, right
}

// insert 는 그림 조각의 범위를 전체와 열린 묶음 모두에 넣는다 — 묶음은 깊이만큼 안쪽 여백을 더 받는다
// (원본 updateBounds: 겉의 묶음이 n*boxMargin, 안이 1*boxMargin — 중첩마다 여백이 늘어난다).
func (l *layout) insert(x1, y1, x2, y2 float64) {
	l.minX = math.Min(l.minX, x1)
	l.maxX = math.Max(l.maxX, x2)
	d := len(l.stack)
	for k, f := range l.stack {
		n := float64(d - k)
		f.x1 = math.Min(f.x1, x1-n*loopPad)
		f.x2 = math.Max(f.x2, x2+n*loopPad)
	}
	_ = y1
	_ = y2
}

// walkMessage 는 메시지 하나의 줄을 정한다. 번호는 표시와 상관없이 모든 메시지에서 카운터를 올리고(원본과 같다 —
// off 구간·autonumber 앞의 메시지도 센다), 표시되는 수가 6 자리를 넘으면 그림을 그만둔다(원문 + 경고 — 리뷰 1회차 [2]·[8]).
func (l *layout) walkMessage(ev event) error {
	m := &msgRow{from: l.idx(ev.from), to: l.idx(ev.to), ar: ev.ar, lines: wrapMsg(ev.text), line: ev.line}
	m.textW = diagram.LinesW(m.lines)
	fl, fr := l.actBounds(m.from)
	tl, tr := l.actBounds(m.to)
	right := fl <= tl
	if m.from == m.to {
		m.self = true
		m.startx = fr
		m.selfDX = math.Max(actorW/2, m.textW/2)
		m.overX = m.startx + m.selfDX
	} else {
		if right {
			m.startx, m.overX = fr, tl
		} else {
			m.startx, m.overX = fl, tr
		}
	}
	m.y0 = l.pos + 6
	m.y1 = m.y0 + float64(len(m.lines))*lineH
	if m.textW == 0 {
		m.y1 = m.y0 // 빈 글 — 줄 없음
	}
	m.arrowY = m.y1 + 8
	if l.seqVisible {
		m.num = fmtNum(l.seqNum)
		m.numSz = numSz     // 자릿수에 따라 글자 크기(원본 fontSize 사다리 — 4자리>9, 6자리>7)
		if len(m.num) > 5 { // 7 자리부터는 파서 상한·그림 상한이 막는다(안 옴)
			m.numSz = 7
		} else if len(m.num) > 3 {
			m.numSz = 9
		}
		if len(m.num) > MaxNumDigits {
			return &Error{Line: ev.line, Msg: fmt.Sprintf("autonumber 번호가 %d 자리를 넘음 — 번호는 %d 자리까지 그린다", MaxNumDigits, MaxNumDigits)}
		}
	}
	l.seqNum = math.Round((l.seqNum+l.seqStep)*100) / 100 // 증가는 표시와 분리(원본과 같다)
	l.msgs = append(l.msgs, m)
	// 범위 — 글(중심 맞춤)·선·동그라미·숫자까지
	cx := (m.startx + m.overX) / 2
	tl2, tr2 := cx-m.textW/2, cx+m.textW/2
	if m.self {
		tl2, tr2 = m.startx-m.textW/2, m.overX
	}
	xl, xr := math.Min(tl2, math.Min(m.startx, m.overX)), math.Max(tr2, math.Max(m.startx, m.overX))
	if m.num != "" { // 동그라미·숫자는 보낸 쪽 선 끝 바깥에 붙는다(숫자 폭도 범위에 — 리뷰 1회차 [8])
		edge := m.startx
		if m.self || m.startx <= m.overX {
			edge -= 2*numR + 2
		} else {
			edge += 2*numR + 2
		}
		nw := diagram.TextW(m.num, m.numSz)
		xl = math.Min(xl, edge-nw/2)
		xr = math.Max(xr, edge+nw/2)
	}
	by := m.arrowY
	if m.self {
		by = m.arrowY + selfDrop
	}
	l.insert(xl, m.y0, xr, by)
	rowH := math.Max(msgMin, m.y1-m.y0+8+8)
	if m.self {
		rowH = math.Max(rowH, m.y1-m.y0+8+selfDrop+8)
	}
	l.pos += rowH
	return nil
}

// walkNote 는 메모 하나의 상자를 정한다(원본 buildNoteModel 의 자리 계산 — 생명선에서 actorMargin/2 만큼 비킨다).
func (l *layout) walkNote(ev event) {
	lines := diagram.WrapLabel(ev.text, noteWrap)
	w := math.Max(noteMin, diagram.LinesW(lines)+2*notePad)
	a := l.idx(ev.from)
	var x float64
	switch ev.placement {
	case leftOf:
		x = l.center[a] - actorGap/2 - w
	case rightOf:
		x = l.center[a] + actorGap/2
	case over:
		if ev.over2 == "" {
			x = l.center[a] - w/2
		} else {
			b := l.idx(ev.over2)
			w = math.Max(w, math.Abs(l.center[a]-l.center[b])+actorGap)
			x = math.Min(l.center[a], l.center[b]) - actorGap/2
		}
	}
	l.pos += loopPad
	nt := &noteRow{lines: lines, x: x, y: l.pos, w: w, h: float64(len(lines))*lineH + 2*notePad}
	l.pos += nt.h + loopPad
	l.notes = append(l.notes, nt)
	l.insert(x, nt.y, x+w, nt.y+nt.h)
}

// walkActiveStart 는 활성 막대를 연다 — 겹친 활성은 활성 폭의 절반씩 옆으로 민다(원본 newActivation).
func (l *layout) walkActiveStart(ev event) {
	i := l.idx(ev.from)
	a := &actDraw{actor: i, level: len(l.open[i]) + 1, x: l.center[i] + float64(len(l.open[i]))*actW/2, y: l.pos + 2, h: -1}
	l.open[i] = append(l.open[i], a)
	l.insert(a.x, a.y, a.x+actW, a.y)
}

// walkActiveEnd 는 가장 최근의 활성을 닫아 막대로 만든다(원본 endActivation — lastIndexOf). 막대가 너무 짧으면
// 위를 내려 원본처럼 최소 높이를 만든다.
func (l *layout) walkActiveEnd(ev event) {
	i := l.idx(ev.from)
	acts := l.open[i]
	if len(acts) == 0 {
		return // 파서가 짝을 검사했다 — 오지 않는다
	}
	a := acts[len(acts)-1]
	l.open[i] = acts[:len(acts)-1]
	if a.y+18 > l.pos {
		a.y = l.pos - 6
		l.pos += 12
	}
	a.h = l.pos - a.y
	l.acts = append(l.acts, a)
	l.insert(a.x, a.y, a.x+actW, a.y+a.h)
}

// walkBlockStart 는 묶음을 연다. 조건 글은 원본처럼 대괄호를 감싸고(원본 adjustLoopHeightForWrap 의
// wrapLabel(`[${msg.message}]`)), 줄 수에 맞춰 머리 줄 높이가 늘어난다(원본 heightAdjust). rect 는 칸·글이 없다.
func (l *layout) walkBlockStart(ev event) {
	l.pos += loopPad
	f := &frameDraw{kind: ev.block, x1: math.MaxFloat64, x2: -math.MaxFloat64, top: l.pos, title: ev.text}
	if ev.block != bRect {
		f.name = blockName[ev.block]
		f.lines = condLines(ev.text)
		f.titleH = stripH(len(f.lines))
		l.pos += f.titleH
	}
	l.frames = append(l.frames, f)
	l.stack = append(l.stack, f)
}

// walkSection 은 else·and·option 구획을 연다 — 구획 글도 대괄호·줄 수에 맞춘 높이(원본과 같은 길).
func (l *layout) walkSection(ev event) {
	l.pos += loopPad + 4
	f := l.stack[len(l.stack)-1]
	sec := sectionDraw{y: l.pos, title: ev.text, lines: condLines(ev.text)}
	f.sections = append(f.sections, sec)
	l.pos += stripH(len(sec.lines)) - 4
}

// walkBlockEnd 는 묶음을 닫는다. 내용이 없는 빈 묶음은 참여자 전체 범위에 그린다(원본은 좌표가 없어 그리지 못 한다).
// 묶음 폭은 「머리 칸 + 조건 글 + 여백」 이상으로 넓힌다 — 조건 글이 머리 칸·테두리와 겹치지 않게(원본은 폭을 늘리지
// 않고 라벨을 묶음 폭에 감아 두 줄로 꺾는다. 이 판은 의뢰 결정대로 폭을 늘리고, condMaxW 를 넘는 긴 글만 감는다).
func (l *layout) walkBlockEnd() {
	if len(l.stack) == 0 {
		return
	}
	f := l.stack[len(l.stack)-1]
	l.stack = l.stack[:len(l.stack)-1]
	l.pos += loopPad
	f.bot = l.pos
	if f.x1 == math.MaxFloat64 {
		f.x1 = l.center[0] - actorW/2 - loopPad
		f.x2 = l.center[l.n-1] + actorW/2 + loopPad
	}
	if need := f.minWidth(); f.x2-f.x1 < need {
		c := (f.x1 + f.x2) / 2
		f.x1, f.x2 = c-need/2, c+need/2
	}
	l.insert(f.x1, f.top, f.x2, f.bot)
}

// minWidth 는 묶음이 담아야 할 최소 폭이다 — 머리 칸(loop·alt …) 오른쪽에 조건 글이 들어가려면 머리 칸 폭 +
// 조건 글 폭 + 양쪽 여백이 필요하다(구획 글은 머리 칸과 다른 줄이라 글 폭 + 여백만).
func (f *frameDraw) minWidth() float64 {
	if f.kind == bRect {
		return 0
	}
	need := 2*loopPad + labelBoxW
	if w := diagram.LinesW(f.lines); w > 0 {
		need += labelPad + w
	}
	for _, sec := range f.sections {
		if w := diagram.LinesW(sec.lines); w > 0 && 2*loopPad+w > need {
			need = 2*loopPad + w
		}
	}
	return need
}

// condLines 는 조건 글을 대괄호를 감싼 줄로 나눈다(빈 글은 줄 없음 — 원본도 빈 message 에는 대괄호를 붙이지 않는다).
func condLines(text string) []string {
	if text == "" {
		return nil
	}
	return diagram.WrapLabel("["+text+"]", condMaxW)
}

// stripH 는 조건 글 줄 수에 맞춘 머리·구획 줄 높이다(한 줄이면 머리 칸 높이와 같게).
func stripH(n int) float64 {
	h := float64(n)*lineH + 12
	if h < labelBoxH+loopPad {
		h = labelBoxH + loopPad
	}
	return h
}

// emit 은 걷어 둔 조각을 SVG 로 쓴다. 전체 폭을 알았으니 좌표를 x+dx 로 옮긴다.
func (l *layout) emit() string {
	c := l.c
	minX := l.minX - padX
	if len(l.center) > 0 {
		minX = math.Min(minX, l.center[0]-actorW/2-padX)
		minX = math.Min(minX, l.center[l.n-1]-actorW/2-padX)
	}
	maxX := l.maxX + padX
	if len(l.center) > 0 {
		maxX = math.Max(maxX, l.center[0]+actorW/2+padX)
		maxX = math.Max(maxX, l.center[l.n-1]+actorW/2+padX)
	}
	// box 띠·제목까지 폭을 본다(제목은 한 줄 — timeline·gantt 처럼 폭을 제목까지 넓힌다)
	for _, b := range c.Boxes {
		if b.Count == 0 {
			continue
		}
		left, right := l.boxRange(b)
		minX = math.Min(minX, left-boxPad-padX)
		maxX = math.Max(maxX, right+boxPad+padX)
	}
	dx := -minX + padX
	totalW := maxX - minX + padX
	if c.Title != "" {
		if w := diagram.TextW(c.Title, titleSz) + 2*padX; w > totalW {
			totalW = w
		}
	}
	totalH := l.mirrorTop + l.actorH + padT

	s := &diagram.Writer{}
	label := c.AccTitle
	if label == "" {
		label = c.FMCaption
	}
	if label == "" {
		label = c.Title
	}
	if label == "" {
		label = "순서도"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(totalW)+" "+diagram.F1(totalH),
		"width", diagram.F1(totalW*diagram.Scale), "height", diagram.F1(totalH*diagram.Scale), "role", "img", "aria-label", label)
	if c.AccDescr != "" {
		s.Open("title")
		s.Text(c.AccDescr)
		s.Close("title")
	}
	X := func(v float64) float64 { return v + dx }

	// 1. rect 띠(옅게 — 모든 내용 뒤)
	for _, f := range l.frames {
		if f.kind != bRect {
			continue
		}
		s.Leaf("rect", "class", "sq-rect", "x", diagram.F1(X(f.x1)), "y", diagram.F1(f.top), "width", diagram.F1(f.x2-f.x1), "height", diagram.F1(f.bot-f.top))
	}
	// 2. box 띠와 이름(이름은 띠 폭에서 감아 여러 줄 — 띠 높이에 반영, 리뷰 1회차 [4])
	for _, b := range c.Boxes {
		if b.Count == 0 {
			continue
		}
		left, right := l.boxRange(b)
		top := l.actorTop
		if b.Name != "" {
			top -= l.boxStrip[b]
		} else {
			top -= boxPad
		}
		bot := l.mirrorTop + l.actorH + boxPad
		s.Leaf("rect", "class", "sq-box", "x", diagram.F1(X(left-boxPad)), "y", diagram.F1(top),
			"width", diagram.F1(right-left+2*boxPad), "height", diagram.F1(bot-top))
		if lines := l.boxLines[b]; len(lines) > 0 {
			s.Open("text", "class", "sq-box-t", "x", diagram.F1(X(left)), "y", diagram.F1(top+boxPad), "font-size", diagram.F1(diagram.FontSize))
			for _, ln := range lines {
				s.Open("tspan", "x", diagram.F1(X(left)), "y", diagram.F1(top+boxPad+diagram.FontSize*0.85))
				s.Text(ln)
				s.Close("tspan")
				top += lineH
			}
			s.Close("text")
		}
	}
	// 3. 생명선(점선) — 참여자 상자 아래에서 아래 상자까지. 글 바탕이 덮을 수 있게 먼저 그린다.
	for i := range l.center {
		s.Leaf("line", "class", "sq-life", "x1", diagram.F1(X(l.center[i])), "y1", diagram.F1(l.actorTop+l.actorH),
			"x2", diagram.F1(X(l.center[i])), "y2", diagram.F1(l.mirrorTop))
	}
	// 4. 묶음 테두리·머리 칸·구획 점선(조건 글은 6.5 에서 그린다 — 활성 막대·메시지보다 뒤에)
	for _, f := range l.frames {
		if f.kind == bRect {
			continue
		}
		s.Leaf("rect", "class", "sq-frame", "x", diagram.F1(X(f.x1)), "y", diagram.F1(f.top), "width", diagram.F1(f.x2-f.x1), "height", diagram.F1(f.bot-f.top))
		// 머리 칸(모서리가 잘린 위쪽 사각형 — 원본 labelBox)
		cut := 7.0
		s.Leaf("path", "class", "sq-lab", "d", pathD([][2]float64{
			{f.x1, f.top}, {f.x1 + labelBoxW, f.top}, {f.x1 + labelBoxW, f.top + labelBoxH - cut},
			{f.x1 + labelBoxW - cut*1.2, f.top + labelBoxH}, {f.x1, f.top + labelBoxH},
		}, X, true))
		boxTextY(s, "sq-lab-t", X(f.x1), f.top, labelBoxW, labelBoxH, []string{f.name})
		for _, sec := range f.sections {
			s.Leaf("line", "class", "sq-div", "x1", diagram.F1(X(f.x1)), "y1", diagram.F1(sec.y), "x2", diagram.F1(X(f.x2)), "y2", diagram.F1(sec.y))
		}
	}
	// 5. 활성 막대
	for _, a := range l.acts {
		s.Leaf("rect", "class", "sq-act", "x", diagram.F1(X(a.x)), "y", diagram.F1(a.y), "width", diagram.F1(actW), "height", diagram.F1(a.h))
	}
	// 6. 메시지 — 글 바탕·글자·선·화살촉·번호(번호는 보낸 쪽 선 끝에 붙는다 — 원본 autonumberX)
	for _, m := range l.msgs {
		cx := (m.startx + m.overX) / 2
		if m.self {
			cx = m.startx
		}
		if m.textW > 0 {
			s.Leaf("rect", "class", "sq-tbg", "x", diagram.F1(X(cx-m.textW/2)-2), "y", diagram.F1(m.y0-1),
				"width", diagram.F1(m.textW+4), "height", diagram.F1(m.y1-m.y0+2))
			boxTextY(s, "sq-msg-t", X(cx-m.textW/2), m.y0, m.textW, m.y1-m.y0, m.lines)
		}
		cls := "sq-msg"
		if m.ar.Dotted {
			cls += " sq-dt"
		}
		if m.self {
			y0 := m.arrowY
			y1 := y0 + selfDrop
			s.Leaf("path", "class", cls, "d", pathD([][2]float64{
				{m.startx, y0}, {m.overX, y0}, {m.overX, y1}, {m.startx + headL + 1, y1},
			}, X, false))
			head(s, X(m.startx+1), y1, -1, m.ar)
			if m.ar.Both {
				head(s, X(m.startx+1), y0, 1, m.ar)
			}
			if m.num != "" {
				numCircle(s, X(m.startx-numR-2), y0, m.numSz, m.num)
			}
			continue
		}
		x1, x2 := m.startx, m.overX
		dir := 1.0
		if x2 < x1 {
			dir = -1
		}
		short := 0.0
		if m.num != "" {
			short = 2*numR + 2 // 동그라미 자리
		}
		if m.ar.Head != 'n' {
			x2 -= dir * 2 // 화살촉 밑동까지
		}
		if m.ar.Both {
			x1 += dir * 2
		}
		s.Leaf("line", "class", cls, "x1", diagram.F1(X(x1+short*dir)), "y1", diagram.F1(m.arrowY), "x2", diagram.F1(X(x2)), "y2", diagram.F1(m.arrowY))
		head(s, X(m.overX), m.arrowY, dir, m.ar)
		if m.ar.Both {
			head(s, X(m.startx), m.arrowY, -dir, m.ar)
		}
		if m.num != "" {
			numCircle(s, X(m.startx+dir*(numR+1)), m.arrowY, m.numSz, m.num)
		}
	}
	// 6.5 묶음 조건 글·구획 글 — 종이색 바탕 rect(tsq-tbg)를 깔고 활성 막대·생명선·메시지보다 뒤에 그린다(수정 1 —
	// 막대가 조건 글을 덮던 것). 조건 글은 머리 칸 오른쪽(원본 title x 공식과 같은 길), 구획 글은 묶음 가운데(원본과 같다).
	for _, f := range l.frames {
		if f.kind == bRect {
			continue
		}
		if w := diagram.LinesW(f.lines); w > 0 {
			x := f.x1 + labelBoxW + labelPad
			inner := f.x2 - loopPad - x
			if w > inner {
				w = inner
			}
			textBG(s, X(x+(inner-w)/2), f.top, w, f.titleH, f.lines)
			boxTextY(s, "sq-sec-t", X(x+(inner-w)/2), f.top, w, f.titleH, f.lines)
		}
		for _, sec := range f.sections {
			if w := diagram.LinesW(sec.lines); w > 0 {
				inner := f.x2 - f.x1 - 2*loopPad
				if w > inner {
					w = inner
				}
				x := f.x1 + loopPad + (inner-w)/2
				h := stripH(len(sec.lines)) - 4
				textBG(s, X(x), sec.y, w, h, sec.lines)
				boxTextY(s, "sq-sec-t", X(x), sec.y, w, h, sec.lines)
			}
		}
	}
	// 7. 메모
	for _, nt := range l.notes {
		s.Leaf("rect", "class", "sq-note", "x", diagram.F1(X(nt.x)), "y", diagram.F1(nt.y), "width", diagram.F1(nt.w), "height", diagram.F1(nt.h))
		boxTextY(s, "sq-note-t", X(nt.x), nt.y, nt.w, nt.h, nt.lines)
	}
	// 8. 참여자 상자(위·아래 되풀이 — 원본 기본 mirrorActors)
	for _, top := range [...]float64{l.actorTop, l.mirrorTop} {
		for i, a := range c.Actors {
			x := l.center[i] - actorW/2
			if a.Actor {
				l.person(s, X(x), top)
			} else {
				s.Leaf("rect", "class", "sq-par", "x", diagram.F1(X(x)), "y", diagram.F1(top), "width", diagram.F1(actorW), "height", diagram.F1(l.actorH), "rx", "3")
			}
			ty := top
			if a.Actor {
				ty += glyphH + glyphGap // 사람 모양 아래에 이름
			}
			boxTextY(s, "sq-par-t", X(x), ty, actorW, l.actorH-(ty-top), l.actorLines[i])
		}
	}
	// 9. 제목
	if c.Title != "" {
		s.Open("text", "class", "sq-title", "x", diagram.F1(padX), "y", diagram.F1(padT+titleSz*0.85), "font-size", diagram.F1(titleSz))
		s.Text(c.Title)
		s.Close("text")
	}
	s.Close("svg")
	return s.String()
}

// person 은 참여자 상자 안에 사람 모양(머리 원 + 몸)을 그린다 — 원본 drawActorTypeActor 의 비율을 상자에 맞춘다.
func (l *layout) person(s *diagram.Writer, x, top float64) {
	cx := x + actorW/2
	hy := top + 4
	s.Leaf("circle", "class", "sq-fig-h", "cx", diagram.F1(cx), "cy", diagram.F1(hy+7), "r", "7")
	by := hy + 14
	end := top + 4 + glyphH
	s.Leaf("line", "class", "sq-fig", "x1", diagram.F1(cx), "y1", diagram.F1(by), "x2", diagram.F1(cx), "y2", diagram.F1(end-8))
	s.Leaf("line", "class", "sq-fig", "x1", diagram.F1(cx-8), "y1", diagram.F1(by+5), "x2", diagram.F1(cx+8), "y2", diagram.F1(by+5))
	s.Leaf("path", "class", "sq-fig", "d", pathD([][2]float64{
		{cx - 7, end}, {cx, end - 8}, {cx, end - 8}, {cx + 7, end},
	}, func(v float64) float64 { return v }, false))
}

// textBG 는 글 블록의 종이색 바탕 rect 를 깐다(선·막대가 글자를 가로지르지 않게 — 메시지 글 바탕과 같은 토큰).
func textBG(s *diagram.Writer, x, y, w, h float64, lines []string) {
	s.Leaf("rect", "class", "sq-tbg", "x", diagram.F1(x-2), "y", diagram.F1(y),
		"width", diagram.F1(w+4), "height", diagram.F1(math.Max(h, float64(len(lines))*lineH)+2))
}

// numCircle 은 autonumber 동그라미와 숫자를 그린다(숫자 글자는 동그라미 가운데 — 크기는 자릿수 사다리).
func numCircle(s *diagram.Writer, x, y, sz float64, num string) {
	s.Leaf("circle", "class", "sq-num", "cx", diagram.F1(x), "cy", diagram.F1(y), "r", diagram.F1(numR))
	s.Open("text", "class", "sq-num-t", "x", diagram.F1(x), "y", diagram.F1(y+sz*0.35), "text-anchor", "middle", "font-size", diagram.F1(sz))
	s.Text(num)
	s.Close("text")
}

// head 는 화살촉을 그린다(dir 은 화살표 방향). 채운 화살촉·열린(가는) 화살촉은 path, 가위표는 두 선이다.
func head(s *diagram.Writer, x, y, dir float64, ar arrowKind) {
	switch ar.Head {
	case 'f':
		// 채운 삼각형 — 원본 arrowhead marker(11 길이 × 10 높이) 비율.
		s.Leaf("path", "class", "sq-head", "d", pathD([][2]float64{
			{x, y}, {x - dir*headL, y - headW}, {x - dir*headL, y + headW},
		}, func(v float64) float64 { return v }, true))
	case 'o':
		// 가는 화살촉(-) · --)) — 원본 filled-head marker(9 길이 × 12 높이, 등이 4 들어간 바베트) 비율.
		// 수정 1: 얇은 삼각형으로는 ->> 와 구분이 약했으므로 원본 비율로 맞췄다.
		s.Leaf("path", "class", "sq-head", "d", pathD([][2]float64{
			{x, y}, {x - dir*9, y - 6}, {x - dir*5, y}, {x - dir*9, y + 6},
		}, func(v float64) float64 { return v }, true))
	case 'x':
		c := x + dir*crossS
		s.Leaf("path", "class", "sq-cross", "d", fmt2("M", c-crossS, y-crossS, "L", c+crossS, y+crossS)+fmt2("M", c-crossS, y+crossS, "L", c+crossS, y-crossS))
	}
}

// boxRange 는 box 가 덮는 참여자 범위다(원본이 첫·끝 참여자로 띠를 계산하는 것과 같다).
func (l *layout) boxRange(b *box) (float64, float64) {
	left, right := math.MaxFloat64, -math.MaxFloat64
	for i, a := range l.c.Actors {
		if a.Box >= 0 && l.c.Boxes[a.Box] == b {
			left = math.Min(left, l.center[i]-actorW/2)
			right = math.Max(right, l.center[i]+actorW/2)
		}
	}
	if left == math.MaxFloat64 {
		return l.center[0] - actorW/2, l.center[0] + actorW/2
	}
	return left, right
}

// wrapMsg 는 메시지 글을 줄로 나눈다(빈 글은 줄 없음).
func wrapMsg(text string) []string {
	if text == "" {
		return nil
	}
	return diagram.WrapLabel(text, msgWrap)
}

// fmtNum 은 autonumber 숫자를 적는다(소수는 아래 0 을 지운다).
func fmtNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// pathD 는 점들을 잇는 path 의 d 값을 만든다(X 로 x 좌표를 옮긴다. close 면 Z).
func pathD(pts [][2]float64, x func(float64) float64, close bool) string {
	var b []byte
	b = append(b, 'M')
	for i, p := range pts {
		if i > 0 {
			b = append(b, 'L')
		}
		b = append(b, diagram.F1(x(p[0]))...)
		b = append(b, ' ')
		b = append(b, diagram.F1(p[1])...)
	}
	if close {
		b = append(b, 'Z')
	}
	return string(b)
}

// fmt2 는 두 점을 잇는 짧은 path 조각이다(가위표).
func fmt2(cmd string, x1, y1 float64, cmd2 string, x2, y2 float64) string {
	return cmd + diagram.F1(x1) + " " + diagram.F1(y1) + " " + cmd2 + diagram.F1(x2) + " " + diagram.F1(y2)
}

// boxTextY 는 상자(x, y, w, h — x 는 이미 옮긴 좌표) 안에 줄들을 가운데 맞춤으로 쓴다(timeline 의 boxText 와 같은 꼴).
func boxTextY(s *diagram.Writer, cls string, x, y, w, h float64, lines []string) {
	if len(lines) == 0 {
		return
	}
	top := y + (h-float64(len(lines))*lineH)/2
	s.Open("text", "class", cls, "text-anchor", "middle", "font-size", diagram.F1(diagram.FontSize))
	for j, l := range lines {
		s.Open("tspan", "x", diagram.F1(x+w/2), "y", diagram.F1(top+lineH*(float64(j)+0.5)+diagram.FontSize*0.35))
		s.Text(l)
		s.Close("tspan")
	}
	s.Close("text")
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
