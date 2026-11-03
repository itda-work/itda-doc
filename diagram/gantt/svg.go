package gantt

import (
	"fmt"
	"time"

	"github.com/itda-work/itda-doc/diagram"
)

// 그리기. 쓰는 요소·속성은 흐름도와 같은 허용 목록이다(svg·g·rect·path·line·text·tspan·title —
// mdoc/security_test.go 가 고정한다). style·이벤트·url()·href·id 없음. 색·선은 클래스 → 테마 토큰(--diagram-*, base.css).
// 클래스는 모두 gt- 접두사(호스트 앱 창 틀 CSS 의 .bar 같은 앱 규칙과 충돌하지 않게 — 수정 1회차):
//
//	gt-band·gt-excl                 구역 띠·제외 날짜 띠          gt-grid·gt-axis   눈금 격자선·축 선
//	gt-alab·gt-sname                축 라벨·구역 이름              gt-bar(+gt-done·gt-active·gt-crit)·gt-ms   막대·마일스톤 마름모
//	gt-tname(+gt-in, 막대 안이면 태그도 gt-done·gt-active·gt-crit)·gt-title   작업 이름·그림 제목
//
// 배치 상수(viewBox 단위, 글자 14 기준). 시간축 폭은 고정이고 전체 폭은 구역 이름 열 + 오른쪽 이름 폭에 따라 정해진다 —
// 넓은 그림은 흐름도와 같이 viewBox 로 쪽 너비에 맞아 줄어든다.
const (
	axisW    = 620.0 // 시간축 폭
	rowH     = 26.0  // 작업 하나의 행 높이
	barH     = 13.0  // 막대 높이
	msHalf   = 8.0   // 마일스톤 마름모의 반폭·반높이
	small    = 12.0  // 축 라벨·구역 이름·작업 이름 글자 크기
	titleSz  = 15.0  // 그림 안 제목 글자 크기
	padLeft  = 8.0
	padTop   = 8.0
	axisArea = 30.0 // 아래 축(선 + 라벨)
	nameMaxW = 220.0
	secMaxW  = 150.0
	lineH    = 16.0 // 여러 줄 이름의 줄 간격
	rowPad   = 10.0 // 여러 줄 이름 행의 위아래 여백 합
	labGap   = 16.0 // 눈금 라벨 사이 목표 여백 — 목표 눈금 수 = 축 폭 ÷ (라벨 폭 + labGap)
)

// group 은 한 구역 띠에 들어가는 작업 묶음이다(구역 없는 앞작업 묶음 포함).
type group struct {
	name   string
	first  int
	count  int
	secIdx int // chart.Sections 차례(구역 없는 묶음이면 -1)
}

// drawSVG 는 chart 를 SVG 로 그린다. issues 는 그리면서 알린 것(제외 띠 상한·tickInterval 을 바꿔 그림).
// 눈금 생성이 나아가지 않으면 *Error 다(genTicks).
func drawSVG(c *chart) (string, []Issue, error) {
	var issues []Issue

	// 시각 범위: 작업 전체(rlo..rhi — 눈금의 기준)에 양쪽 여백(범위의 1/40, 범위가 한 점이면 하루)
	rlo, rhi := timeRange(c.Tasks)
	lo, hi := rlo, rhi
	span := hi.Sub(lo)
	if span <= 0 {
		span = 24 * time.Hour
	}
	lo, hi = lo.Add(-span/40), hi.Add(span/40)
	span = hi.Sub(lo)

	// 구역 묶음(빈 구역은 그리지 않는다 — 파서가 경고했다)
	used := make([]bool, len(c.Sections))
	for _, t := range c.Tasks {
		if t.Section >= 0 {
			used[t.Section] = true
		}
	}
	var groups []group
	{
		i := 0
		for i < len(c.Tasks) && c.Tasks[i].Section < 0 {
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
			for j < len(c.Tasks) && c.Tasks[j].Section == s {
				j++
			}
			groups = append(groups, group{name: c.Sections[s].Name, first: i, count: j - i, secIdx: s})
			i = j
		}
	}

	// 폭: 왼쪽 구역 이름 열 + 시간축 + 오른쪽 작업 이름
	leftW := 0.0
	secLines := map[int][]string{}
	for _, g := range groups {
		if g.secIdx < 0 {
			continue
		}
		ls := diagram.WrapLabel(g.name, secMaxW)
		secLines[g.secIdx] = ls
		for _, l := range ls {
			if w := diagram.TextW(l, small) + 18; w > leftW {
				leftW = w
			}
		}
	}
	if len(groups) > 0 && groups[0].secIdx < 0 && leftW == 0 {
		leftW = 12 // 구역 없는 그림도 시간축이 왼쪽에서 조금 떨어진다
	}
	ax0 := padLeft + leftW
	x := func(t time.Time) float64 {
		frac := float64(t.Sub(lo)) / float64(span)
		return ax0 + float64(frac*axisW) // float64(…) 변환은 곱셈-덧셈 합치기(FMA)를 막는다 — 흐름도와 같은 결정성 규칙
	}

	// 눈금 라벨의 그려질 x — 눈금 바로 아래(가운데 맞춤). 축 아래는 구역 이름 열도 비어 있어 왼쪽 끝 라벨은 그림 왼쪽 끝까지만
	// 당기고, 오른쪽 끝 라벨은 그림 폭을 넓혀 담는다(아래 totalW) — 라벨을 눈금에서 떼어 옮기지 않는다.
	labX := func(t time.Time) float64 {
		return max(x(t), diagram.TextW(c.AF.format(t), small)/2+2)
	}
	// 라벨 겹침 판정 — 그려질 그대로(끝 라벨 클램프 포함) 인접 라벨 쌍을 본다
	labFits := func(ts []time.Time) bool {
		prevEnd := -1.0
		for i, t := range ts {
			half := diagram.TextW(c.AF.format(t), small) / 2
			tx := labX(t)
			if i > 0 && tx-half < prevEnd+2 {
				return false
			}
			prevEnd = tx + half
		}
		return true
	}
	tickList, tickIssues, err := chooseTicks(c, rlo, rhi, labFits)
	if err != nil {
		return "", nil, err
	}
	issues = append(issues, tickIssues...)
	c.ticks = tickList

	// 각 작업의 막대·이름 자리
	type namePos struct {
		lines []string
		inBar bool
		x     float64 // 오른쪽 이름의 시작 x
		barX0 float64
		barX1 float64
		mid   float64 // 마일스톤 중심 x
		ms    bool
	}
	rightW := 0.0
	np := make([]namePos, len(c.Tasks))
	for i, t := range c.Tasks {
		n := &np[i]
		n.ms = t.Milestone
		if t.Milestone {
			// 마일스톤 마름모는 작업 시작·끝의 가운데(시작 + 기간/2 — Mermaid 와 같게)
			n.mid = x(t.Start.Add(t.End.Sub(t.Start) / 2))
			n.barX0, n.barX1 = n.mid-msHalf, n.mid+msHalf
		} else {
			n.barX0, n.barX1 = x(t.Start), x(t.End)
			if n.barX1-n.barX0 < 1.2 { // 0d 같은 순간 작업도 보인다
				n.barX1 = n.barX0 + 1.2
			}
		}
		if !t.Milestone && n.barX1-n.barX0 >= diagram.TextW(t.Name, small)+8 {
			n.inBar = true
			continue
		}
		n.x = n.barX1 + 5
		n.lines = diagram.WrapLabel(t.Name, nameMaxW)
		for _, l := range n.lines {
			if w := diagram.TextW(l, small) + 10; w > rightW {
				rightW = w
			}
		}
	}
	if rightW == 0 {
		rightW = 8
	}

	topY := padTop
	if c.Title != "" {
		topY += titleSz + 10
	}
	// 행 높이: 여러 줄 이름이면 줄 수만큼 늘린다. 구역은 행 합과 구역 이름 줄 수 가운데 큰 쪽(남는 높이는 구역 아래 여백).
	rowTop := make([]float64, len(c.Tasks))
	rowHt := make([]float64, len(c.Tasks))
	gTop := make([]float64, len(groups))
	gHt := make([]float64, len(groups))
	y := topY
	for gi, g := range groups {
		gTop[gi] = y
		sum := 0.0
		for i := g.first; i < g.first+g.count; i++ {
			h := rowH
			if k := len(np[i].lines); k > 1 {
				h = max(h, float64(k)*lineH+rowPad)
			}
			rowTop[i], rowHt[i] = y+sum, h
			sum += h
		}
		if g.secIdx >= 0 {
			sum = max(sum, float64(len(secLines[g.secIdx]))*lineH+rowPad)
		}
		gHt[gi] = sum
		y += sum
	}
	chartH := y - topY
	axisY := topY + chartH
	totalW := ax0 + axisW + rightW
	for _, t := range tickList {
		totalW = max(totalW, labX(t)+diagram.TextW(c.AF.format(t), small)/2+2)
	}
	if c.Title != "" {
		if w := diagram.TextW(c.Title, titleSz) + padLeft + 8; w > totalW {
			totalW = w
		}
	}
	totalH := axisY + axisArea

	s := &diagram.Writer{}
	label := c.AccTitle
	if label == "" {
		label = c.FMCaption
	}
	if label == "" {
		label = c.Title
	}
	if label == "" {
		label = "간트 차트"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(totalW)+" "+diagram.F1(totalH),
		"width", diagram.F1(totalW*diagram.Scale), "height", diagram.F1(totalH*diagram.Scale), "role", "img", "aria-label", label)
	if c.AccDescr != "" {
		s.Open("title")
		s.Text(c.AccDescr)
		s.Close("title")
	}

	// 구역 띠(짝수 차례 묶음은 바탕 없음, 홀수만 — 번갈이)
	for gi := range groups {
		if gi%2 == 0 {
			continue
		}
		s.Leaf("rect", "class", "gt-band", "x", diagram.F1(ax0), "y", diagram.F1(gTop[gi]),
			"width", diagram.F1(axisW), "height", diagram.F1(gHt[gi]))
	}

	// 제외 날짜의 옅은 세로 띠 — UTC 날짜 경계로 하루씩 보고([그날 00:00, 다음날 00:00) 을 그림 범위 lo..hi 로 잘라),
	// 하루 폭이 눈에 보일 때만, 400개까지. 판정은 그날 00:00 시각으로 — 날짜 꼴(시각 없음)의 끝 미루기와 같은 판정이다
	// (시각 꼴의 날짜 제외는 파서가 받지 않는다).
	if !c.Excl.none {
		dayW := x(lo.Add(24*time.Hour)) - x(lo)
		if dayW >= 0.5 {
			n := 0
			for d := time.Date(lo.Year(), lo.Month(), lo.Day(), 0, 0, 0, 0, time.UTC); d.Before(hi); d = d.AddDate(0, 0, 1) {
				if !c.Excl.check(d, c.DF) {
					continue
				}
				if n >= MaxTicks {
					issues = append(issues, Issue{Line: 0, Msg: "제외 날짜 띠가 많아 모두 그리지 않음"})
					break
				}
				a, b := d, d.AddDate(0, 0, 1)
				if a.Before(lo) {
					a = lo
				}
				if b.After(hi) {
					b = hi
				}
				s.Leaf("rect", "class", "gt-excl", "x", diagram.F1(x(a)), "y", diagram.F1(topY), "width", diagram.F1(x(b)-x(a)), "height", diagram.F1(chartH))
				n++
			}
		}
	}

	// 눈금 격자선·축 선·라벨
	for _, t := range tickList {
		tx := x(t)
		s.Leaf("line", "class", "gt-grid", "x1", diagram.F1(tx), "y1", diagram.F1(topY), "x2", diagram.F1(tx), "y2", diagram.F1(axisY))
	}
	s.Leaf("line", "class", "gt-axis", "x1", diagram.F1(ax0-6), "y1", diagram.F1(axisY), "x2", diagram.F1(ax0+axisW+6), "y2", diagram.F1(axisY))
	for _, t := range tickList { // 라벨은 눈금마다 — 격자선과 같다
		lab := c.AF.format(t)
		s.Open("text", "class", "gt-alab", "x", diagram.F1(labX(t)), "y", diagram.F1(axisY+18), "text-anchor", "middle", "font-size", diagram.F1(small))
		s.Text(lab)
		s.Close("text")
	}

	// 태그 클래스(막대·마름모·막대 안 이름이 같은 차례로 단다 — CSS 는 crit 를 마지막에 둔다)
	cls := func(t *task, base string) string {
		c := base
		if t.Done {
			c += " gt-done"
		}
		if t.Active {
			c += " gt-active"
		}
		if t.Crit {
			c += " gt-crit"
		}
		return c
	}
	ymOf := func(i int) float64 { return rowTop[i] + rowHt[i]/2 }

	// 막대·마일스톤
	for i, t := range c.Tasks {
		n := &np[i]
		ym := ymOf(i)
		if n.ms {
			m := cls(t, "gt-ms")
			d := "M" + diagram.F1(n.mid) + " " + diagram.F1(ym-msHalf) +
				" L" + diagram.F1(n.mid+msHalf) + " " + diagram.F1(ym) +
				" L" + diagram.F1(n.mid) + " " + diagram.F1(ym+msHalf) +
				" L" + diagram.F1(n.mid-msHalf) + " " + diagram.F1(ym) + " Z"
			s.Leaf("path", "class", m, "d", d)
			continue
		}
		s.Leaf("rect", "class", cls(t, "gt-bar"), "x", diagram.F1(n.barX0), "y", diagram.F1(ym-barH/2),
			"width", diagram.F1(n.barX1-n.barX0), "height", diagram.F1(barH), "rx", "3")
	}

	// 작업 이름 — 막대 안(폭이 되면), 아니면 오른쪽
	for i, t := range c.Tasks {
		n := &np[i]
		ym := ymOf(i)
		if n.inBar {
			s.Open("text", "class", cls(t, "gt-tname gt-in"), "x", diagram.F1((n.barX0+n.barX1)/2), "y", diagram.F1(ym+small*0.35),
				"text-anchor", "middle", "font-size", diagram.F1(small))
			s.Text(t.Name)
			s.Close("text")
			continue
		}
		lh := lineH
		top := ym - float64(float64(len(n.lines))*lh)/2
		s.Open("text", "class", "gt-tname", "font-size", diagram.F1(small))
		for j, l := range n.lines {
			s.Open("tspan", "x", diagram.F1(n.x), "y", diagram.F1(top+lh*(float64(j)+0.5)+small*0.35))
			s.Text(l)
			s.Close("tspan")
		}
		s.Close("text")
	}

	// 구역 이름 — 왼쪽 열(가로 글자, 오른쪽 끝 맞춤)
	for gi, g := range groups {
		if g.secIdx < 0 {
			continue
		}
		ym := gTop[gi] + gHt[gi]/2
		ls := secLines[g.secIdx]
		lh := lineH
		top := ym - float64(float64(len(ls))*lh)/2
		s.Open("text", "class", "gt-sname", "text-anchor", "end", "font-size", diagram.F1(small))
		for j, l := range ls {
			s.Open("tspan", "x", diagram.F1(ax0-8), "y", diagram.F1(top+lh*(float64(j)+0.5)+small*0.35))
			s.Text(l)
			s.Close("tspan")
		}
		s.Close("text")
	}

	// 그림 안 제목
	if c.Title != "" {
		s.Open("text", "class", "gt-title", "x", diagram.F1(padLeft), "y", diagram.F1(padTop+titleSz*0.85), "font-size", diagram.F1(titleSz))
		s.Text(c.Title)
		s.Close("text")
	}

	s.Close("svg")
	return diagram.AriaRefs(s.String()), issues, nil // aria-label 의 탭·줄바꿈은 문자 참조로(공통 — state 와 같다)
}

// chooseTicks 는 눈금 시각을 고른다(격자선 = 라벨). tickInterval 이 있으면 그 간격 그대로 — 눈금이 MaxTicks 를 넘거나
// 라벨이 겹치면 경고하고 자동 눈금으로 그린다. 자동은 사다리(ladder)에서 목표 눈금 수(축 폭 ÷ (라벨 폭 + labGap), 최소 2)에
// 가장 가까운 칸부터, 눈금이 상한 안이고 라벨이 겹치지 않는 첫 칸이다. 눈금은 실제 범위 rlo..rhi(여백 없이 — 모든 작업
// 가운데 가장 이른 시작부터 가장 늦은 끝까지, 행 차례와 무관) 안에 선다 — 첫 눈금은 rlo 를 간격 경계로 올린 시각이다.
func chooseTicks(c *chart, rlo, rhi time.Time, fits func([]time.Time) bool) ([]time.Time, []Issue, error) {
	var issues []Issue
	if c.HasTick {
		ts, err := genTicks(rlo, rhi, c.Tick)
		if err != nil {
			return nil, nil, &Error{Line: c.TickLine, Msg: err.Error()}
		}
		switch {
		case len(ts) > MaxTicks:
			issues = append(issues, Issue{Line: c.TickLine, Msg: fmt.Sprintf("tickInterval %s 의 눈금이 %d 개를 넘어 자동 눈금으로 그림", c.Tick, MaxTicks)})
		case !fits(ts):
			issues = append(issues, Issue{Line: c.TickLine, Msg: fmt.Sprintf("tickInterval %s 의 눈금 라벨이 겹쳐 자동 눈금으로 그림", c.Tick)})
		default:
			return ts, nil, nil
		}
	}
	minutes := c.DF.has("mm") || c.AF.has('M')
	hours := minutes || c.DF.has("HH") || c.AF.has('H')
	lad := ladder(minutes, hours)
	span := rhi.Sub(rlo)
	if span <= 0 {
		span = 24 * time.Hour
	}
	want := max(2, int(axisW/(diagram.TextW(c.AF.format(rlo), small)+labGap)))
	for k := nearest(lad, span/time.Duration(want)); k < len(lad); k++ {
		ts, err := genTicks(rlo, rhi, lad[k])
		if err != nil {
			return nil, nil, &Error{Msg: err.Error()}
		}
		if len(ts) <= MaxTicks && fits(ts) {
			return ts, issues, nil
		}
	}
	// 사다리의 마지막 칸(100년)까지 라벨이 겹치면 그리지 않는다(원문 + 경고) — 겹친 라벨을 그린 채 성공으로 돌려주지 않는다.
	// 줄은 axisFormat 줄(없으면 머리 줄). 성공으로 나가는 길은 위 두 곳뿐이고 둘 다 fits·상한을 지난 눈금이다.
	return nil, nil, &Error{Line: c.AxisLine, Msg: "축 라벨이 겹쳐 그리지 않음 — axisFormat 을 줄인다"}
}
