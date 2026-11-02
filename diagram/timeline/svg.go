package timeline

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
)

// 그리기. 쓰는 요소·속성은 흐름도·간트와 같은 허용 목록이다(svg·rect·line·path·text·tspan·title —
// mdoc/security_test.go 가 고정한다). 화살촉은 marker 가 아니라 삼각형 path(흐름도와 같다). 색은 클래스 → 차트
// 팔레트(--chart-1…8, base.css) — 새 색 상수·새 토큰 없이 테마를 따른다. 클래스는 모두 tl- 접두사(앱 CSS 충돌 —
// timeline_test.go TestClassCollisions):
//
//	tl-sec·tl-per·tl-evn(+색 tl-c1…8)   구역 머리·시기·사건 상자(사건은 같은 색을 반투명으로)
//	tl-sec-t·tl-per-t·tl-evn-t          상자 글자(진한 면 위 --chart-on, 사건은 --diagram-text)
//	tl-axis·tl-drop·tl-head             시간축 선·세로 점선·화살촉
//	tl-title                            그림 안 제목
//
// 배치 상수(viewBox 단위, 글자 14 기준). 상자 폭은 고정이고 전체 폭은 시기 수에 정비례한다 — 넓은 그림은
// 흐름도·간트와 같이 viewBox 로 쪽 너비에 맞아 줄어든다.
const (
	boxW    = 160.0 // 시기·사건 상자 폭(고정 — 긴 글은 줄을 바꾼다)
	colGap  = 20.0  // 시기 열 사이
	padX    = 8.0   // 왼쪽 여백
	padT    = 10.0
	textPad = 12.0 // 상자 안 글자의 좌우 여백(합) — 줄바꿈 폭 = boxW - textPad
	boxPad  = 9.0  // 상자 안 글자의 위아래 여백(합)
	evGap   = 8.0  // 사건 상자 사이(세로)
	secGap  = 10.0 // 구역 머리 행과 시기 행 사이
	secPad  = 8.0  // 구역 머리 안 글자의 위아래 여백(합)
	axisGap = 18.0 // 시기 행 아래 → 시간축 선
	tail    = 30.0 // 시간축의 오른쪽 연장(화살촉 자리 포함)
	evTopG  = 20.0 // 시간축 선 → 첫 사건 상자
	dropEnd = 6.0  // 마지막 사건 상자 → 점선 끝(화살촉 자리 포함)
	lineH   = 18.0 // diagram.LineH — 줄 간격
	titleSz = 15.0 // 그림 안 제목 글자 크기
)

// palette 는 색 차례의 팔레트 클래스다(구역마다, 구역이 없으면 시기마다 순환 — 차트와 같은 8색).
func palette(i int) string { return fmt.Sprintf("tl-c%d", i%8+1) }

// group 은 한 색을 이루는 시기 묶음이다(구역 없는 앞묶음 포함).
type group struct {
	first  int
	count  int
	secIdx int // tl.Sections 차례(구역 없는 묶음이면 -1)
}

// drawSVG 는 tl 을 SVG 로 그린다. 원본 Mermaid 렌더러(timelineRenderer.ts)와 같은 뼈대를 테마 토큰으로 옮긴 것이다:
// 구역 머리 상자는 그 구역 시기들의 폭만큼 걸치고, 시기 상자는 한 줄로 늘어서고, 사건 상자는 시기 아래 세로 점선에
// 매달린다. 원본은 활동선을 맨 아래에 두지만 이 판은 시기 행과 사건 사이(시간축)에 둔다(안내서 9장).
func drawSVG(c *tl) (string, []Issue, error) {
	n := len(c.Periods)

	// 구역 없는 앞 시기도 그린다(원본 렌더러는 그리지 않는다 — 안내서 9장). 묶음: 구역 없는 앞묶음 + 시기 있는 구역.
	used := make([]bool, len(c.Sections))
	for _, d := range c.Periods {
		if d.Section >= 0 {
			used[d.Section] = true
		}
	}
	var groups []group
	{
		i := 0
		for i < n && c.Periods[i].Section < 0 {
			i++
		}
		if i > 0 {
			groups = append(groups, group{first: 0, count: i, secIdx: -1})
		}
		for s := range c.Sections {
			if !used[s] {
				continue
			}
			j := i
			for j < n && c.Periods[j].Section == s {
				j++
			}
			groups = append(groups, group{first: i, count: j - i, secIdx: s})
			i = j
		}
	}
	// 색 차례 — 구역이 있는 그림이면 구역마다(구역 없는 앞묶음은 0번), 구역이 없는 그림이면 시기마다(원본 cScale 순환과 같다).
	order := make([]int, len(c.Sections))
	for i := range order {
		order[i] = -1
	}
	k := 0
	noSec := len(groups) == 1 && groups[0].secIdx < 0
	if !noSec && len(groups) > 0 && groups[0].secIdx < 0 {
		k = 1 // 구역 없는 앞묶음이 0번, 구역은 1번부터
	}
	for _, g := range groups {
		if g.secIdx >= 0 {
			order[g.secIdx] = k
			k++
		}
	}
	colorOf := func(i int) int {
		s := c.Periods[i].Section
		if s >= 0 && order[s] >= 0 {
			return order[s]
		}
		if noSec {
			return i // 구역이 전혀 없는 그림 — 시기마다 순환(원본 cScale 과 같다)
		}
		return 0 // 구역 있는 그림의 구역 없는 앞 시기 — 모두 앞묶음 색(리뷰 2회차 [2]: 시기 인덱스로 두면 뒤 구역 색을 미리 쓴다)
	}

	// 줄 나누기(폭 고정) — 시기 행 높이는 가장 많은 줄 수에 맞춘다(같은 행의 상자 높이를 맞춘다).
	perLines := make([][]string, n)
	maxPer := 1
	for i, d := range c.Periods {
		perLines[i] = diagram.WrapLabel(d.Name, boxW-textPad)
		maxPer = max(maxPer, len(perLines[i]))
	}
	evLines := make([][][]string, n) // 시기 → 사건 → 줄
	evH := make([][]float64, n)      // 시기 → 사건 상자 높이
	maxStack := 0.0
	for i, d := range c.Periods {
		h := 0.0
		for _, e := range d.Events {
			ls := diagram.WrapLabel(e, boxW-textPad)
			evLines[i] = append(evLines[i], ls)
			hh := float64(len(ls))*lineH + boxPad
			evH[i] = append(evH[i], hh)
			h += hh + evGap
		}
		if len(d.Events) > 0 {
			h -= evGap
		}
		maxStack = max(maxStack, h)
	}
	// 구역 머리 줄 — 머리 폭은 시기 폭만큼 넓어 보통 한 줄이지만, 줄 수가 다르면 가장 많은 쪽에 맞춘다(행 높이 통일).
	maxSec := 1
	secLines := map[int][]string{}
	for _, g := range groups {
		if g.secIdx < 0 {
			continue
		}
		w := float64(g.count)*boxW + float64(g.count-1)*colGap - textPad
		ls := diagram.WrapLabel(c.Sections[g.secIdx].Name, w)
		secLines[g.secIdx] = ls
		maxSec = max(maxSec, len(ls))
	}

	// 세로 줄맞춤
	topY := padT
	if c.Title != "" {
		topY += titleSz + 8
	}
	secY := topY
	secH := 0.0
	if len(secLines) > 0 {
		secH = float64(maxSec)*lineH + secPad
	}
	perY := secY + secH
	if secH > 0 {
		perY += secGap
	}
	perH := float64(maxPer)*lineH + boxPad
	axisY := perY + perH + axisGap
	evTop := axisY + evTopG
	totalH := evTop + maxStack + dropEnd + 9
	if maxStack == 0 {
		totalH = axisY + 14 // 사건 없는 그림 — 축까지
	}

	// 가로 줄맞춤 — 제목이 시기보다 길면 그 폭까지 넓힌다(gantt 의 제목 처리와 같다 — 리뷰 [1]. 제목은 한 줄이라
	// 상자 라벨처럼 줄을 바꾸지 않는다).
	x0 := padX
	colX := func(i int) float64 { return x0 + float64(i)*(boxW+colGap) }
	lastX := colX(n-1) + boxW
	axisX1, axisX2 := x0-6, lastX+tail
	totalW := axisX2 + 4
	if c.Title != "" {
		if w := diagram.TextW(c.Title, titleSz) + padX + 8; w > totalW {
			totalW = w
		}
	}

	s := &diagram.Writer{}
	label := c.AccTitle
	if label == "" {
		label = c.FMCaption
	}
	if label == "" {
		label = c.Title
	}
	if label == "" {
		label = "타임라인"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(totalW)+" "+diagram.F1(totalH),
		"width", diagram.F1(totalW*diagram.Scale), "height", diagram.F1(totalH*diagram.Scale), "role", "img", "aria-label", label)
	if c.AccDescr != "" {
		s.Open("title")
		s.Text(c.AccDescr)
		s.Close("title")
	}

	// 세로 점선(시기 상자 밑 → 마지막 사건 밑 화살촉까지 하나로)과 시간축 선을 모든 상자보다 먼저 그린다(원본도 점선이
	// 사건 상자 뒤로 지나간다). 사건 상자는 반투명이라 점선이 비쳐 보이지 않게, 종이색 바탕 rect(tl-evn-bg) 위에 얹는다.
	for i, d := range c.Periods {
		if len(d.Events) == 0 {
			continue
		}
		cx := colX(i) + boxW/2
		stack := 0.0
		for _, h := range evH[i] {
			stack += h + evGap
		}
		end := evTop + stack - evGap + dropEnd // 마지막 상자 하단 + dropEnd
		s.Leaf("line", "class", "tl-drop", "x1", diagram.F1(cx), "y1", diagram.F1(perY+perH+1), "x2", diagram.F1(cx), "y2", diagram.F1(end))
		s.Leaf("path", "class", "tl-head", "d", fmt.Sprintf("M%s %s L%s %s L%s %s Z",
			diagram.F1(cx), diagram.F1(end+5), diagram.F1(cx-3), diagram.F1(end), diagram.F1(cx+3), diagram.F1(end)))
	}
	s.Leaf("line", "class", "tl-axis", "x1", diagram.F1(axisX1), "y1", diagram.F1(axisY), "x2", diagram.F1(axisX2-7), "y2", diagram.F1(axisY))
	s.Leaf("path", "class", "tl-head", "d", fmt.Sprintf("M%s %s L%s %s L%s %s Z",
		diagram.F1(axisX2), diagram.F1(axisY), diagram.F1(axisX2-7), diagram.F1(axisY-4), diagram.F1(axisX2-7), diagram.F1(axisY+4)))

	// 사건 상자 — 종이색 바탕(tl-evn-bg, 점선을 가린다) 위에 같은 색의 반투명 상자와 글자
	for i, d := range c.Periods {
		if len(d.Events) == 0 {
			continue
		}
		y := evTop
		for k, ls := range evLines[i] {
			h := evH[i][k]
			s.Leaf("rect", "class", "tl-evn-bg", "x", diagram.F1(colX(i)), "y", diagram.F1(y),
				"width", diagram.F1(boxW), "height", diagram.F1(h), "rx", "5")
			s.Leaf("rect", "class", "tl-evn "+palette(colorOf(i)), "x", diagram.F1(colX(i)), "y", diagram.F1(y),
				"width", diagram.F1(boxW), "height", diagram.F1(h), "rx", "5")
			boxText(s, "tl-evn-t", colX(i), y, boxW, h, ls)
			y += h + evGap
		}
	}

	// 시기 상자와 글자
	for i := range c.Periods {
		s.Leaf("rect", "class", "tl-per "+palette(colorOf(i)), "x", diagram.F1(colX(i)), "y", diagram.F1(perY),
			"width", diagram.F1(boxW), "height", diagram.F1(perH), "rx", "5")
		boxText(s, "tl-per-t", colX(i), perY, boxW, perH, perLines[i])
	}

	// 구역 머리 상자와 글자
	for _, g := range groups {
		if g.secIdx < 0 {
			continue
		}
		w := float64(g.count)*boxW + float64(g.count-1)*colGap
		s.Leaf("rect", "class", "tl-sec "+palette(order[g.secIdx]), "x", diagram.F1(colX(g.first)),
			"y", diagram.F1(secY), "width", diagram.F1(w), "height", diagram.F1(secH), "rx", "3")
		boxText(s, "tl-sec-t", colX(g.first), secY, w, secH, secLines[g.secIdx])
	}

	// 그림 안 제목
	if c.Title != "" {
		s.Open("text", "class", "tl-title", "x", diagram.F1(padX), "y", diagram.F1(padT+titleSz*0.85), "font-size", diagram.F1(titleSz))
		s.Text(c.Title)
		s.Close("text")
	}

	s.Close("svg")
	return diagram.AriaRefs(s.String()), nil, nil // aria-label 의 탭·줄바꿈은 문자 참조로(공통 — state 와 같다)
}

// boxText 는 상자(x, y, w, h) 안에 줄들을 가운데 맞춤으로 쓴다(gantt 의 tspan 방식 — 줄마다 tspan, 줄 간격 lineH).
func boxText(s *diagram.Writer, cls string, x, y, w, h float64, lines []string) {
	top := y + (h-float64(len(lines))*lineH)/2
	s.Open("text", "class", cls, "text-anchor", "middle", "font-size", diagram.F1(diagram.FontSize))
	for j, l := range lines {
		s.Open("tspan", "x", diagram.F1(x+w/2), "y", diagram.F1(top+lineH*(float64(j)+0.5)+diagram.FontSize*0.35))
		s.Text(l)
		s.Close("tspan")
	}
	s.Close("text")
}
