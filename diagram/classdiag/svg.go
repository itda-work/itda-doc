package classdiag

import (
	"math"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 그리기. 쓰는 요소·속성은 흐름도·간트·타임라인·순서도·ER·상태도와 같은 허용 목록이다(svg·g·rect·path·line·text·tspan·
// title — mdoc/security_test.go 가 고정한다). 끝 모양은 marker 가 아니라 path 다(고정 id 가 한 문서의 여러 그림에서 겹치지
// 않게 — 흐름도·상태도와 같은 방식). 색은 클래스 → 토큰(base.css), 새 색 상수·새 토큰 없음. 클래스는 모두 cls- 접두사다
// (흐름도가 cl-box·cl-title 을 subgraph 에 쓴다 — 앱 CSS 충돌은 classdiag_test.go TestClassCollisions):
//
//	cls-rel(·cls-dot)              관계 선(점선)
//	cls-ext·cls-comp·cls-agg·cls-dep 끝 모양 — 속이 빈 삼각형·채운 마름모·속이 빈 마름모·채운 갈고리 화살촉
//	cls-lbg·cls-lbl                관계 라벨 바탕·글자       cls-card  양끝 수량 글(바탕 없음)
//	cls-box·cls-div                클래스 상자·칸 구분선
//	cls-ann·cls-name·cls-mem       주석 «이름»·클래스 이름(굵게)·멤버 글(cls-static 밑줄·cls-abstract 기울임)
//
// 그리는 차례: 관계 선 → 끝 모양 → 라벨 바탕·글 → 수량 글 → 클래스(클래스가 마지막이라 선이 상자를 지나가도 가린다).

// 끝 모양 크기 — 원본 markers.js 의 꼴(viewBox 18 단위)을 그대로 쓴다: 삼각형 M1,7 L18,13 V1 Z(길이 17·반 폭 6), 마름모
// M18,7 L9,13 L1,7 L9,1 Z(길이 17·반 폭 6), 의존 M18,7 L9,13 L14,7 L9,1 Z(끝에서 9·반 폭 6, 안쪽 꺾임 4).
const (
	shapeLen  = 17.0
	shapeHalf = 6.0
	depBack   = 9.0
	depNotch  = 4.0
)

func p2(p layered.Pt) string { return diagram.F1(p.X) + " " + diagram.F1(p.Y) }

// hyp 는 √(x²+y²) 다 — 곱셈-덧셈을 명시적 float64 변환으로 나눠 FMA 합치기를 막는다(layered 의 hyp 와 같다).
func hyp(x, y float64) float64 { return math.Sqrt(float64(x*x) + float64(y*y)) }

// frame 은 tip 에서 from 의 반대쪽을 향하는 단위 벡터 u 와 그 법선 n 이다(from 쪽에서 tip 쪽으로).
func frame(tip, from layered.Pt) (ux, uy float64) {
	dx, dy := tip.X-from.X, tip.Y-from.Y
	d := hyp(dx, dy)
	if d == 0 {
		return 0, 1
	}
	return dx / d, dy / d
}

// at 은 tip 에서 u 반대로 back, 법선 쪽으로 side 만큼 간 점이다.
func at(tip layered.Pt, ux, uy, back, side float64) layered.Pt {
	return layered.Pt{X: tip.X - float64(ux*back) - float64(uy*side), Y: tip.Y - float64(uy*back) + float64(ux*side)}
}

// endShape 는 끝 모양 e 의 path 와 선 끝을 당길 길이다(None 이면 "").
func endShape(e End, tip, from layered.Pt) (string, float64) {
	ux, uy := frame(tip, from)
	pt := func(back, side float64) string { return p2(at(tip, ux, uy, back, side)) }
	switch e {
	case Extension:
		return "M" + p2(tip) + " L" + pt(shapeLen, shapeHalf) + " L" + pt(shapeLen, -shapeHalf) + " Z", shapeLen
	case Composition, Aggregation:
		return "M" + p2(tip) + " L" + pt(shapeLen/2, shapeHalf) + " L" + pt(shapeLen, 0) + " L" + pt(shapeLen/2, -shapeHalf) + " Z", shapeLen
	case Dependency:
		return "M" + p2(tip) + " L" + pt(depBack, shapeHalf) + " L" + pt(depNotch, 0) + " L" + pt(depBack, -shapeHalf) + " Z", depNotch + 1
	}
	return "", 0
}

var shapeClass = [...]string{"", "cls-ext", "cls-comp", "cls-agg", "cls-dep"}

// pullBack 은 선 끝을 끝 모양 안쪽으로 당긴다(선 끝이 모양 끝을 뚫고 나오지 않게). 짧은 선은 그대로(흐름도와 같다).
func pullBack(tip, from layered.Pt, by float64) layered.Pt {
	dx, dy := tip.X-from.X, tip.Y-from.Y
	d := hyp(dx, dy)
	if by == 0 || d <= by*2 {
		return tip
	}
	return layered.Pt{X: tip.X - float64(dx/d*by), Y: tip.Y - float64(dy/d*by)}
}

func drawSVG(c *chart, l *laid) string {
	s := &diagram.Writer{}
	label := c.AccTitle
	if label == "" {
		label = c.FMCaption
	}
	if label == "" {
		label = "클래스도"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(l.W)+" "+diagram.F1(l.H), "width", diagram.F1(l.W*diagram.Scale),
		"height", diagram.F1(l.H*diagram.Scale), "role", "img", "aria-label", label)
	if c.AccDescr != "" {
		s.Open("title")
		s.Text(c.AccDescr)
		s.Close("title")
	}
	fs := diagram.F1(fontSize)

	if len(c.Rels) > 0 {
		type shape struct{ cls, d string }
		var shapes []shape
		s.Open("g", "class", "cls-rels")
		for i, r := range c.Rels {
			orig := l.edges[i].Pts
			pts := append([]layered.Pt(nil), orig...)
			n := len(pts)
			if d, by := endShape(r.Type1, orig[0], orig[1]); d != "" {
				shapes = append(shapes, shape{shapeClass[r.Type1], d})
				pts[0] = pullBack(orig[0], orig[1], by)
			}
			if d, by := endShape(r.Type2, orig[n-1], orig[n-2]); d != "" {
				shapes = append(shapes, shape{shapeClass[r.Type2], d})
				pts[n-1] = pullBack(orig[n-1], orig[n-2], by)
			}
			cls := "cls-rel"
			if r.Dotted {
				cls = "cls-rel cls-dot"
			}
			s.Leaf("path", "class", cls, "d", layered.Basis(pts))
		}
		s.Close("g")
		if len(shapes) > 0 {
			s.Open("g", "class", "cls-ends")
			for _, sh := range shapes {
				s.Leaf("path", "class", sh.cls, "d", sh.d)
			}
			s.Close("g")
		}
		labeled := false
		for i := range c.Rels {
			labeled = labeled || len(l.edges[i].Label) > 0
		}
		if labeled {
			s.Open("g", "class", "cls-lbls")
			for i := range c.Rels {
				ep := l.edges[i]
				if len(ep.Label) == 0 {
					continue
				}
				s.Leaf("rect", "class", "cls-lbg", "x", diagram.F1(ep.LC.X-ep.LW/2), "y", diagram.F1(ep.LC.Y-ep.LH/2),
					"width", diagram.F1(ep.LW), "height", diagram.F1(ep.LH), "rx", "2")
				centerLines(s, "cls-lbl", ep.LC, ep.Label, fs)
			}
			s.Close("g")
		}
		if len(l.cards) > 0 {
			s.Open("g", "class", "cls-cards")
			for _, cd := range l.cards {
				centerLines(s, "cls-card", cd.C, []string{cd.text}, fs)
			}
			s.Close("g")
		}
	}

	s.Open("g", "class", "cls-nodes")
	for i := range c.Classes {
		drawClass(s, l.boxes[i], l.at[i], fs)
	}
	s.Close("g")
	s.Close("svg")
	return diagram.AriaRefs(s.String())
}

func drawClass(s *diagram.Writer, b box, c layered.Pt, fs string) {
	x0, y0 := c.X-b.w/2, c.Y-b.h/2
	s.Leaf("rect", "class", "cls-box", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(b.w), "height", diagram.F1(b.h))
	y1, y2 := y0+b.nameH, y0+b.nameH+b.attrH
	for _, y := range []float64{y1, y2} {
		s.Leaf("line", "class", "cls-div", "x1", diagram.F1(x0), "y1", diagram.F1(y), "x2", diagram.F1(x0+b.w), "y2", diagram.F1(y))
	}
	top := y0 + padY
	if len(b.annot) > 0 {
		topLines(s, "cls-ann", c.X, top, b.annot, "middle", fs)
		top += float64(float64(len(b.annot)) * lineH)
	}
	topLines(s, "cls-name", c.X, top, b.name, "middle", fs)
	members(s, x0+padX, y1, b.attrs, b.attrM, fs)
	members(s, x0+padX, y2, b.meths, b.methM, fs)
}

func members(s *diagram.Writer, x, y float64, ls [][]string, ms []member, fs string) {
	top := y + secPad
	for i, l := range ls {
		cls := "cls-mem"
		switch ms[i].Cls {
		case Static:
			cls = "cls-mem cls-static"
		case Abstract:
			cls = "cls-mem cls-abstract"
		}
		topLines(s, cls, x, top, l, "start", fs)
		top += float64(float64(len(l)) * lineH)
	}
}

// topLines 는 위 끝 top 부터 줄들을 쓴다(줄마다 tspan, anchor 는 text-anchor). 기준선은 줄 가운데에서 글자 크기의 0.35
// 아래 — dominant-baseline 을 쓰지 않는다(흐름도 writeLines 와 같은 규칙).
func topLines(s *diagram.Writer, cls string, x, top float64, lines []string, anchor, fs string) {
	if len(lines) == 1 {
		s.Open("text", "class", cls, "x", diagram.F1(x), "y", diagram.F1(top+lineH/2+fontSize*0.35), "text-anchor", anchor, "font-size", fs)
		s.Text(lines[0])
		s.Close("text")
		return
	}
	s.Open("text", "class", cls, "text-anchor", anchor, "font-size", fs)
	for i, ln := range lines {
		s.Open("tspan", "x", diagram.F1(x), "y", diagram.F1(top+float64(lineH*(float64(i)+0.5))+fontSize*0.35))
		s.Text(ln)
		s.Close("tspan")
	}
	s.Close("text")
}

// centerLines 는 가운데 c 에 줄들을 쓴다.
func centerLines(s *diagram.Writer, cls string, c layered.Pt, lines []string, fs string) {
	topLines(s, cls, c.X, c.Y-float64(float64(len(lines))*lineH)/2, lines, "middle", fs)
}
