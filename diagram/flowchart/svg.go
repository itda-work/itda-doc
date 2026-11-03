package flowchart

import (
	"strings"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// SVG 쓰기. 쓰는 요소·속성은 이것뿐이다(mdoc/security_test.go 의 허용 목록이 고정한다 — 차트와 같은 목록에 경로의 C):
//
//	요소 svg·g·rect·path·line·circle·text·tspan·title
//	속성 viewBox·width·height·class·role·aria-label·x·y·rx·d·x1·y1·x2·y2·cx·cy·r·text-anchor·font-size
//
// style·이벤트 속성·url()·href·id 를 쓰지 않는다(화살촉은 marker 가 아니라 삼각형 경로라 id 가 필요 없다 — 한 문서에
// 그림이 여럿이어도 겹치는 id 가 없다). 색·선은 클래스이고 값은 테마 토큰(--diagram-*)이 준다(base.css):
//
//	cl-box·cl-title  subgraph 상자·제목      edge(+dotted·thick)·head  화살표 선·화살촉
//	el-bg·el         간선 라벨 바탕·글       node(+rim)·node-in·nl      노드 모양(원통 테두리, 서브루틴 안쪽 선)·글
//
// SVG 를 쌓는 도구(Writer·Esc·F1)는 공통 패키지(diagram)다 — 간트 차트가 함께 쓴다(itda-hyve#409).

func drawSVG(g *Graph, l *laid) string {
	s := &diagram.Writer{}
	label := g.AccTitle
	if label == "" {
		label = g.Title
	}
	if label == "" {
		label = "흐름도"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(l.W)+" "+diagram.F1(l.H), "width", diagram.F1(l.W*diagram.Scale), "height", diagram.F1(l.H*diagram.Scale),
		"role", "img", "aria-label", label)
	if g.AccDescr != "" {
		s.Open("title")
		s.Text(g.AccDescr)
		s.Close("title")
	}
	fs := diagram.F1(fontSize)

	if len(l.clusters) > 0 {
		s.Open("g", "class", "clusters")
		for _, c := range l.clusters {
			if !c.show {
				continue
			}
			s.Leaf("rect", "class", "cl-box", "x", diagram.F1(c.x), "y", diagram.F1(c.y), "width", diagram.F1(c.w), "height", diagram.F1(c.h), "rx", "4")
			for i, t := range c.title {
				s.Open("text", "class", "cl-title", "x", diagram.F1(c.x+8), "y", diagram.F1(c.y+2+float64(lineH*(float64(i)+0.5))+fontSize*0.35), "font-size", diagram.F1(fontSize-1))
				s.Text(t)
				s.Close("text")
			}
		}
		s.Close("g")
	}

	var heads []string
	s.Open("g", "class", "edges")
	for i, e := range g.Edges {
		if e.Stroke == StrokeInvisible {
			continue
		}
		ep := l.edges[i]
		cls := "edge"
		switch e.Stroke {
		case StrokeDotted:
			cls += " dotted"
		case StrokeThick:
			cls += " thick"
		}
		pts := append([]pt(nil), ep.pts...)
		thick := e.Stroke == StrokeThick
		if e.Arrow {
			heads = append(heads, arrowHead(pts[len(pts)-1], pts[len(pts)-2], thick))
			pts[len(pts)-1] = pullBack(pts[len(pts)-1], pts[len(pts)-2], 6)
		}
		if e.Both {
			heads = append(heads, arrowHead(pts[0], pts[1], thick))
			pts[0] = pullBack(pts[0], pts[1], 6)
		}
		s.Leaf("path", "class", cls, "d", layered.Basis(toLayered(pts)))
	}
	s.Close("g")
	if len(heads) > 0 {
		s.Open("g", "class", "heads")
		for _, h := range heads {
			s.Leaf("path", "class", "head", "d", h)
		}
		s.Close("g")
	}

	labeled := false
	for i, e := range g.Edges {
		if e.Stroke != StrokeInvisible && len(l.edges[i].label) > 0 {
			labeled = true
		}
	}
	if labeled {
		s.Open("g", "class", "elabels")
		for i, e := range g.Edges {
			ep := l.edges[i]
			if e.Stroke == StrokeInvisible || len(ep.label) == 0 {
				continue
			}
			s.Leaf("rect", "class", "el-bg", "x", diagram.F1(ep.lc.X-ep.lw/2), "y", diagram.F1(ep.lc.Y-ep.lh/2), "width", diagram.F1(ep.lw), "height", diagram.F1(ep.lh), "rx", "2")
			writeLines(s, "el", ep.lc, ep.label, fs)
		}
		s.Close("g")
	}

	s.Open("g", "class", "nodes")
	for i, n := range g.Nodes {
		nb := l.nodes[i]
		x0, y0 := nb.c.X-nb.w/2, nb.c.Y-nb.h/2
		tc := nb.c
		switch n.Shape {
		case ShapeRect:
			s.Leaf("rect", "class", "node", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(nb.w), "height", diagram.F1(nb.h))
		case ShapeRound:
			s.Leaf("rect", "class", "node", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(nb.w), "height", diagram.F1(nb.h), "rx", diagram.F1(roundRx))
		case ShapeStadium:
			s.Leaf("rect", "class", "node", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(nb.w), "height", diagram.F1(nb.h), "rx", diagram.F1(nb.h/2))
		case ShapeSubroutine:
			s.Leaf("rect", "class", "node", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(nb.w), "height", diagram.F1(nb.h))
			for _, x := range []float64{x0 + 7, x0 + nb.w - 7} {
				s.Leaf("line", "class", "node-in", "x1", diagram.F1(x), "y1", diagram.F1(y0), "x2", diagram.F1(x), "y2", diagram.F1(y0+nb.h))
			}
		case ShapeCircle:
			s.Leaf("circle", "class", "node", "cx", diagram.F1(nb.c.X), "cy", diagram.F1(nb.c.Y), "r", diagram.F1(nb.w/2))
		case ShapeCylinder:
			ry := cylRy(nb.w)
			rx := nb.w / 2
			x1, y1 := x0+nb.w, y0+nb.h
			arc := " A" + diagram.F1(rx) + " " + diagram.F1(ry) + " 0 0 "
			s.Leaf("path", "class", "node", "d", "M"+diagram.F1(x0)+" "+diagram.F1(y0+ry)+arc+"1 "+diagram.F1(x1)+" "+diagram.F1(y0+ry)+" L"+diagram.F1(x1)+" "+diagram.F1(y1-ry)+
				arc+"1 "+diagram.F1(x0)+" "+diagram.F1(y1-ry)+" Z")
			s.Leaf("path", "class", "node rim", "d", "M"+diagram.F1(x0)+" "+diagram.F1(y0+ry)+arc+"0 "+diagram.F1(x1)+" "+diagram.F1(y0+ry))
			tc.Y += ry / 2
		default: // 마름모·육각형·평행사변형·사다리꼘
			s.Leaf("path", "class", "node", "d", polyPath(shapePoly(nb, n.Shape)))
		}
		writeLines(s, "nl", tc, nb.lines, fs)
	}
	s.Close("g")
	s.Close("svg")
	return diagram.AriaRefs(s.String()) // aria-label 의 탭·줄바꿈은 문자 참조로(공통 — state 와 같다)
}

// xmlChars 는 XML 1.0 의 Char 밖 글자를 U+FFFD 로 바꾼다(공통 diagram — 시험이 쓴다).
func xmlChars(s string) string { return diagram.XmlChars(s) }

// writeLines 는 가운데 c 에 줄들을 쓴다(줄마다 tspan). 기준선은 글자 크기의 0.35 아래 — dominant-baseline 을 쓰지 않는다.
func writeLines(s *diagram.Writer, cls string, c pt, lines []string, fs string) {
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

func p2(p pt) string { return diagram.F1(p.X) + " " + diagram.F1(p.Y) }

func polyPath(ps []pt) string {
	var b strings.Builder
	for i, p := range ps {
		if i == 0 {
			b.WriteString("M")
		} else {
			b.WriteString(" L")
		}
		b.WriteString(p2(p))
	}
	b.WriteString(" Z")
	return b.String()
}

// arrowHead 는 tip 에서 from 의 반대쪽을 향하는 화살촉 삼각형이다.
func arrowHead(tip, from pt, thick bool) string {
	dx, dy := tip.X-from.X, tip.Y-from.Y
	d := hyp(dx, dy)
	if d == 0 {
		dx, dy, d = 0, 1, 1
	}
	ux, uy := dx/d, dy/d
	ln, hw := 9.0, 4.5
	if thick {
		ln, hw = 11, 5.5
	}
	bx, by := tip.X-float64(ux*ln), tip.Y-float64(uy*ln)
	return polyPath([]pt{tip, {bx - float64(uy*hw), by + float64(ux*hw)}, {bx + float64(uy*hw), by - float64(ux*hw)}})
}

// pullBack 은 선 끝을 화살촉 안쪽으로 당긴다(선 끝이 화살촉 끝을 뚫고 나오지 않게). 짧은 선은 그대로.
func pullBack(tip, from pt, by float64) pt {
	dx, dy := tip.X-from.X, tip.Y-from.Y
	d := hyp(dx, dy)
	if d <= by*2 {
		return tip
	}
	return pt{tip.X - float64(dx/d*by), tip.Y - float64(dy/d*by)}
}
