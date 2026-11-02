package state

import (
	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 그리기. 쓰는 요소·속성은 흐름도·간트·타임라인·순서도·ER 과 같은 허용 목록이다(svg·g·rect·path·line·circle·text·tspan·
// title — mdoc/security_test.go 가 고정한다). 화살촉은 marker 가 아니라 삼각형 path 다(흐름도 arrowHead 와 같은 방식 — 고정
// id 가 한 문서의 여러 그림에서 겹치지 않게). 색은 클래스 → 토큰(base.css), 새 색 상수·새 토큰 없음. 클래스는 모두 st- 접두사
// (앱 CSS 충돌 — state_test.go TestClassCollisions):
//
//	st-cl·st-cl-h·st-cl-d·st-cl-t  복합 상태 상자·제목 칸·제목 칸 아래 선·제목 글
//	st-tr·st-head                  전이 선·화살촉           st-lbg·st-lbl  전이 라벨 바탕(--paper 계열)·글자
//	st-node·st-div·st-name·st-desc 상태 상자·머리 아래 구분선·이름(머리) 글·설명 글
//	st-start·st-end·st-end-in      시작 원(선 색으로 채움)·끝 바깥 원·끝 안 원
//	st-choice·st-bar               선택 마름모·갈래·합류 막대(선 색으로 채움)
//
// 그리는 차례: 복합 상태 → 전이 선 → 화살촉 → 라벨 바탕 → 라벨 글 → 상태(상태가 마지막이라 선이 상자를 지나가도 가린다 —
// 흐름도와 같다).

// 화살촉 크기(흐름도 arrowHead 의 보통 선과 같다)와 선 끝을 당기는 길이.
const (
	headLen  = 9.0
	headHalf = 4.5
	pullBy   = 6.0
)

func p2(p layered.Pt) string { return diagram.F1(p.X) + " " + diagram.F1(p.Y) }

// arrowHead 는 tip 에서 from 의 반대쪽을 향하는 화살촉 삼각형이다(흐름도 arrowHead 와 같은 꼴).
func arrowHead(tip, from layered.Pt) string {
	dx, dy := tip.X-from.X, tip.Y-from.Y
	d := hyp(dx, dy)
	if d == 0 {
		dx, dy, d = 0, 1, 1
	}
	ux, uy := dx/d, dy/d
	bx, by := tip.X-float64(ux*headLen), tip.Y-float64(uy*headLen)
	return "M" + p2(tip) + " L" + p2(layered.Pt{X: bx - float64(uy*headHalf), Y: by + float64(ux*headHalf)}) +
		" L" + p2(layered.Pt{X: bx + float64(uy*headHalf), Y: by - float64(ux*headHalf)}) + " Z"
}

// pullBack 은 선 끝을 화살촉 안쪽으로 당긴다(선 끝이 화살촉 끝을 뚫고 나오지 않게). 짧은 선은 그대로(흐름도와 같다).
func pullBack(tip, from layered.Pt, by float64) layered.Pt {
	dx, dy := tip.X-from.X, tip.Y-from.Y
	d := hyp(dx, dy)
	if d <= by*2 {
		return tip
	}
	return layered.Pt{X: tip.X - float64(dx/d*by), Y: tip.Y - float64(dy/d*by)}
}

// titleHt 는 layered 가 복합 상태 상자 위에 남긴 제목 띠의 높이다(layered rankAxis 의 setTitle 과 같은 셈).
func titleHt(lines []string) float64 { return float64(float64(len(lines))*lineH) + 4 }

func drawSVG(c *chart, l *laid) string {
	s := &diagram.Writer{}
	label := c.AccTitle
	if label == "" {
		label = c.FMCaption
	}
	if label == "" {
		label = "상태도"
	}
	s.Open("svg", "class", "diagram-svg", "viewBox", "0 0 "+diagram.F1(l.W)+" "+diagram.F1(l.H), "width", diagram.F1(l.W*diagram.Scale),
		"height", diagram.F1(l.H*diagram.Scale), "role", "img", "aria-label", label)
	if c.AccDescr != "" {
		s.Open("title")
		s.Text(c.AccDescr)
		s.Close("title")
	}
	fs := diagram.F1(fontSize)

	if len(l.clusters) > 0 {
		s.Open("g", "class", "st-cls")
		for _, cb := range l.clusters {
			if !cb.Show {
				continue
			}
			x0, y0, x1 := cb.X, cb.Y, cb.X+cb.W
			s.Leaf("rect", "class", "st-cl", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(cb.W), "height", diagram.F1(cb.H), "rx", "6")
			th := titleHt(cb.Title)
			yb := y0 + th
			s.Leaf("path", "class", "st-cl-h", "d", "M"+diagram.F1(x0)+" "+diagram.F1(yb)+" L"+diagram.F1(x0)+" "+diagram.F1(y0+6)+
				" A6 6 0 0 1 "+diagram.F1(x0+6)+" "+diagram.F1(y0)+" L"+diagram.F1(x1-6)+" "+diagram.F1(y0)+
				" A6 6 0 0 1 "+diagram.F1(x1)+" "+diagram.F1(y0+6)+" L"+diagram.F1(x1)+" "+diagram.F1(yb)+" Z")
			s.Leaf("line", "class", "st-cl-d", "x1", diagram.F1(x0), "y1", diagram.F1(yb), "x2", diagram.F1(x1), "y2", diagram.F1(yb))
			centerLines(s, "st-cl-t", layered.Pt{X: (x0 + x1) / 2, Y: y0 + th/2}, cb.Title, fs)
		}
		s.Close("g")
	}

	if len(c.Trans) > 0 {
		heads := make([]string, len(c.Trans))
		s.Open("g", "class", "st-trs")
		for i := range c.Trans {
			pts := append([]layered.Pt(nil), l.edges[i].Pts...)
			n := len(pts)
			heads[i] = arrowHead(pts[n-1], pts[n-2])
			pts[n-1] = pullBack(pts[n-1], pts[n-2], pullBy)
			s.Leaf("path", "class", "st-tr", "d", layered.Basis(pts))
		}
		s.Close("g")
		s.Open("g", "class", "st-heads")
		for _, h := range heads {
			s.Leaf("path", "class", "st-head", "d", h)
		}
		s.Close("g")
		labeled := false
		for i := range c.Trans {
			labeled = labeled || len(l.edges[i].Label) > 0
		}
		if labeled {
			s.Open("g", "class", "st-lbls")
			for i := range c.Trans {
				ep := l.edges[i]
				if len(ep.Label) == 0 {
					continue
				}
				s.Leaf("rect", "class", "st-lbg", "x", diagram.F1(ep.LC.X-ep.LW/2), "y", diagram.F1(ep.LC.Y-ep.LH/2),
					"width", diagram.F1(ep.LW), "height", diagram.F1(ep.LH), "rx", "2")
				centerLines(s, "st-lbl", ep.LC, ep.Label, fs)
			}
			s.Close("g")
		}
	}

	s.Open("g", "class", "st-nodes")
	for i := range c.States {
		drawState(s, l.boxes[i], l.at[i], fs)
	}
	s.Close("g")
	s.Close("svg")
	return ariaRefs(s.String())
}

// ariaRefs 는 aria-label 값 안의 탭·LF·CR 을 문자 참조로 쓴다(공통 diagram.AriaRefs — 리뷰 5회차 [2]).
func ariaRefs(svg string) string { return diagram.AriaRefs(svg) }

func drawState(s *diagram.Writer, b box, c layered.Pt, fs string) {
	x0, y0 := c.X-b.w/2, c.Y-b.h/2
	switch b.kind {
	case Start:
		s.Leaf("circle", "class", "st-start", "cx", diagram.F1(c.X), "cy", diagram.F1(c.Y), "r", diagram.F1(startD/2))
		return
	case End:
		s.Leaf("circle", "class", "st-end", "cx", diagram.F1(c.X), "cy", diagram.F1(c.Y), "r", diagram.F1(endD/2))
		s.Leaf("circle", "class", "st-end-in", "cx", diagram.F1(c.X), "cy", diagram.F1(c.Y), "r", diagram.F1(endInD/2))
		return
	case Choice:
		s.Leaf("path", "class", "st-choice", "d", "M"+diagram.F1(c.X)+" "+diagram.F1(y0)+" L"+diagram.F1(x0+b.w)+" "+diagram.F1(c.Y)+
			" L"+diagram.F1(c.X)+" "+diagram.F1(y0+b.h)+" L"+diagram.F1(x0)+" "+diagram.F1(c.Y)+" Z")
		return
	case Fork, Join:
		s.Leaf("rect", "class", "st-bar", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(b.w), "height", diagram.F1(b.h), "rx", "2")
		return
	}
	s.Leaf("rect", "class", "st-node", "x", diagram.F1(x0), "y", diagram.F1(y0), "width", diagram.F1(b.w), "height", diagram.F1(b.h), "rx", diagram.F1(roundR))
	if len(b.descr) == 0 {
		centerLines(s, "st-name", c, b.name, fs)
		return
	}
	yd := y0 + b.headH
	s.Leaf("line", "class", "st-div", "x1", diagram.F1(x0), "y1", diagram.F1(yd), "x2", diagram.F1(x0+b.w), "y2", diagram.F1(yd))
	centerLines(s, "st-name", layered.Pt{X: c.X, Y: y0 + padY + float64(float64(len(b.name))*lineH)/2}, b.name, fs)
	var lines []string
	for _, d := range b.descr {
		lines = append(lines, d...)
	}
	centerLines(s, "st-desc", layered.Pt{X: c.X, Y: yd + divGap + float64(float64(len(lines))*lineH)/2}, lines, fs)
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
