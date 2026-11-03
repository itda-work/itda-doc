package mdoc

import (
	"fmt"
	"math"
	"sort"

	"github.com/yuin/goldmark/ast"
)

// 산점도·버블(1.11, itda-hyve#114 단계 D). 표 꼴은 라벨 | x | y [| 크기] 이다: 첫 열은 점 라벨, 둘째·셋째 열이 x·y, 버블의 넷째 열이 크기다.
// 축 제목은 그 열의 머리 칸이다. 항목이 같은 간격으로 놓이는 drawXY 와 달리 x 도 연속 축이라 데이터 모델과 그리기를 따로 둔다.
// v1 은 점 집합 하나(색 s1), 선으로 잇지 않는다.

// 상한(실측, 설계 1.11): 산점도 점 200 · 버블 40. 넘으면 앞의 것만 그리고 경고한다. 산점도 200점(반지름 2.5)은 분포가 읽히고
// SVG 가 약 28KB 다. 버블은 고르게 흩어진 40개에서 라벨 36개가 자리를 얻었지만, 몰린 60개는 한 덩어리가 되어 라벨이 22개뿐이었다.
const (
	chartMaxPoints  = 200
	chartMaxBubbles = 40
)

// 버블 반지름: 가장 큰 크기가 bubbleRMax, 넓이가 크기에 비례한다(r = bubbleRMax·√(v/최대)). bubbleRMin 보다 작으면 그만큼으로 그린다 —
// 최대의 약 0.5% 보다 작은 값만 해당하고, 보이지 않게 되는 것을 막는다(그 값들의 넓이 비례는 깨진다).
const (
	bubbleRMax = 22.0
	bubbleRMin = 1.5
)

// chartScatter 는 산점도·버블의 데이터다.
type chartScatter struct {
	Cols   [3]chartCol // x · y · 크기(버블만)
	Points []chartPoint
}

// chartCol 은 산점도의 한 열(축)이다: 이름(머리 칸 — 축 제목), 단위(값이 모두 % 면 "%"), 값 글자의 소수 자리(3 까지).
type chartCol struct {
	Name     string
	Unit     string
	Decimals int
}

type chartPoint struct {
	Label string
	V     [3]float64 // x · y · 크기
	Row   int        // 표의 행 차례(0 부터) — 움직임 차례
}

// colText 는 열 k 의 값 글자다(단위가 % 면 붙인다).
func (sc *chartScatter) colText(k int, v float64) string {
	s := fmtVal(v, sc.Cols[k].Decimals)
	if sc.Cols[k].Unit == "%" {
		s += "%"
	}
	return s
}

// readScatter 는 산점도·버블의 표를 읽는다. 열이 모자라거나 그릴 점이 없으면 경고하고 거짓 — 표 그대로다.
// x·y(버블은 크기도) 중 하나라도 빈 행은 뺀다(경고 없음 — 빈 칸은 빈 값이다). 읽지 못한 칸은 경고하고 빈 값으로 둔다.
// 버블의 크기가 음수면 경고하고 빼고, 0 이면 넓이가 없으므로 경고 없이 뺀다(원 조각의 0 과 같다).
func (st *state) readScatter(ch *Chart, line int, header []ast.Node, rows [][]ast.Node) bool {
	typ := ch.ChartType
	need, limit, cols := 3, chartMaxPoints, "라벨·x·y 세 열"
	if typ == "bubble" {
		need, limit, cols = 4, chartMaxBubbles, "라벨·x·y·크기 네 열"
	}
	if len(header) < need {
		st.warn(line, WarnChart, fmt.Sprintf("%s 은 %s이 있어야 해 차트를 그리지 않고 표로 둠", typ, cols))
		return false
	}
	if len(header) > need {
		st.warn(line, WarnChart, fmt.Sprintf("%s 은 %s만 써 %d 번째 열부터는 그리지 않음", typ, cols, need+1))
	}
	if ch.Unit != "" {
		// 축이 둘(버블은 셋)이라 단위 하나로 정해지지 않는다 — 단위는 열 머리글(축 제목)에 적는다
		st.warn(line, WarnChart, fmt.Sprintf("%s 에는 unit 을 쓰지 않아 무시함(단위는 열 머리글에 — 예: 매출(억 원))", typ))
		ch.Unit = ""
	}
	if len(rows) > limit {
		st.warn(line, WarnChart, fmt.Sprintf("점이 %d 개라 앞의 %d 개만 그림", len(rows), limit))
		rows = rows[:limit]
	}
	sc := &chartScatter{}
	nc := need - 1
	for k := 0; k < nc; k++ {
		sc.Cols[k].Name = cellText(header[k+1], st.src)
	}
	var pct, plain [3]int
	var pctEx, plainEx [3]string
	for ri, row := range rows {
		p := chartPoint{Row: ri}
		if len(row) > 0 {
			p.Label = cellText(row[0], st.src)
		}
		ok := true
		for k := 0; k < nc; k++ {
			raw := ""
			if k+1 < len(row) {
				raw = cellText(row[k+1], st.src)
			}
			v, dec, isPct, good, empty := parseChartNumber(raw, "")
			if !good {
				if !empty {
					st.warn(st.nodeLine(row[k+1], line), WarnChart, fmt.Sprintf("차트 값 %q 를 숫자로 읽지 못해 빈 값으로 둠", clip(raw)))
				}
				ok = false
				continue
			}
			sc.Cols[k].Decimals = max(sc.Cols[k].Decimals, min(dec, 3))
			if isPct {
				pct[k]++
				if pctEx[k] == "" {
					pctEx[k] = raw
				}
			} else {
				plain[k]++
				if plainEx[k] == "" {
					plainEx[k] = raw
				}
			}
			p.V[k] = v
		}
		if !ok {
			continue // 한 좌표라도 비면 그 점은 없다
		}
		if typ == "bubble" && p.V[2] < 0 {
			st.warn(line, WarnChart, fmt.Sprintf("버블 크기는 음수를 그리지 못해 %q 를 뺌", clip(p.Label)))
			continue
		}
		if typ == "bubble" && p.V[2] == 0 {
			continue // 넓이 0 — 그릴 것이 없다
		}
		sc.Points = append(sc.Points, p)
	}
	// 열마다 % 추론과 단위 혼합 경고(4장의 규칙을 열마다)
	for k := 0; k < nc; k++ {
		switch {
		case pct[k] > 0 && plain[k] == 0:
			sc.Cols[k].Unit = "%"
		case pct[k] > 0:
			st.warn(line, WarnChart, fmt.Sprintf("%s 열(%s)의 값에 %% 를 붙인 칸(%q)과 붙이지 않은 칸(%q)이 섞여 단위 없이 그림(%% 를 모두 붙인다)",
				[3]string{"x", "y", "크기"}[k], clip(sc.Cols[k].Name), clip(pctEx[k]), clip(plainEx[k])))
		}
	}
	if len(sc.Points) == 0 {
		st.warn(line, WarnChart, fmt.Sprintf(":::chart 의 표에 %s 를 모두 읽을 수 있는 행이 없어 차트를 그리지 않음", map[bool]string{true: "x·y·크기", false: "x·y"}[typ == "bubble"]))
		return false
	}
	ch.Scatter = sc
	return true
}

// tipPoint 는 점의 <title> 이다("본점 · 매출: 120 · 이익: 14 · 직원 수: 35").
func (sc *chartScatter) tipPoint(p chartPoint, bubble bool) string {
	t := ""
	if p.Label != "" {
		t = p.Label + " · "
	}
	n := 2
	if bubble {
		n = 3
	}
	for k := 0; k < n; k++ {
		if k > 0 {
			t += " · "
		}
		name := sc.Cols[k].Name
		if name == "" {
			name = [3]string{"x", "y", "크기"}[k]
		}
		t += name + ": " + sc.colText(k, p.V[k])
	}
	return t
}

// bubbleR 는 크기 v 의 반지름이다 — 넓이가 크기에 비례(최대 vmax 가 bubbleRMax).
func bubbleR(v, vmax float64) float64 {
	if !(vmax > 0) || !(v > 0) {
		return bubbleRMin
	}
	return math.Max(bubbleRMin, bubbleRMax*math.Sqrt(v/vmax))
}

// drawScatter 는 산점도·버블이다. x·y 모두 연속 축(보기 좋은 눈금)이고, 0 포함은 꺾은선의 규칙(0 에 가까우면 0 부터)이다.
// 버블은 원이 그림 영역 밖으로 나가지 않도록 그쪽 눈금을 한 칸씩 넓힌다. 점 라벨은 겹치지 않는 자리(오른쪽·왼쪽·위·아래)에만 적는다.
func drawScatter(ch *Chart) string {
	sc := ch.Scatter
	bubble := ch.ChartType == "bubble"
	pts := sc.Points
	n := len(pts)

	// 범위
	var lo, hi [2]float64
	for k := 0; k < 2; k++ {
		lo[k], hi[k] = math.Inf(1), math.Inf(-1)
		for _, p := range pts {
			lo[k], hi[k] = math.Min(lo[k], p.V[k]), math.Max(hi[k], p.V[k])
		}
		if lo[k] > 0 && lo[k] <= hi[k]*0.5 || hi[k] < 0 && hi[k] >= lo[k]*0.5 {
			lo[k], hi[k] = math.Min(lo[k], 0), math.Max(hi[k], 0)
		}
	}
	vmax := 0.0
	if bubble {
		for _, p := range pts {
			vmax = math.Max(vmax, p.V[2])
		}
	}
	rOf := func(p chartPoint) float64 {
		if bubble {
			return bubbleR(p.V[2], vmax)
		}
		if n > 60 {
			return 2.5
		}
		return 3.5
	}

	// 머리 줄: 왼쪽에 y 축 제목, 버블은 오른쪽에 크기 범례(가장 큰 값과 그 4분의 1 쯤의 보기 좋은 값)
	headH := 20.0
	var legend svgw
	legW := 0.0
	if bubble {
		headH = 2*bubbleRMax + 8
		base := headH - 2
		// 작은 원은 최대의 4분의 1 이상 2분의 1 이하인 보기 좋은 값(1·2·2.5·5 × 10ⁿ) — 그 값의 자리로 적는다(2.5 를 "3" 으로 반올림하지 않게)
		q := niceUp(vmax / 4)
		qt := fmtVal(q, max(sc.Cols[2].Decimals, stepDec(q)))
		if sc.Cols[2].Unit == "%" {
			qt += "%"
		}
		type it struct {
			r float64
			t string
		}
		items := []it{{bubbleRMax, sc.colText(2, vmax)}, {bubbleR(q, vmax), qt}}
		for _, x := range items {
			legW += 2*x.r + 4 + textW(x.t, fsAxis) + 12
		}
		// 이름은 원·값 글자가 남긴 자리 안으로(긴 값 둘이면 이름이 줄어든다 — 범례가 그림 너비를 넘지 않게)
		name := fitText("넓이: "+sc.Cols[2].Name, fsLegend, math.Max(30, math.Min(170, cvW-legW-10)))
		legW += textW(name, fsLegend) + 10
		x := cvW - legW
		legend.open("g", "class", "legend")
		legend.label(x, base-4, fsLegend, "start", "lg", name)
		x += textW(name, fsLegend) + 10
		for _, it := range items {
			legend.leaf("circle", "", "cx", f1(x+it.r), "cy", f1(base-it.r), "r", f1(it.r), "class", "sw bub s1")
			x += 2*it.r + 4
			legend.label(x, base-4, fsAxis, "start", "tick", it.t)
			x += textW(it.t, fsAxis) + 12
		}
		legend.close("g")
	}
	// y 축 제목은 범례 왼쪽에 들면 그 줄에, 들지 않으면 범례 아래 줄에 따로 둔다(리뷰 R1 — 긴 범례와 겹쳤다)
	yTitle := sc.Cols[1].Name
	if legW > 0 && textW(yTitle, fsAxis)+16 > cvW-legW {
		headH += 18
	}
	yTitle = fitText(yTitle, fsAxis, cvW)

	const plotH = 230.0
	y0 := headH + 10
	y1 := y0 + plotH
	var t [2]struct {
		lo, hi, step float64
		dec          int
	}
	for k := 0; k < 2; k++ {
		t[k].lo, t[k].hi, t[k].step, t[k].dec = niceTicks(lo[k], hi[k])
	}
	tickText := func(k int, v float64) string {
		s := fmtNum(v, t[k].dec)
		if sc.Cols[k].Unit == "%" {
			s += "%"
		}
		return s
	}
	var x0, x1 float64
	xv := func(v float64) float64 { return x0 + (v-t[0].lo)/(t[0].hi-t[0].lo)*(x1-x0) }
	yv := func(v float64) float64 { return y1 - (v-t[1].lo)/(t[1].hi-t[1].lo)*(y1-y0) }
	layout := func() {
		tickW := 0.0
		for v := t[1].lo; v <= t[1].hi+t[1].step/2; v += t[1].step {
			tickW = math.Max(tickW, textW(tickText(1, v), fsAxis))
		}
		// 왼쪽: y 눈금 글자, 그리고 가운데 맞춤인 첫 x 눈금 글자의 반(리뷰 R1 — 큰 x 와 작은 y 에서 그림 왼쪽 밖으로 나갔다)
		x0 = math.Max(tickW+10, textW(tickText(0, t[0].lo), fsAxis)/2+2)
		x1 = cvW - math.Max(6, textW(tickText(0, t[0].hi), fsAxis)/2+2)
	}
	layout()
	if bubble {
		// 원이 그림 영역 밖으로 나가면 그쪽 눈금을 한 칸 넓힌다(넓히면 비율이 줄어 몇 번이면 든다)
		for it := 0; it < 6; it++ {
			var l, r, b, tp bool
			for _, p := range pts {
				rr := rOf(p)
				cx, cy := xv(p.V[0]), yv(p.V[1])
				l = l || cx-rr < x0-0.01
				r = r || cx+rr > x1+0.01
				b = b || cy+rr > y1+0.01
				tp = tp || cy-rr < y0-0.01
			}
			if !(l || r || b || tp) {
				break
			}
			for _, g := range []struct {
				on  bool
				at  *float64
				dir float64
				k   int
			}{{l, &t[0].lo, -1, 0}, {r, &t[0].hi, 1, 0}, {b, &t[1].lo, -1, 1}, {tp, &t[1].hi, 1, 1}} {
				if g.on {
					*g.at += g.dir * t[g.k].step
				}
			}
			layout()
		}
	}
	h := y1 + 44

	var s svgw
	svgOpen(&s, ch, h)
	s.b.WriteString(legend.b.String())
	s.label(0, headH-4, fsAxis, "start", "unit axt", yTitle)
	// 눈금·격자: 가로선(y 눈금)·세로선(x 눈금)
	s.open("g", "class", "grid")
	for v := t[1].lo; v <= t[1].hi+t[1].step/2; v += t[1].step {
		y := yv(v)
		cls := "gl"
		if math.Abs(v) < t[1].step/1e6 {
			cls = "gl zero"
		}
		s.leaf("line", "", "x1", f1(x0), "y1", f1(y), "x2", f1(x1), "y2", f1(y), "class", cls)
		s.label(x0-6, y+4, fsAxis, "end", "tick", tickText(1, v))
	}
	nx := int(math.Round((t[0].hi-t[0].lo)/t[0].step)) + 1
	every := 1
	if nx > 1 {
		gap := (x1 - x0) / float64(nx-1)
		maxW := 0.0
		for k := 0; k < nx; k++ {
			maxW = math.Max(maxW, textW(tickText(0, t[0].lo+float64(k)*t[0].step), fsAxis))
		}
		every = max(1, int(math.Ceil((maxW+8)/gap)))
	}
	for k := 0; k < nx; k++ {
		v := t[0].lo + float64(k)*t[0].step
		x := xv(v)
		cls := "gl"
		if math.Abs(v) < t[0].step/1e6 {
			cls = "gl zero"
		}
		s.leaf("line", "", "x1", f1(x), "y1", f1(y0), "x2", f1(x), "y2", f1(y1), "class", cls)
		if k%every == 0 {
			s.label(x, y1+16, fsAxis, "middle", "tick", tickText(0, v))
		}
	}
	s.close("g")

	// 점: 버블은 큰 것부터(작은 원이 큰 원에 가리지 않게)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	if bubble {
		sort.SliceStable(order, func(a, b int) bool { return pts[order[a]].V[2] > pts[order[b]].V[2] })
	}
	cls := "dot sc s1 "
	if bubble {
		cls = "dot bub s1 "
	}
	s.open("g", "class", "marks")
	for _, i := range order {
		p := pts[i]
		s.leaf("circle", sc.tipPoint(p, bubble), "cx", f1(xv(p.V[0])), "cy", f1(yv(p.V[1])), "r", f1(rOf(p)), "class", cls+delay(p.Row))
	}
	s.close("g")

	// 점 라벨: 그림 영역 안에서 다른 라벨·점 가운데와 겹치지 않는 첫 자리(오른쪽 → 왼쪽 → 위 → 아래), 없으면 적지 않는다(<title> 에는 있다)
	if ch.Labels {
		var obst [][4]float64
		for _, p := range pts {
			ro := math.Min(rOf(p), 5) + 1
			cx, cy := xv(p.V[0]), yv(p.V[1])
			obst = append(obst, [4]float64{cx - ro, cy - ro, cx + ro, cy + ro})
		}
		var placed []valLab
		var placedIdx []int
		for _, i := range order {
			p := pts[i]
			if p.Label == "" {
				continue
			}
			lab := fitText(p.Label, fsVal, 110)
			r := rOf(p)
			cx, cy := xv(p.V[0]), yv(p.V[1])
			for _, c := range []valLab{
				{cx + r + 3, cy + fsVal*0.36, "start", "", lab},
				{cx - r - 3, cy + fsVal*0.36, "end", "", lab},
				{cx, cy - r - 4, "middle", "", lab},
				{cx, cy + r + fsVal + 2, "middle", "", lab},
			} {
				b := c.box()
				if b[0] < x0+1 || b[2] > cvW || b[1] < y0-6 || b[3] > y1-1 {
					continue
				}
				hit := false
				for j, o := range obst {
					if j != i && overlaps(b, o, 0) {
						hit = true
						break
					}
				}
				for _, q := range placed {
					if !hit && overlaps(b, q.box(), 2) {
						hit = true
					}
				}
				if hit {
					continue
				}
				c.class = "val plab " + delay(p.Row)
				placed = append(placed, c)
				placedIdx = append(placedIdx, i)
				break
			}
		}
		if len(placed) > 0 {
			s.open("g", "class", "vals")
			for k, l := range placed {
				p := pts[placedIdx[k]]
				s.open("text", "x", f1(l.x), "y", f1(l.y), "font-size", f1(fsVal), "text-anchor", l.anchor, "class", l.class)
				s.text(l.t)
				if l.t != p.Label {
					s.title(p.Label)
				}
				s.close("text")
			}
			s.close("g")
		}
	}

	// x 축 제목(가운데, 눈금 아래)
	s.label((x0+x1)/2, y1+36, fsAxis, "middle", "unit axt", fitText(sc.Cols[0].Name, fsAxis, x1-x0))
	s.close("svg")
	return s.b.String()
}
