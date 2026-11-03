package flowchart

import (
	"math"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 계층 배치(Sugiyama)는 공통 패키지 diagram/layered 다(itda-hyve#409 ④ — ER 이 같은 배치를 쓰게 꺼냈다). 여기는 흐름도 노드의
// 크기(모양·라벨 줄)와 간선 라벨 줄을 정해 넘기고, 간선 끝을 모양의 테두리에서 자르는 일(clipShape)을 맡는다.

const (
	fontSize  = diagram.FontSize // 공통(diagram) — 글자 폭 어림의 기준
	lineH     = diagram.LineH    // 공통
	padX      = 14.0
	padY      = 9.0
	wrapW     = 176.0 // 노드 라벨의 줄 너비(한글 약 12.5자) — 넘으면 낱말에서 줄을 바꾼다
	edgeWrapW = 140.0
	nodeSep   = 40.0
	rankSep   = 44.0 // 노드 계층 사이(가운데 라벨 계층의 반씩)
)

// pt 는 흐름도 안의 점이다(layered.Pt 와 같은 꼴 — 경계에서 바꾼다).
type pt struct{ X, Y float64 }

func toLayered(ps []pt) []layered.Pt {
	out := make([]layered.Pt, len(ps))
	for i, p := range ps {
		out[i] = layered.Pt(p)
	}
	return out
}

// laid 는 배치 결과다(최종 좌표, 왼쪽 위가 0,0 근처).
type laid struct {
	W, H     float64
	nodes    []nodeBox
	edges    []edgePath
	clusters []clusterBox
}

type nodeBox struct {
	c     pt
	w, h  float64
	lines []string
}

type edgePath struct {
	pts    []pt // 지나는 점(원래 방향 — From 쪽이 처음). 자기 간선은 3차 곡선 하나의 네 점
	self   bool
	label  []string
	lc     pt // 라벨 가운데
	lw, lh float64
}

type clusterBox struct {
	x, y, w, h float64
	title      []string
	show       bool
}

func layout(g *Graph) (*laid, error) {
	in := &layered.Graph{Dir: layered.Dir(g.Dir), NodeSep: nodeSep, RankSep: rankSep, // Dir 은 같은 차례(TB·BT·LR·RL)
		Nodes: make([]layered.Node, len(g.Nodes)), Edges: make([]layered.Edge, len(g.Edges)), Clusters: make([]layered.Cluster, len(g.Clusters))}
	lines := make([][]string, len(g.Nodes))
	for i, n := range g.Nodes {
		lines[i] = wrapLabel(n.Label, wrapW)
		sz := shapeSize(n.Shape, lines[i])
		in.Nodes[i] = layered.Node{W: sz[0], H: sz[1], Cluster: n.Cluster}
	}
	for i, e := range g.Edges {
		le := layered.Edge{From: e.From, To: e.To, Length: e.Length, Hidden: e.Stroke == StrokeInvisible}
		if e.Label != "" && e.Stroke != StrokeInvisible {
			le.Label = wrapLabel(e.Label, edgeWrapW)
		}
		in.Edges[i] = le
	}
	for i, c := range g.Clusters {
		in.Clusters[i] = layered.Cluster{Title: c.Title}
	}
	in.Clip = func(i int, b layered.Box, to layered.Pt) layered.Pt {
		return layered.Pt(clipShape(nodeBox{c: pt(b.C), w: b.W, h: b.H}, g.Nodes[i].Shape, pt(to)))
	}
	r, err := layered.Layout(in)
	if err != nil {
		return nil, err
	}
	out := &laid{W: r.W, H: r.H, nodes: make([]nodeBox, len(r.Nodes)), edges: make([]edgePath, len(r.Edges)), clusters: make([]clusterBox, len(r.Clusters))}
	for i, n := range r.Nodes {
		out.nodes[i] = nodeBox{c: pt(n.C), w: n.W, h: n.H, lines: lines[i]}
	}
	for i, e := range r.Edges {
		var pts []pt
		for _, p := range e.Pts {
			pts = append(pts, pt(p))
		}
		out.edges[i] = edgePath{pts: pts, self: e.Self, label: e.Label, lc: pt(e.LC), lw: e.LW, lh: e.LH}
	}
	for i, c := range r.Clusters {
		out.clusters[i] = clusterBox{x: c.X, y: c.Y, w: c.W, h: c.H, title: c.Title, show: c.Show}
	}
	return out, nil
}

func clipShape(n nodeBox, sh Shape, to pt) pt {
	c := n.c
	dx, dy := to.X-c.X, to.Y-c.Y
	if dx == 0 && dy == 0 {
		return c
	}
	hw, hh := n.w/2, n.h/2
	switch sh {
	case ShapeCircle:
		d := hyp(dx, dy)
		return pt{c.X + float64(dx/d*hw), c.Y + float64(dy/d*hw)}
	case ShapeDiamond:
		t := 1 / (math.Abs(dx)/hw + math.Abs(dy)/hh)
		return pt{c.X + float64(dx*t), c.Y + float64(dy*t)}
	case ShapeHexagon, ShapeLean, ShapeLeanAlt, ShapeTrap, ShapeTrapAlt:
		poly := shapePoly(n, sh)
		best := math.Inf(1)
		for i := range poly {
			p, q := poly[i], poly[(i+1)%len(poly)]
			if t, ok := rayHit(c, pt{dx, dy}, p, q); ok && t < best {
				best = t
			}
		}
		if !math.IsInf(best, 1) {
			return pt{c.X + float64(dx*best), c.Y + float64(dy*best)}
		}
	}
	t := layered.RectT(hw, hh, dx, dy)
	// 둥근 모양은 사각형 교점이 둥근 부분에 들면 그 원호·타원호와 다시 교차시킨다(리뷰 R2 — 경기장형의 고리 끝이 4.2 떨어졌다)
	switch sh {
	case ShapeStadium, ShapeRound:
		rx, ry := cornerRadii(sh, n.w, n.h)
		px, py := float64(dx*t), float64(dy*t) // 가운데 기준 사각형 교점
		if math.Abs(px) > hw-rx+1e-9 && math.Abs(py) > hh-ry+1e-9 {
			ox, oy := math.Copysign(hw-rx, dx), math.Copysign(hh-ry, dy) // 모서리 타원의 가운데(노드 가운데 기준)
			if s, ok := rayEllipse(-ox, -oy, dx, dy, rx, ry); ok {
				t = s
			}
		}
	case ShapeCylinder:
		ry := cylRy(n.w)
		py := float64(dy * t)
		if ey := hh - ry; math.Abs(py) > ey+1e-9 { // 위(아래) 타원호: 가운데 (0, ∓ey), 반지름 hw·ry
			if s, ok := rayEllipse(0, -math.Copysign(ey, dy), dx, dy, hw, ry); ok {
				t = s
			}
		}
	}
	return pt{c.X + float64(dx*t), c.Y + float64(dy*t)}
}

// roundRx 는 둥근 사각형 노드(A(글))의 모서리 반지름이다 — svg.go 의 rx 와 같다.
const roundRx = 7.0

// cornerRadii 는 svg.go 가 그리는 둥근 rect 의 실제 모서리 반지름(x, y)이다. SVG 규칙대로: rx 만 적으면 ry = rx, 그다음
// rx 는 너비의 반, ry 는 높이의 반을 넘지 않게 따로 줄인다(SVG 2 rect). 경기장형은 rx = 높이/2 라 높이가 너비보다 크면
// 모서리가 원이 아니라 타원이다(리뷰 R3).
func cornerRadii(sh Shape, w, h float64) (float64, float64) {
	r := roundRx
	if sh == ShapeStadium {
		r = h / 2
	}
	return math.Min(r, w/2), math.Min(r, h/2)
}

// rayEllipse 는 가운데에서 (dx,dy) 방향의 반직선이 타원((x-ex)/a)²+((y-ey)/b)²=1 과 만나는 큰 쪽의 t 다.
// (x0,y0) 은 반직선 시작점을 타원 가운데 기준으로 잰 것(시작점 - 타원 가운데)이다.
func rayEllipse(x0, y0, dx, dy, a, b float64) (float64, bool) {
	u0, v0, du, dv := x0/a, y0/b, dx/a, dy/b
	qa := float64(du*du) + float64(dv*dv)
	qb := 2 * (float64(u0*du) + float64(v0*dv))
	qc := float64(u0*u0) + float64(v0*v0) - 1
	disc := float64(qb*qb) - float64(4*qa*qc)
	if qa == 0 || disc < 0 {
		return 0, false
	}
	return (-qb + math.Sqrt(disc)) / (2 * qa), true
}

// rayHit 은 c + t·d 가 선분 pq 와 만나는 t(> 0)다.
func rayHit(c, d, p, q pt) (float64, bool) {
	ex, ey := q.X-p.X, q.Y-p.Y
	den := float64(d.X*ey) - float64(d.Y*ex)
	if math.Abs(den) < 1e-12 {
		return 0, false
	}
	t := (float64((p.X-c.X)*ey) - float64((p.Y-c.Y)*ex)) / den
	u := (float64((p.X-c.X)*d.Y) - float64((p.Y-c.Y)*d.X)) / den
	if t <= 0 || u < -1e-9 || u > 1+1e-9 {
		return 0, false
	}
	return t, true
}
