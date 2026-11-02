package er

import (
	"strings"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 그리기. 쓰는 요소·속성은 흐름도·간트·타임라인·순서도와 같은 허용 목록이다(svg·g·rect·path·line·circle·text·tspan·title —
// mdoc/security_test.go 가 고정한다). 끝 기호(까마귀발)는 marker 가 아니라 path·circle 이다. 색은 클래스 → 토큰(base.css),
// 새 색 상수·새 토큰 없음. 클래스는 모두 er- 접두사(앱 CSS 충돌 — er_test.go TestClassCollisions):
//
//	er-rel(+er-dash)   관계 선(-- 실선 식별, .. 점선 비식별)        er-mk·er-mko  끝 기호 선·동그라미
//	er-rbg·er-role     역할 글 바탕(--paper 계열 — 선이 글자를 가로지르지 않게)·글자
//	er-box·er-rim      엔티티 상자 바탕(머리 칸 색)·테두리        er-row-a·er-row-b  속성 줄 번갈이 바탕
//	er-div             머리·열·줄 나눔 선                          er-name            머리 이름
//	er-at·er-key·er-cmt 속성 타입·이름 / 키 / 설명 글자
//
// 그리는 차례: 관계 선 → 끝 기호 → 역할 글 바탕 → 역할 글 → 엔티티 상자(상자가 마지막이라 선이 상자를 지나가도 가린다 —
// 원본도 엔티티를 선 위에 그린다).

// 끝 기호 크기(viewBox 단위). 원본 erMarkers 의 꼴(막대 둘 · 동그라미 + 막대 · 발 + 막대 · 발 + 동그라미)을 줄인 것.
const (
	markHalf = 8.0  // 까마귀발이 상자 테두리에서 벌어지는 반 너비
	barHalf  = 7.0  // 막대 반 길이
	footLen  = 14.0 // 까마귀발이 모이는 점까지
	circR    = 5.0  // 동그라미 반지름
)

func at(e end, d float64) layered.Pt {
	return layered.Pt{X: e.tip.X + float64(e.ux*d), Y: e.tip.Y + float64(e.uy*d)}
}

// side 는 p 에서 선의 옆(왼쪽이 +)으로 s 만큼 간 점이다.
func side(e end, p layered.Pt, s float64) layered.Pt {
	return layered.Pt{X: p.X - float64(e.uy*s), Y: p.Y + float64(e.ux*s)}
}

// markParts 는 끝 기호의 선분과 동그라미 가운데(없으면 ok 거짓)다 — tip 에서 선을 따라 상자 밖으로:
//
//	ExactlyOne  ||   막대 9·15
//	ZeroOrOne   o|   막대 9, 동그라미 19
//	OneOrMore   }|   발(0–14), 막대 18
//	ZeroOrMore  }o   발(0–14), 동그라미 20(끝 25 = markLen)
func markParts(e end) (segs [][2]layered.Pt, circ layered.Pt, ok bool) {
	bar := func(d float64) {
		c := at(e, d)
		segs = append(segs, [2]layered.Pt{side(e, c, barHalf), side(e, c, -barHalf)})
	}
	foot := func() {
		m := at(e, footLen)
		segs = append(segs, [2]layered.Pt{m, side(e, e.tip, markHalf)}, [2]layered.Pt{m, side(e, e.tip, -markHalf)})
	}
	switch e.card {
	case ExactlyOne:
		bar(9)
		bar(15)
	case ZeroOrOne:
		bar(9)
		circ, ok = at(e, 19), true
	case OneOrMore:
		foot()
		bar(18)
	case ZeroOrMore:
		foot()
		circ, ok = at(e, 20), true
	}
	return segs, circ, ok
}

// markExtent 는 끝 기호를 담는 네 점이다(viewBox 맞춤용).
func markExtent(e end) []layered.Pt {
	far := at(e, markLen)
	return []layered.Pt{side(e, e.tip, markHalf), side(e, e.tip, -markHalf), side(e, far, markHalf), side(e, far, -markHalf)}
}

func p2(p layered.Pt) string { return diagram.F1(p.X) + " " + diagram.F1(p.Y) }

func drawSVG(c *chart, l *laid) string {
	s := &diagram.Writer{}
	label := c.AccTitle
	if label == "" {
		label = c.FMCaption
	}
	if label == "" {
		label = "ER 다이어그램"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(l.W)+" "+diagram.F1(l.H), "width", diagram.F1(l.W*diagram.Scale),
		"height", diagram.F1(l.H*diagram.Scale), "role", "img", "aria-label", label)
	if c.AccDescr != "" {
		s.Open("title")
		s.Text(c.AccDescr)
		s.Close("title")
	}
	fs := diagram.F1(fontSize)

	if len(c.Relations) > 0 {
		s.Open("g", "class", "er-rels")
		for i, r := range c.Relations {
			cls := "er-rel"
			if !r.Identifying {
				cls += " er-dash"
			}
			s.Leaf("path", "class", cls, "d", layered.Basis(l.edges[i].Pts))
		}
		s.Close("g")
		s.Open("g", "class", "er-marks")
		for i := range c.Relations {
			for _, e := range l.ends[i] {
				segs, circ, ok := markParts(e)
				var d strings.Builder
				for k, sg := range segs {
					if k > 0 {
						d.WriteByte(' ')
					}
					d.WriteString("M" + p2(sg[0]) + " L" + p2(sg[1]))
				}
				s.Leaf("path", "class", "er-mk", "d", d.String())
				if ok {
					s.Leaf("circle", "class", "er-mko", "cx", diagram.F1(circ.X), "cy", diagram.F1(circ.Y), "r", diagram.F1(circR))
				}
			}
		}
		s.Close("g")
		labeled := false
		for i := range c.Relations {
			labeled = labeled || len(l.edges[i].Label) > 0
		}
		if labeled {
			s.Open("g", "class", "er-roles")
			for i := range c.Relations {
				ep := l.edges[i]
				if len(ep.Label) == 0 {
					continue
				}
				s.Leaf("rect", "class", "er-rbg", "x", diagram.F1(ep.LC.X-ep.LW/2), "y", diagram.F1(ep.LC.Y-ep.LH/2),
					"width", diagram.F1(ep.LW), "height", diagram.F1(ep.LH), "rx", "2")
				centerLines(s, "er-role", ep.LC, ep.Label, fs)
			}
			s.Close("g")
		}
	}

	s.Open("g", "class", "er-ents")
	for i := range c.Entities {
		drawEntity(s, l.boxes[i], l.at[i], fs)
	}
	s.Close("g")
	s.Close("svg")
	return diagram.AriaRefs(s.String()) // aria-label 의 탭·줄바꿈은 문자 참조로(공통 — state 와 같다)
}

func drawEntity(s *diagram.Writer, b box, c layered.Pt, fs string) {
	x0, y0 := c.X-b.w/2, c.Y-b.h/2
	rect := func(cls string, x, y, w, h float64) {
		s.Leaf("rect", "class", cls, "x", diagram.F1(x), "y", diagram.F1(y), "width", diagram.F1(w), "height", diagram.F1(h))
	}
	rect("er-box", x0, y0, b.w, b.h)
	if len(b.rows) > 0 {
		y := y0 + b.headH
		for i, r := range b.rows {
			cls := "er-row-a"
			if i%2 == 1 {
				cls = "er-row-b"
			}
			rect(cls, x0, y, b.w, r.h)
			y += r.h
		}
		// 나눔 선: 머리 아래, 열 사이(속성 표 높이만), 줄 사이
		var d strings.Builder
		line := func(ax, ay, bx, by float64) {
			if d.Len() > 0 {
				d.WriteByte(' ')
			}
			d.WriteString("M" + diagram.F1(ax) + " " + diagram.F1(ay) + " L" + diagram.F1(bx) + " " + diagram.F1(by))
		}
		top, bot := y0+b.headH, y0+b.h
		line(x0, top, x0+b.w, top)
		x := x0
		for _, w := range b.cols[:len(b.cols)-1] {
			x += w
			line(x, top, x, bot)
		}
		y = top
		for _, r := range b.rows[:len(b.rows)-1] {
			y += r.h
			line(x0, y, x0+b.w, y)
		}
		s.Leaf("path", "class", "er-div", "d", d.String())
	}
	rect("er-rim", x0, y0, b.w, b.h)
	centerLines(s, "er-name", layered.Pt{X: c.X, Y: y0 + b.headH/2}, b.name, fs)
	y := y0 + b.headH
	for _, r := range b.rows {
		x := x0
		for k, cell := range r.cells {
			cls := "er-at"
			switch {
			case k == 2 && b.hasKey:
				cls = "er-key"
			case k >= 2:
				cls = "er-cmt"
			}
			if len(cell) == 1 && cell[0] == "" {
				x += b.cols[k]
				continue
			}
			tx := diagram.F1(x + cellPadX)
			if len(cell) == 1 {
				s.Open("text", "class", cls, "x", tx, "y", diagram.F1(y+rowPadY+lineH/2+fontSize*0.35), "font-size", fs)
				s.Text(cell[0])
				s.Close("text")
			} else {
				s.Open("text", "class", cls, "font-size", fs)
				for i, ln := range cell {
					s.Open("tspan", "x", tx, "y", diagram.F1(y+rowPadY+float64(lineH*(float64(i)+0.5))+fontSize*0.35))
					s.Text(ln)
					s.Close("tspan")
				}
				s.Close("text")
			}
			x += b.cols[k]
		}
		y += r.h
	}
}

// centerLines 는 가운데 c 에 줄들을 쓴다(줄마다 tspan). 기준선은 글자 크기의 0.35 아래 — dominant-baseline 을 쓰지 않는다
// (흐름도 writeLines 와 같은 규칙).
func centerLines(s *diagram.Writer, cls string, c layered.Pt, lines []string, fs string) {
	top := c.Y - float64(float64(len(lines))*lineH)/2
	if len(lines) == 1 {
		s.Open("text", "class", cls, "x", diagram.F1(c.X), "y", diagram.F1(top+lineH/2+fontSize*0.35), "text-anchor", "middle", "font-size", fs)
		s.Text(lines[0])
		s.Close("text")
		return
	}
	s.Open("text", "class", cls, "text-anchor", "middle", "font-size", fs)
	for i, ln := range lines {
		s.Open("tspan", "x", diagram.F1(c.X), "y", diagram.F1(top+float64(lineH*(float64(i)+0.5))+fontSize*0.35))
		s.Text(ln)
		s.Close("tspan")
	}
	s.Close("text")
}
