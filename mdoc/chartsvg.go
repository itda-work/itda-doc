package mdoc

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// 차트를 인라인 SVG 로 그린다(1.11). 쓰는 요소·속성은 아래 svgw 가 내는 것뿐이다(보안 테스트가 허용 목록으로 고정한다):
//   요소 svg·g·rect·path·line·circle·text·tspan·title
//   속성 viewBox·class·role·aria-label·x·y·width·height·rx·d·x1·y1·x2·y2·cx·cy·r·text-anchor·font-size·font-weight·pathLength
// style·이벤트 속성·url()·외부 참조(href)를 쓰지 않는다. 글자는 모두 이스케이프한다. 색은 클래스(s1…s8)로 테마 토큰을 따른다.
//
// 좌표는 너비 600 의 viewBox 다. SVG 는 너비 100% 로 늘어나므로 A4(본문 약 166mm)에서 글자 12 ≈ 9.4pt, 슬라이드에서는
// 비례해 커진다. 글자 너비는 잴 수 없어 어림한다(한글 1em, 숫자·영문 약 0.58em).
//
// 움직임을 위한 표지(클래스): 막대 col(음수는 neg — app.css 의 .bar 와 겹치지 않게)·쌓은 묶음 stack·선 line(pathLength=1 — 혼합 차트의 선 테두리
// line halo 도 선과 함께 그려진다)·점 dot·면 area·원 조각 slice·값 val, 혼합 차트의 선 계열(선·점·범례 표식) cl,
// 항목 차례 d0…d15, 보조 축의 오른쪽 눈금 tick ax2(움직이지 않는다), 기준선 ref(점선 ref-line·라벨 ref-lab — 움직이지 않는다, 점선은 base.css 의 .ref-line 이 정하고 움직임의 .line 과 겹치지 않는다).
// 화면(app.css)이 이 클래스로 움직이고, 인쇄·PDF·움직임 줄이기에서는 아무것도 하지 않는다(다 그려진 모습).

const (
	cvW      = 600.0
	fsAxis   = 12.0 // 눈금·항목 글자
	fsVal    = 11.0 // 값 글자
	fsLegend = 12.0
)

func (r *htmlRenderer) chart(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	ch := node.(*Chart)
	_, _ = w.WriteString("<figure")
	cls := []string{"chart", "chart-" + ch.ChartType}
	cls = append(cls, ch.Classes...)
	attr(w, "class", strings.Join(cls, " "))
	if ch.ID != "" {
		attr(w, "id", ch.ID)
	}
	dataAttrs(w, ch)
	_, _ = w.WriteString(">\n")
	_, _ = w.WriteString(drawChart(ch))
	if ch.Caption != "" {
		_, _ = w.WriteString("\n<figcaption>")
		_, _ = w.Write(util.EscapeHTML([]byte(ch.Caption)))
		_, _ = w.WriteString("</figcaption>")
	}
	_, _ = w.WriteString("\n</figure>\n")
	return ast.WalkContinue, nil // 자식(table=below 의 표, 설명 문단)은 그림 뒤에 그대로
}

// ---- SVG 쓰개

type svgw struct{ b strings.Builder }

func (s *svgw) open(tag string, kv ...string) {
	s.b.WriteString("<" + tag)
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" && kv[i] != "aria-label" {
			continue
		}
		s.b.WriteString(" " + kv[i] + `="`)
		s.b.Write(util.EscapeHTML([]byte(kv[i+1])))
		s.b.WriteByte('"')
	}
	s.b.WriteByte('>')
}
func (s *svgw) close(tag string) { s.b.WriteString("</" + tag + ">") }
func (s *svgw) text(t string)    { s.b.Write(util.EscapeHTML([]byte(t))) }
func (s *svgw) title(t string) {
	s.b.WriteString("<title>")
	s.text(t)
	s.b.WriteString("</title>")
}

// leaf 는 자식 없는 요소다. tip 이 있으면 <title>(마우스를 올리면 보이는 값)을 단다.
func (s *svgw) leaf(tag, tip string, kv ...string) {
	s.open(tag, kv...)
	if tip != "" {
		s.title(tip)
	}
	s.close(tag)
}

func (s *svgw) label(x, y, size float64, anchor, class, t string) {
	s.open("text", "x", f1(x), "y", f1(y), "font-size", f1(size), "text-anchor", anchor, "class", class)
	s.text(t)
	s.close("text")
}

func f1(v float64) string {
	v = math.Round(v*10) / 10
	if v == 0 {
		v = 0 // -0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// ---- 글자

// textW 는 글자 너비 어림이다(px, 글자 크기 size).
func textW(s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		switch {
		case r == ' ':
			w += 0.28
		case r >= 0x1100 && (unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || (r >= 0xff00 && r <= 0xffef)):
			w += 0.98
		case r >= '0' && r <= '9':
			w += 0.58
		case r >= 'A' && r <= 'Z', r == 'W', r == 'M', r == '%':
			w += 0.66
		case r >= 'a' && r <= 'z':
			w += 0.54
		case r == '.' || r == ',' || r == ':' || r == ';' || r == '·' || r == '\'' || r == '|':
			w += 0.3
		default:
			w += 0.6
		}
	}
	return w * size
}

// textRight 는 값 글자(fsVal)의 오른쪽 끝 어림이다(text-anchor 에 따라).
func textRight(x float64, anchor, t string) float64 {
	switch w := textW(t, fsVal); anchor {
	case "start":
		return x + w
	case "middle":
		return x + w/2
	}
	return x
}

// fitText 는 너비 width 에 들도록 뒤를 …로 줄인다.
func fitText(s string, size, width float64) string {
	if textW(s, size) <= width {
		return s
	}
	r := []rune(s)
	for n := len(r) - 1; n > 0; n-- {
		t := strings.TrimSpace(string(r[:n])) + "…"
		if textW(t, size) <= width {
			return t
		}
	}
	return "…"
}

// wrap2 는 두 줄로 나눈다(공백이 있으면 가운데에 가까운 공백에서, 없으면 글자 수 절반에서). 줄마다 너비에 맞게 줄인다.
func wrap2(s string, size, width float64) []string {
	if textW(s, size) <= width {
		return []string{s}
	}
	r := []rune(s)
	cut, best := -1, math.MaxFloat64
	for i, c := range r {
		if c == ' ' {
			if d := math.Abs(textW(string(r[:i]), size) - textW(string(r[i+1:]), size)); d < best {
				cut, best = i, d
			}
		}
	}
	var a, b string
	if cut > 0 {
		a, b = string(r[:cut]), string(r[cut+1:])
	} else {
		h := len(r) / 2
		for h < len(r) && textW(string(r[:h]), size) < textW(s, size)/2 {
			h++
		}
		a, b = string(r[:h]), string(r[h:])
	}
	return []string{fitText(a, size, width), fitText(b, size, width)}
}

// ---- 숫자

func fmtNum(v float64, dec int) string {
	neg := v < 0
	s := strconv.FormatFloat(math.Abs(v), 'f', dec, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

// fmtVal 은 값 글자다: 소수 자리 dec 로 적되, 0 이 아닌 값이 0 으로 보이면(0.0004 를 3자리로 "0.000") 첫 유효 숫자가 보일 때까지
// 그 값만 자리를 늘린다(6 까지) — 다른 값의 자리는 그대로다(itda-hyve#114 함께 볼 것: 표시 정밀도).
func fmtVal(v float64, dec int) string {
	s := fmtNum(v, dec)
	for d := dec + 1; v != 0 && d <= 6 && strings.Trim(s, "-0.,") == ""; d++ {
		s = fmtNum(v, d)
	}
	return s
}

func (ch *Chart) valText(v float64) string { return ch.valTextAx(0, v) }

// valTextAx 는 축 ax(0 왼쪽·1 오른쪽)의 단위·소수 자리로 적은 값 글자다.
func (ch *Chart) valTextAx(ax int, v float64) string {
	unit, dec := ch.unitOf(ax)
	s := fmtVal(v, dec)
	if unit == "%" {
		s += "%"
	}
	return s
}

func (ch *Chart) tip(cat, series string, v float64) string { return ch.tipAx(cat, series, 0, v) }

// tipAx 는 축 ax 의 단위로 적은 <title> 이다("1분기 · 이익률: 8.2%").
func (ch *Chart) tipAx(cat, series string, ax int, v float64) string {
	unit, dec := ch.unitOf(ax)
	t := cat
	if series != "" && (len(ch.Series) > 1 || ch.ChartType == "pie" || ch.ChartType == "donut") && series != cat {
		t += " · " + series
	}
	t += ": " + fmtVal(v, dec)
	switch {
	case unit == "%":
		t += "%"
	case unit != "":
		t += " " + unit
	}
	return t
}

// refText 는 기준선 라벨이다("목표 40" — 라벨이 없으면 값만). 소수 자리는 값 글자와 ref 에 적힌 것 중 긴 쪽.
func (ch *Chart) refText() string {
	v := fmtVal(ch.Ref.Value, max(ch.Decimals, ch.Ref.Decimals))
	if ch.Unit == "%" {
		v += "%"
	}
	if ch.Ref.Label != "" {
		return ch.Ref.Label + " " + v
	}
	return v
}

// refTip 은 기준선의 <title> 이다("목표: 40 억 원").
func (ch *Chart) refTip() string {
	name := ch.Ref.Label
	if name == "" {
		name = "기준선"
	}
	t := name + ": " + fmtVal(ch.Ref.Value, max(ch.Decimals, ch.Ref.Decimals))
	switch {
	case ch.Unit == "%":
		t += "%"
	case ch.Unit != "":
		t += " " + ch.Unit
	}
	return t
}

// valsLayer 는 값 글자를 쓸 곳이다. 기준선이 없으면 그림 그대로(막대 곁), 있으면 따로 모아 기준선 뒤에 낸다 —
// 점선이 글자를 가로지르지 않게(글자에는 종이색 테두리를 base.css 의 .vals 가 준다). 혼합 차트(막대 + 선)도 같다 — 선이 막대의
// 값 글자를 가로지르지 않게. 기준선도 선 계열도 없는 차트의 SVG 는 바뀌지 않는다.
func valsLayer(s *svgw, ch *Chart) (vs, late *svgw) {
	if ch.Ref == nil && ch.Line < 0 {
		return s, nil
	}
	late = &svgw{}
	return late, late
}

func flushVals(s *svgw, late *svgw) {
	if late == nil || late.b.Len() == 0 {
		return
	}
	s.open("g", "class", "vals")
	s.b.WriteString(late.b.String())
	s.close("g")
}

// refGroup 은 기준선이다: 종이색 테두리 선(ref-halo — 같은 색 막대 위에서도 점선이 보이게) 위에 점선, 그리고 라벨.
func refGroup(s *svgw, ch *Chart, x1, y1, x2, y2, lx, ly float64, anchor, lab string) {
	s.open("g", "class", "ref")
	s.leaf("line", "", "x1", f1(x1), "y1", f1(y1), "x2", f1(x2), "y2", f1(y2), "class", "ref-halo")
	s.leaf("line", ch.refTip(), "x1", f1(x1), "y1", f1(y1), "x2", f1(x2), "y2", f1(y2), "class", "ref-line")
	s.label(lx, ly, fsVal, anchor, "ref-lab", lab)
	s.close("g")
}

// niceTicks 는 [lo, hi] 를 덮는 보기 좋은 눈금(1·2·2.5·5 × 10^k 간격, 4~6칸)이다.
func niceTicks(lo, hi float64) (tlo, thi, step float64, dec int) {
	if hi == lo {
		if hi == 0 {
			hi = 1
		} else if hi > 0 {
			lo = 0
		} else {
			hi = 0
		}
	}
	raw := (hi - lo) / 5
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		step = m * mag
		if (hi-lo)/step <= 6 {
			break
		}
	}
	tlo = math.Floor(lo/step+1e-9) * step
	thi = math.Ceil(hi/step-1e-9) * step
	if thi == tlo {
		thi = tlo + step
	}
	return tlo, thi, step, stepDec(step)
}

// stepDec 는 눈금 간격의 소수 자리다(2.5 → 1, 0.25 → 2).
func stepDec(step float64) int {
	for d := 0; d < 6; d++ {
		p := math.Pow(10, float64(d))
		if math.Abs(step*p-math.Round(step*p)) < 1e-6 {
			return d
		}
	}
	return 6
}

// niceUp 은 x 이상인 가장 작은 보기 좋은 간격(1·2·2.5·5 × 10^k)이다.
func niceUp(x float64) float64 {
	if !(x > 0) {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(x)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if m*mag >= x*(1-1e-9) {
			return m * mag
		}
	}
	return 10 * mag
}

// alignTicks 는 오른쪽 축(보조 축)의 눈금이다. 격자선은 왼쪽 축과 함께 쓰므로 칸 수 n 이 같다 — 오른쪽 눈금 글자는 왼쪽 격자선
// 높이에 놓인다. 두 축이 모두 0 을 지나면 0 을 같은 격자선(왼쪽 0 의 칸 k0)에 맞춘다 — 막대와 선의 0 이 같은 높이다. 다만 그러려고
// 간격이 따로 고를 때의 두 배를 넘으면(선이 그림의 절반도 못 쓰면) 맞추지 않는다. [lo, hi] 는 0 포함 규칙을 적용한 범위다.
func alignTicks(n, k0 int, lo, hi float64) (tlo, step float64, dec int) {
	if hi == lo {
		switch {
		case hi == 0:
			hi = 1
		case hi > 0:
			lo = 0
		default:
			hi = 0
		}
	}
	free := niceUp((hi - lo) / float64(n))
	if k0 >= 0 && lo <= 0 && hi >= 0 && (k0 > 0 || lo == 0) && (k0 < n || hi == 0) {
		need := 0.0
		if k0 > 0 {
			need = -lo / float64(k0)
		}
		if k0 < n {
			need = math.Max(need, hi/float64(n-k0))
		}
		if step = niceUp(need); step <= 2*free {
			return -float64(k0) * step, step, stepDec(step)
		}
	}
	step = free
	for i := 0; i < 40; i++ {
		tlo = math.Floor(lo/step+1e-9) * step
		if tlo+float64(n)*step >= hi-step*1e-9 {
			break
		}
		step = niceUp(step * 1.0001) // 다음 간격
	}
	return tlo, step, stepDec(step)
}

func delay(i int) string { return "d" + strconv.Itoa(min(i, 15)) }

// ---- 그리기

func drawChart(ch *Chart) string {
	switch ch.ChartType {
	case "pie", "donut":
		return drawPie(ch)
	case "hbar":
		return drawHBar(ch)
	case "scatter", "bubble":
		return drawScatter(ch)
	}
	return drawXY(ch)
}

func svgOpen(s *svgw, ch *Chart, h float64) { svgOpenW(s, ch, cvW, h) }

func svgOpenW(s *svgw, ch *Chart, w, h float64) {
	label := ch.Caption
	if label == "" {
		label = "차트"
	}
	s.open("svg", "class", "chart-svg t-"+ch.ChartType, "viewBox", "0 0 "+f1(w)+" "+f1(h), "role", "img", "aria-label", label)
}

// header 는 위쪽 줄: 왼쪽에 단위, 오른쪽에 범례(계열이 둘 이상일 때). 높이를 돌려준다.
func header(s *svgw, ch *Chart, top float64) float64 {
	unit := ""
	if ch.Unit != "" && ch.Unit != "%" {
		unit = "단위: " + ch.Unit
	}
	if ch.Axis2 >= 0 && ch.Unit2 != "" && ch.Unit2 != "%" {
		// 오른쪽 축의 단위(% 는 눈금 글자에 붙는다)
		if unit != "" {
			unit += " · "
		}
		unit += "오른쪽 축 단위: " + ch.Unit2
	}
	var names []string
	if len(ch.Series) > 1 {
		for _, se := range ch.Series {
			names = append(names, se.Name)
		}
	}
	if unit == "" && len(names) == 0 {
		return 0
	}
	unitW := 0.0
	if unit != "" {
		unit = fitText(unit, fsAxis, cvW) // 두 축의 단위가 모두 길면 한 줄을 넘는다
		unitW = textW(unit, fsAxis) + 18
	}
	// 범례 줄 나누기: 오른쪽 끝에서부터, 한 줄에 들지 않으면 다음 줄
	type item struct {
		name string
		w    float64
		line bool // 혼합 차트의 선 계열 — 표식이 사각형이 아니라 선과 점
	}
	var rows [][]item
	avail := cvW - unitW
	cur, curW := []item{}, 0.0
	for k, n := range names {
		n = fitText(n, fsLegend, 160)
		if k == ch.Axis2 {
			n += " (오른쪽 축)" // 오른쪽 축 계열은 범례에 글자로도 적는다 — 색만으로 어느 축인지 가리지 않게
		}
		iw := 14 + textW(n, fsLegend)
		if k == ch.Line {
			iw += 6
		}
		if len(cur) > 0 && curW+16+iw > avail || len(rows) == 0 && len(cur) == 0 && iw > avail && unitW > 0 {
			// 첫 항목도 단위 줄 곁에 들지 않으면 첫 줄은 단위만 두고 범례는 다음 줄부터(리뷰 R1 — 단계 C 의 두 단위·"(오른쪽 축)" 이 길다)
			rows = append(rows, cur)
			cur, curW, avail = nil, 0, cvW
		}
		if len(cur) > 0 {
			curW += 16
		}
		cur = append(cur, item{n, iw, k == ch.Line})
		curW += iw
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}
	if unit != "" {
		s.label(0, top+12, fsAxis, "start", "unit", unit)
	}
	si := 0
	s.open("g", "class", "legend")
	for ri, row := range rows {
		w := 0.0
		for i, it := range row {
			if i > 0 {
				w += 16
			}
			w += it.w
		}
		x := cvW - w
		y := top + float64(ri)*20
		for _, it := range row {
			si++
			if it.line {
				s.leaf("rect", "", "x", f1(x), "y", f1(y+6.8), "width", "16", "height", "2.4", "rx", "1.2", "class", "sw cl s"+strconv.Itoa(si))
				s.leaf("circle", "", "cx", f1(x+8), "cy", f1(y+8), "r", "3.2", "class", "sw pt cl s"+strconv.Itoa(si))
				s.label(x+20, y+12, fsLegend, "start", "lg", it.name)
				x += it.w + 16
				continue
			}
			s.leaf("rect", "", "x", f1(x), "y", f1(y+3), "width", "10", "height", "10", "rx", "2", "class", "sw s"+strconv.Itoa(si))
			s.label(x+14, y+12, fsLegend, "start", "lg", it.name)
			x += it.w + 16
		}
	}
	s.close("g")
	return math.Max(1, float64(len(rows))) * 20
}

// drawXY 는 항목이 가로축인 차트(bar·stacked·line·area)다.
func drawXY(ch *Chart) string {
	var s svgw
	n := len(ch.Cats)
	ns := len(ch.Series)
	const h = 300.0

	// 값의 범위(왼쪽 축 lo·hi, 오른쪽 축 lo2·hi2)
	lo, hi := math.Inf(1), math.Inf(-1)
	lo2, hi2 := math.Inf(1), math.Inf(-1)
	if ch.ChartType == "stacked" {
		for i := 0; i < n; i++ {
			pos, neg := 0.0, 0.0
			for _, se := range ch.Series {
				if se.Has[i] {
					if se.Values[i] >= 0 {
						pos += se.Values[i]
					} else {
						neg += se.Values[i]
					}
				}
			}
			lo, hi = math.Min(lo, neg), math.Max(hi, pos)
		}
	} else {
		for si, se := range ch.Series {
			for i, v := range se.Values {
				if !se.Has[i] {
					continue
				}
				if ch.axis(si) == 1 {
					lo2, hi2 = math.Min(lo2, v), math.Max(hi2, v)
				} else {
					lo, hi = math.Min(lo, v), math.Max(hi, v)
				}
			}
		}
	}
	if ch.Ref != nil {
		lo, hi = math.Min(lo, ch.Ref.Value), math.Max(hi, ch.Ref.Value) // 기준선은 축 범위에 든다
	}
	if lo > hi {
		lo, hi = 0, 0 // 왼쪽 축에 값이 없다(보조 축 계열에만 숫자가 있을 때)
	}
	near0 := func(lo, hi float64) bool { return lo > 0 && lo <= hi*0.5 || hi < 0 && hi >= lo*0.5 }
	if ch.ChartType != "line" || near0(lo, hi) {
		lo, hi = math.Min(lo, 0), math.Max(hi, 0) // 막대·면은 0 에서 자란다. 선도 0 에 가까우면 0 부터
	}
	tlo, thi, step, tdec := niceTicks(lo, hi)
	var head svgw
	hh := header(&head, ch, 0)
	if ch.ChartType == "bar" && ch.Labels {
		// 음수 막대의 값 글자는 막대 아래에 놓인다. 막대가 축 끝까지 자라면 글자가 항목 이름 줄로 내려가 겹치므로("-2.5%3월", 실측)
		// 글자 자리(fsVal+4)가 날 때까지 눈금을 한 칸씩 내린다. 그림 높이는 항목 글자 두 줄일 때로 어림한다(실제는 그보다 높다).
		vmin, nb := 0.0, 0
		for si, se := range ch.Series {
			if si == ch.Line {
				continue
			}
			nb++
			for i, v := range se.Values {
				if se.Has[i] {
					vmin = math.Min(vmin, v)
				}
			}
		}
		if n*nb > 36 {
			vmin = 0 // 막대가 많아 값 글자를 어차피 뺀다(아래)
		}
		ph := h - (10 + 2*15) - 8
		if hh > 0 {
			ph -= hh + 10
		}
		for k := 0; k < 3 && vmin < 0 && (vmin-tlo)/(thi-tlo)*ph < fsVal+4; k++ {
			tlo -= step
		}
	}
	var ticks []string
	tickW := 0.0
	for v := tlo; v <= thi+step/2; v += step {
		t := fmtNum(v, tdec)
		if ch.Unit == "%" {
			t += "%"
		}
		ticks = append(ticks, t)
		tickW = math.Max(tickW, textW(t, fsAxis))
	}
	// 오른쪽 축(보조 축): 선의 0 포함 규칙(오른쪽 축 계열은 늘 선이다), 격자선은 왼쪽 축과 같은 칸 수
	var ticks2 []string
	tlo2, thi2, tickW2 := 0.0, 1.0, 0.0
	if ch.Axis2 >= 0 {
		if near0(lo2, hi2) {
			lo2, hi2 = math.Min(lo2, 0), math.Max(hi2, 0)
		}
		nt := int(math.Round((thi - tlo) / step))
		k0 := -1
		if tlo <= 0 && thi >= 0 {
			k0 = int(math.Round(-tlo / step))
		}
		step2, tdec2 := 0.0, 0
		tlo2, step2, tdec2 = alignTicks(nt, k0, lo2, hi2)
		thi2 = tlo2 + float64(nt)*step2
		for k := 0; k <= nt; k++ {
			t := fmtNum(tlo2+float64(k)*step2, tdec2)
			if ch.Unit2 == "%" {
				t += "%"
			}
			ticks2 = append(ticks2, t)
			tickW2 = math.Max(tickW2, textW(t, fsAxis))
		}
	}

	// 가로: 항목 칸
	x0 := tickW + 10
	x1 := cvW - 4
	if ch.ChartType == "line" || ch.ChartType == "area" {
		x1 = cvW - 14
	}
	refLab := ""
	if ch.Ref != nil {
		// 기준선 라벨은 그림 영역 오른쪽 밖에 둔다 — 막대·점·값 글자와 겹치지 않게
		refLab = fitText(ch.refText(), fsVal, 120)
		x1 = cvW - 4 - textW(refLab, fsVal) - 8
	}
	if ch.Axis2 >= 0 {
		x1 = cvW - 4 - tickW2 - 6 // 오른쪽 축 눈금 글자(x1+6 부터 왼쪽 맞춤). 보조 축과 기준선은 함께 오지 않는다(chart.go)
	}
	slot := (x1 - x0) / float64(n)
	maxLab := 0.0
	for _, c := range ch.Cats {
		maxLab = math.Max(maxLab, textW(c, fsAxis))
	}
	lines, every := 1, 1
	switch {
	case maxLab <= slot-4:
	case slot >= 44:
		lines = 2
	default:
		every = int(math.Ceil((math.Min(maxLab, 90) + 8) / slot))
	}

	top := 0.0
	if hh > 0 {
		top = hh + 10
	}
	y0 := top + 8
	y1 := h - (10 + float64(lines)*15)
	yv := func(v float64) float64 { return y1 - (v-tlo)/(thi-tlo)*(y1-y0) }
	yv2 := func(v float64) float64 { return y1 - (v-tlo2)/(thi2-tlo2)*(y1-y0) }
	yOf := func(si int) func(float64) float64 { // 계열 si 의 축
		if ch.axis(si) == 1 {
			return yv2
		}
		return yv
	}
	cx := func(i int) float64 { return x0 + slot*(float64(i)+0.5) }

	svgOpen(&s, ch, h)
	s.b.WriteString(head.b.String())
	// 눈금·격자
	s.open("g", "class", "grid")
	for k, t := range ticks {
		v := tlo + float64(k)*step
		y := yv(v)
		cls := "gl"
		if math.Abs(v) < step/1e6 {
			cls = "gl zero"
		}
		s.leaf("line", "", "x1", f1(x0), "y1", f1(y), "x2", f1(x1), "y2", f1(y), "class", cls)
		s.label(x0-6, y+4, fsAxis, "end", "tick", t)
	}
	for k, t := range ticks2 {
		// 오른쪽 축 눈금은 왼쪽 격자선 높이에(칸 수가 같다)
		s.label(x1+6, y1-float64(k)/float64(len(ticks2)-1)*(y1-y0)+4, fsAxis, "start", "tick ax2", t)
	}
	s.close("g")

	zero := yv(math.Max(tlo, math.Min(0, thi)))
	vs, late := valsLayer(&s, ch)
	switch ch.ChartType {
	case "bar":
		// 막대 계열(혼합 차트면 선 계열 말고). 색은 열 차례(s1…) 그대로다
		var bars []int
		for si := range ch.Series {
			if si != ch.Line {
				bars = append(bars, si)
			}
		}
		nb := len(bars)
		group := slot * 0.7
		if nb == 1 {
			group = slot * 0.58
		}
		gap := math.Min(3, group/float64(nb)*0.12)
		bw := (group - gap*float64(nb-1)) / float64(nb)
		// 값 글자가 막대 위에 들어가는가(하나라도 안 들어가면 모두 뺀다 — 들쭉날쭉하지 않게)
		labels := ch.Labels && n*nb <= 36
		if labels {
			room := bw + gap
			if nb == 1 {
				room = slot - 4
			}
			for _, si := range bars {
				se := ch.Series[si]
				for i, v := range se.Values {
					if se.Has[i] && textW(ch.valText(v), fsVal) > room+2 {
						labels = false
					}
				}
			}
		}
		var barLabs []valLab // 혼합 차트의 막대 값 글자 — 선의 값 글자와 맞춰 본 뒤 낸다
		s.open("g", "class", "marks")
		for i := 0; i < n; i++ {
			gx := cx(i) - group/2
			for bi, si := range bars {
				se := ch.Series[si]
				if !se.Has[i] {
					continue
				}
				v := se.Values[i]
				x := gx + float64(bi)*(bw+gap)
				y, hgt := yv(v), zero-yv(v)
				cls := "col s" + strconv.Itoa(si+1) + " " + delay(i)
				if v < 0 {
					y, hgt = zero, yv(v)-zero
					cls += " neg"
				}
				s.leaf("rect", ch.tip(ch.Cats[i], se.Name, v), "x", f1(x), "y", f1(y), "width", f1(bw), "height", f1(math.Max(hgt, 0.5)), "rx", f1(math.Min(2.5, bw/4)), "class", cls)
				if labels {
					ly := y - 4
					if v < 0 {
						ly = y + hgt + fsVal
					}
					if ch.Line >= 0 {
						barLabs = append(barLabs, valLab{x + bw/2, ly, "middle", "val " + delay(i), ch.valText(v)})
					} else {
						vs.label(x+bw/2, ly, fsVal, "middle", "val "+delay(i), ch.valText(v))
					}
				}
			}
		}
		if ch.Line >= 0 {
			comboLine(&s, vs, ch, barLabs, n, cx, yOf(ch.Line), zero, x1, y1)
		}
		s.close("g")
	case "stacked":
		group := slot * 0.58
		labels := ch.Labels && n <= 24
		s.open("g", "class", "marks")
		for i := 0; i < n; i++ {
			x := cx(i) - group/2
			pos, neg := 0.0, 0.0
			s.open("g", "class", "stack "+delay(i))
			for si, se := range ch.Series {
				if !se.Has[i] || se.Values[i] == 0 {
					continue
				}
				v := se.Values[i]
				var a, b float64
				if v >= 0 {
					a, b = pos, pos+v
					pos = b
				} else {
					a, b = neg+v, neg
					neg = a
				}
				ya, yb := yv(a), yv(b)
				s.leaf("rect", ch.tip(ch.Cats[i], se.Name, v), "x", f1(x), "y", f1(yb), "width", f1(group), "height", f1(math.Max(ya-yb, 0.5)), "class", "seg s"+strconv.Itoa(si+1))
				if labels && ya-yb >= fsVal+5 && textW(ch.valText(v), fsVal) <= group-4 {
					vs.label(x+group/2, (ya+yb)/2+fsVal*0.36, fsVal, "middle", "val in", ch.valText(v))
				}
			}
			s.close("g")
			if labels && textW(ch.valText(pos), fsVal) <= slot-2 && pos > 0 {
				vs.label(cx(i), yv(pos)-4, fsVal, "middle", "val tot "+delay(i), ch.valText(pos))
			}
		}
		s.close("g")
	case "line", "area":
		s.open("g", "class", "marks")
		for si, se := range ch.Series {
			c := "s" + strconv.Itoa(si+1)
			d, area := linePath(se, n, cx, yOf(si), zero)
			if ch.ChartType == "area" && area != "" {
				s.leaf("path", "", "d", area, "class", "area "+c)
			}
			if d != "" {
				s.leaf("path", "", "d", d, "class", "line "+c, "pathLength", "1")
			}
		}
		// 점과 값(선마다 마지막 값, 계열이 하나이고 항목이 12개 이하면 모든 값)
		for si, se := range ch.Series {
			c := "s" + strconv.Itoa(si+1)
			last := lastIndex(se, n)
			all := ns == 1 && n <= 12
			yvs := yOf(si)
			for i := 0; i < n; i++ {
				if !se.Has[i] {
					continue
				}
				v := se.Values[i]
				s.leaf("circle", ch.tipAx(ch.Cats[i], se.Name, ch.axis(si), v), "cx", f1(cx(i)), "cy", f1(yvs(v)), "r", dotR(n), "class", "dot "+c+" "+delay(i))
				if ch.Labels && (all || i == last) {
					l := lineLabel(ch, si, i, n, last, all, ns > 1 && si%2 == 1 && !all, cx, yvs, x1, y1)
					vs.label(l.x, l.y, fsVal, l.anchor, l.class, l.t)
				}
			}
		}
		s.close("g")
	}
	if ch.Ref != nil {
		y := yv(ch.Ref.Value)
		refGroup(&s, ch, x0, y, x1, y, x1+6, y+fsVal*0.36, "start", refLab)
	}
	flushVals(&s, late) // 기준선·선 계열 뒤(없으면 아무것도 하지 않는다)

	// 항목 글자
	s.open("g", "class", "cats")
	for i, c := range ch.Cats {
		if i%every != 0 {
			continue
		}
		width := slot*float64(every) - 4
		if lines == 2 {
			ls := wrap2(c, fsAxis, width)
			s.open("text", "x", f1(cx(i)), "y", f1(y1+16), "font-size", f1(fsAxis), "text-anchor", "middle", "class", "cat")
			for k, l := range ls {
				s.open("tspan", "x", f1(cx(i)), "y", f1(y1+16+float64(k)*15))
				s.text(l)
				s.close("tspan")
			}
			if len(ls) > 1 || ls[0] != c {
				s.title(c)
			}
			s.close("text")
			continue
		}
		t := fitText(c, fsAxis, width)
		s.open("text", "x", f1(cx(i)), "y", f1(y1+16), "font-size", f1(fsAxis), "text-anchor", "middle", "class", "cat")
		s.text(t)
		if t != c {
			s.title(c)
		}
		s.close("text")
	}
	s.close("g")
	s.close("svg")
	return s.b.String()
}

// drawHBar 는 가로 막대다(항목 이름이 길 때 알맞다).
func drawHBar(ch *Chart) string {
	var s svgw
	n, ns := len(ch.Cats), len(ch.Series)
	barH := 13.0
	if ns == 1 {
		barH = 16
	}
	rowH := float64(ns)*barH + float64(ns-1)*2 + 11
	lo, hi := 0.0, 0.0
	maxVal := 0.0
	for _, se := range ch.Series {
		for i, v := range se.Values {
			if se.Has[i] {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
				maxVal = math.Max(maxVal, textW(ch.valText(v), fsVal))
			}
		}
	}
	if ch.Ref != nil {
		lo, hi = math.Min(lo, ch.Ref.Value), math.Max(hi, ch.Ref.Value)
	}
	tlo, thi, step, tdec := niceTicks(lo, hi)
	var head svgw
	hh := header(&head, ch, 0)
	top := 0.0
	if hh > 0 {
		top = hh + 8
	}
	refLab := ""
	if ch.Ref != nil {
		// 세로 기준선의 라벨은 막대 영역 위에 한 줄을 비워 둔다
		refLab = fitText(ch.refText(), fsVal, 200)
		top += 16
	}
	maxLab := 0.0
	for _, c := range ch.Cats {
		maxLab = math.Max(maxLab, textW(c, fsAxis))
	}
	labW := math.Min(maxLab, cvW*0.32)
	twoLines := maxLab > labW && rowH >= 30
	x0 := labW + 10
	x1 := cvW - 4
	if ch.Labels {
		x1 -= maxVal + 6
		// 음수 값 글자는 막대 왼쪽 끝(xv(v)-4)에 오른쪽 맞춤으로 붙는다 — 항목 이름(labW 에 오른쪽 맞춤)과 6 띄우도록
		// 그림 시작 x0 를 필요한 만큼만 민다: x0 + f·(x1-x0) - 4 - w ≥ labW + 6, f = (v-tlo)/(thi-tlo) < 1(축은 0 을 포함)
		for _, se := range ch.Series {
			for i, v := range se.Values {
				if se.Has[i] && v < 0 {
					f := (v - tlo) / (thi - tlo)
					x0 = math.Max(x0, (labW+10+textW(ch.valText(v), fsVal)-f*x1)/(1-f))
				}
			}
		}
		x0 = math.Min(x0, x1-60) // 그림 너비는 남긴다(값 글자가 터무니없이 길 때)
	}
	y0 := top + 4
	h := y0 + float64(n)*rowH + 22
	xv := func(v float64) float64 { return x0 + (v-tlo)/(thi-tlo)*(x1-x0) }
	zero := xv(0)

	svgOpen(&s, ch, h)
	s.b.WriteString(head.b.String())
	s.open("g", "class", "grid")
	for v := tlo; v <= thi+step/2; v += step {
		x := xv(v)
		cls := "gl"
		if math.Abs(v) < step/1e6 {
			cls = "gl zero"
		}
		s.leaf("line", "", "x1", f1(x), "y1", f1(y0), "x2", f1(x), "y2", f1(y0+float64(n)*rowH), "class", cls)
		t := fmtNum(v, tdec)
		if ch.Unit == "%" {
			t += "%"
		}
		s.label(x, h-6, fsAxis, "middle", "tick", t)
	}
	s.close("g")
	vs, late := valsLayer(&s, ch)
	s.open("g", "class", "marks")
	for i, c := range ch.Cats {
		ry := y0 + float64(i)*rowH + 5.5
		mid := ry + (rowH-11)/2
		if twoLines {
			ls := wrap2(c, fsAxis, labW)
			s.open("text", "x", f1(labW), "y", f1(mid-2), "font-size", f1(fsAxis), "text-anchor", "end", "class", "cat")
			for k, l := range ls {
				y := mid + fsAxis*0.36
				if len(ls) == 2 {
					y = mid - 3 + float64(k)*14
				}
				s.open("tspan", "x", f1(labW), "y", f1(y))
				s.text(l)
				s.close("tspan")
			}
			if len(ls) > 1 || ls[0] != c {
				s.title(c)
			}
			s.close("text")
		} else {
			t := fitText(c, fsAxis, labW)
			s.open("text", "x", f1(labW), "y", f1(mid+fsAxis*0.36), "font-size", f1(fsAxis), "text-anchor", "end", "class", "cat")
			s.text(t)
			if t != c {
				s.title(c)
			}
			s.close("text")
		}
		for si, se := range ch.Series {
			if !se.Has[i] {
				continue
			}
			v := se.Values[i]
			y := ry + float64(si)*(barH+2)
			x, w := zero, xv(v)-zero
			cls := "col s" + strconv.Itoa(si+1) + " " + delay(i)
			if v < 0 {
				x, w = xv(v), zero-xv(v)
				cls += " neg"
			}
			s.leaf("rect", ch.tip(c, se.Name, v), "x", f1(x), "y", f1(y), "width", f1(math.Max(w, 0.5)), "height", f1(barH), "rx", "2", "class", cls)
			if ch.Labels {
				lx, anchor := x+w+4, "start"
				if v < 0 {
					lx, anchor = x-4, "end"
				}
				vs.label(lx, y+barH/2+fsVal*0.36, fsVal, anchor, "val "+delay(i), ch.valText(v))
			}
		}
	}
	s.close("g")
	if ch.Ref != nil {
		x := xv(ch.Ref.Value)
		w := textW(refLab, fsVal)
		lx := math.Max(w/2+2, math.Min(x, cvW-w/2-2))
		refGroup(&s, ch, x, y0-2, x, y0+float64(n)*rowH, lx, y0-6, "middle", refLab)
		flushVals(&s, late)
	}
	s.close("svg")
	return s.b.String()
}

// drawPie 는 원·도넛이다. 첫 계열만 쓴다. 원 아래에 항목·값·비율 목록을 둔다 — 좁은 viewBox(360)라 다단의 한 단에
// 넣어도 글자가 읽힌다(넓은 자리에서는 CSS 가 너비를 줄여 크게 늘지 않게 한다).
func drawPie(ch *Chart) string {
	const pw = 360.0
	var s svgw
	se := ch.Series[0]
	total := 0.0
	var idx []int
	for i, v := range se.Values {
		if se.Has[i] && v > 0 {
			total += v
			idx = append(idx, i)
		}
	}
	R := 92.0
	cx, cy := pw/2, R+6
	rowH := 21.0
	h := cy + R + 18 + float64(len(idx))*rowH
	inner := 0.0
	if ch.ChartType == "donut" {
		inner = R * 0.6
	}
	svgOpenW(&s, ch, pw, h)
	s.open("g", "class", "pie")
	a := -math.Pi / 2
	pt := func(r, ang float64) string { return f1(cx+r*math.Cos(ang)) + " " + f1(cy+r*math.Sin(ang)) }
	type lab struct {
		x, y float64
		t    string
	}
	var inside []lab
	for k, i := range idx {
		v := se.Values[i]
		sweep := v / total * 2 * math.Pi
		b := a + sweep
		large := "0"
		if sweep > math.Pi {
			large = "1"
		}
		var d string
		switch {
		case len(idx) == 1:
			// 한 조각이 원 전체 — 반원 둘로
			d = "M" + pt(R, -math.Pi/2) + "A" + f1(R) + " " + f1(R) + " 0 1 1 " + pt(R, math.Pi/2) + "A" + f1(R) + " " + f1(R) + " 0 1 1 " + pt(R, -math.Pi/2) + "Z"
			if inner > 0 {
				d += "M" + pt(inner, -math.Pi/2) + "A" + f1(inner) + " " + f1(inner) + " 0 1 0 " + pt(inner, math.Pi/2) + "A" + f1(inner) + " " + f1(inner) + " 0 1 0 " + pt(inner, -math.Pi/2) + "Z"
			}
		case inner > 0:
			d = "M" + pt(R, a) + "A" + f1(R) + " " + f1(R) + " 0 " + large + " 1 " + pt(R, b) + "L" + pt(inner, b) +
				"A" + f1(inner) + " " + f1(inner) + " 0 " + large + " 0 " + pt(inner, a) + "Z"
		default:
			d = "M" + f1(cx) + " " + f1(cy) + "L" + pt(R, a) + "A" + f1(R) + " " + f1(R) + " 0 " + large + " 1 " + pt(R, b) + "Z"
		}
		s.leaf("path", ch.tip(ch.Cats[i], "", v)+" ("+pctText(v/total)+")", "d", d, "class", "slice s"+strconv.Itoa(k+1)+" "+delay(k))
		if ch.Labels && sweep >= 0.42 {
			mid := (a + b) / 2
			r := R * 0.63
			if inner > 0 {
				r = (R + inner) / 2
			}
			inside = append(inside, lab{cx + r*math.Cos(mid), cy + r*math.Sin(mid) + fsVal*0.36, pctText(v / total)})
		}
		a = b
	}
	s.close("g")
	for _, l := range inside {
		s.label(l.x, l.y, fsVal, "middle", "val in", l.t)
	}
	if inner > 0 {
		s.label(cx, cy+4, 21, "middle", "total", fmtVal(total, ch.Decimals))
		u := "합계"
		if ch.Unit != "" && ch.Unit != "%" {
			u = "합계 · " + ch.Unit
		}
		s.label(cx, cy+21, fsAxis, "middle", "unit", u)
	}
	// 아래 목록: 색 · 이름 · 값 · 비율
	s.open("g", "class", "legend")
	ly := cy + R + 14
	for k, i := range idx {
		y := ly + float64(k)*rowH
		v := se.Values[i]
		s.leaf("rect", "", "x", "6", "y", f1(y+4), "width", "11", "height", "11", "rx", "2", "class", "sw s"+strconv.Itoa(k+1))
		val := ch.valText(v)
		if ch.Unit != "" && ch.Unit != "%" {
			val += " " + ch.Unit
		}
		s.label(pw-4, y+14, fsLegend, "end", "lg pct", pctText(v/total))
		vx := pw - 52
		s.label(vx, y+14, fsLegend, "end", "lg vnum", val)
		nameW := vx - textW(val, fsLegend) - 12 - 24
		s.label(24, y+14, fsLegend, "start", "lg", fitText(ch.Cats[i], fsLegend, nameW))
	}
	s.close("g")
	s.close("svg")
	return s.b.String()
}

// valLab 은 미뤄 내는 값 글자다.
type valLab struct {
	x, y     float64
	anchor   string
	class, t string
}

// box 는 값 글자(fsVal)가 차지하는 칸의 어림이다(왼쪽·위·오른쪽·아래).
func (l valLab) box() [4]float64 {
	w := textW(l.t, fsVal)
	left := l.x
	switch l.anchor {
	case "middle":
		left -= w / 2
	case "end":
		left -= w
	}
	return [4]float64{left, l.y - fsVal*0.8, left + w, l.y + fsVal*0.25}
}

// comboGap 은 혼합 차트에서 선의 값 글자와 막대의 값 글자 사이에 둘 틈이다 — 붙어 있으면 "31.0 36.0" 처럼 한 쌍으로 읽힌다(실측).
const comboGap = 8.0

// overlaps 는 두 칸이 틈 gap 안으로 다가오는지다.
func overlaps(a, b [4]float64, gap float64) bool {
	return a[0] < b[2]+gap && b[0] < a[2]+gap && a[1] < b[3]+gap && b[1] < a[3]+gap
}

// linePath 는 계열 se 의 선(빈 값에서 끊긴다)과 면(base 까지)의 path d 다. 점이 없으면 빈 글자.
func linePath(se chartSeries, n int, cx func(int) float64, yv func(float64) float64, base float64) (string, string) {
	var d, area strings.Builder
	var run []int
	flush := func() {
		if len(run) == 0 {
			return
		}
		for k, i := range run {
			if k == 0 {
				d.WriteString("M" + f1(cx(i)) + " " + f1(yv(se.Values[i])))
			} else {
				d.WriteString("L" + f1(cx(i)) + " " + f1(yv(se.Values[i])))
			}
		}
		if len(run) > 1 {
			area.WriteString("M" + f1(cx(run[0])) + " " + f1(base))
			for _, i := range run {
				area.WriteString("L" + f1(cx(i)) + " " + f1(yv(se.Values[i])))
			}
			area.WriteString("L" + f1(cx(run[len(run)-1])) + " " + f1(base) + "Z")
		}
		run = nil
	}
	for i := 0; i < n; i++ {
		if se.Has[i] {
			run = append(run, i)
		} else {
			flush()
		}
	}
	flush()
	return d.String(), area.String()
}

func lastIndex(se chartSeries, n int) int {
	for i := n - 1; i >= 0; i-- {
		if se.Has[i] {
			return i
		}
	}
	return -1
}

func dotR(n int) string {
	if n > 24 {
		return "2"
	}
	return "3.2"
}

// lineLabel 은 계열 si 의 선 위 점 i 의 값 글자 자리다. alt 면(계열 여럿의 짝수 번째) 점 아래에 둔다. yv 는 그 계열의 축이다.
func lineLabel(ch *Chart, si, i, n, last int, all, alt bool, cx func(int) float64, yv func(float64) float64, x1, y1 float64) valLab {
	se := ch.Series[si]
	v := se.Values[i]
	t := ch.valTextAx(ch.axis(si), v)
	// 값은 선을 피해 둔다: 이웃보다 낮은 점(골)은 아래, 높은 점(봉우리)·오르는 점은 위. 아래 자리가 그림 아래 끝(y1)을 넘으면
	// 항목 이름 줄과 겹치므로 위에 둔다(축 최솟값의 점 — 리뷰 R1)
	y := yv(v) - 8
	if (below(se, i, n) || alt) && yv(v)+fsVal+6+fsVal*0.25 <= y1 {
		y = yv(v) + fsVal + 6
	}
	anchor := "middle"
	x := cx(i)
	switch {
	case i == last && !all && n > 1:
		anchor, x = "end", cx(i)+4
	case all && slopeThrough(se, i, n) < 0:
		// 내려가는 길의 점: 선은 왼쪽 위에서 와서 오른쪽 아래로 간다 — 오른쪽 위가 비었다
		anchor, x, y = "start", cx(i)+5, yv(v)-6
	case all && slopeThrough(se, i, n) > 0:
		anchor, x, y = "end", cx(i)-5, yv(v)-6
	}
	if ch.Ref != nil || ch.Axis2 >= 0 {
		// 기준선 라벨·오른쪽 축 눈금은 그림 영역 오른쪽 밖(x1+6 부터)이다 — 값 글자의 오른쪽 끝을 x1 안으로 당긴다
		x -= math.Max(0, textRight(x, anchor, t)-x1)
	}
	return valLab{x, y, anchor, "val " + delay(i), t}
}

// comboLine 은 혼합 차트(bar + line="계열")의 선 계열을 막대 위에 그린다: 종이색 테두리 선(같은 색 막대 위에서도 보이게) 위에
// 선과 점. 선·점·범례 표식에는 표지 cl 을 단다 — 옅은 계열 색(공문의 회색조)이 종이 위에서 묻히는 테마가 혼합 차트의 선만
// 진하게 칠할 수 있게(공문 theme.css). 값 글자는 막대가 먼저다 — 선의 값 글자는 모든 점(항목 12개 이하) → 마지막 점 → 없음 차례로, 막대 값 글자와
// 겹치지도 붙지도(comboGap) 않고, 선 값 글자끼리 겹치지 않으며, 그림 안에 드는 첫 단계를 쓴다. 모든 값 글자는 선 뒤(.vals, 종이색 테두리)에 낸다 — 선이 글자를 가로지르지 않게.
func comboLine(s, vs *svgw, ch *Chart, barLabs []valLab, n int, cx func(int) float64, yv func(float64) float64, base, x1, y1 float64) {
	si := ch.Line
	se := ch.Series[si]
	c := "s" + strconv.Itoa(si+1)
	if d, _ := linePath(se, n, cx, yv, base); d != "" {
		s.leaf("path", "", "d", d, "class", "line halo", "pathLength", "1")
		s.leaf("path", "", "d", d, "class", "line cl "+c, "pathLength", "1")
	}
	last := lastIndex(se, n)
	labs := func(all bool) []valLab {
		var out []valLab
		for i := 0; i < n; i++ {
			if se.Has[i] && (all || i == last) {
				out = append(out, lineLabel(ch, si, i, n, last, all, false, cx, yv, x1, y1))
			}
		}
		return out
	}
	// clear 는 그 단계의 선 값 글자가 모두 그림 안(왼쪽 0·오른쪽 viewBox 끝·아래 y1 — 항목 이름 줄 위)이고, 서로 겹치지 않으며,
	// 막대 값 글자와 겹치지도 붙지도 않는지다(리뷰 R1: 12항목의 긴 숫자는 서로 겹치고 오른쪽으로 넘쳤다)
	clear := func(ls []valLab) bool {
		for k, a := range ls {
			if b := a.box(); b[0] < 0 || b[2] > cvW || b[3] > y1 {
				return false
			}
			for _, c := range ls[k+1:] {
				if overlaps(a.box(), c.box(), 2) {
					return false
				}
			}
			for _, b := range barLabs {
				if overlaps(a.box(), b.box(), comboGap) {
					return false
				}
			}
		}
		return true
	}
	var lineLabs []valLab
	if ch.Labels {
		for _, all := range []bool{n <= 12, false} {
			if ls := labs(all); clear(ls) {
				lineLabs = ls
				break
			}
		}
	}
	for i := 0; i < n; i++ {
		if se.Has[i] {
			s.leaf("circle", ch.tipAx(ch.Cats[i], se.Name, ch.axis(si), se.Values[i]), "cx", f1(cx(i)), "cy", f1(yv(se.Values[i])), "r", dotR(n), "class", "dot cl "+c+" "+delay(i))
		}
	}
	for _, ls := range [][]valLab{barLabs, lineLabs} {
		for _, l := range ls {
			vs.label(l.x, l.y, fsVal, l.anchor, l.class, l.t)
		}
	}
}

// below 는 선 위의 점 i 가 이웃 점들보다 낮은지다(값 글자를 점 아래에 둘지).
func below(se chartSeries, i, n int) bool {
	sum, k := 0.0, 0
	for _, j := range []int{i - 1, i + 1} {
		if j >= 0 && j < n && se.Has[j] {
			sum += se.Values[j]
			k++
		}
	}
	return k > 0 && se.Values[i] < sum/float64(k)
}

// slopeThrough 는 점 i 를 지나는 선이 한 방향으로 오르면 1, 내리면 -1, 봉우리·골·끝점이면 0 이다.
func slopeThrough(se chartSeries, i, n int) int {
	if i == 0 || i == n-1 || !se.Has[i-1] || !se.Has[i+1] {
		return 0
	}
	a, b, c := se.Values[i-1], se.Values[i], se.Values[i+1]
	switch {
	case a > b && b > c:
		return -1
	case a < b && b < c:
		return 1
	}
	return 0
}

func pctText(f float64) string {
	p := f * 100
	if p < 10 && math.Round(p*10)/10 != math.Round(p) {
		return strconv.FormatFloat(math.Round(p*10)/10, 'f', 1, 64) + "%"
	}
	return strconv.Itoa(int(math.Round(p))) + "%"
}
