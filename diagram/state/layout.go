package state

import (
	"math"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 배치 상수(viewBox 단위, 글자 14 기준). 모양 크기는 원본 v3 렌더의 꼴을 따른다 — 시작 원 지름 14(stateStart), 끝 이중 원
// 지름 14·안 원 5 를 조금 키운 16·9(stateEnd), 선택 마름모 28(choice), 갈래·합류 막대 70×10(가로 그림은 10×70 — forkJoin).
// 상태 상자는 둥근 사각형(rx 10 — dataFetcher)이고 설명이 둘 이상이면 머리 + 구분선 + 설명 줄(rectWithTitle)이다.
const (
	fontSize  = diagram.FontSize
	lineH     = diagram.LineH
	nameWrap  = 200.0 // 상태 이름·설명 줄바꿈 폭
	labelWrap = 140.0 // 전이 라벨 줄바꿈 폭(흐름도 화살표 글과 같다)
	padX      = 14.0  // 상자 안 글자 좌우 여백(한쪽)
	padY      = 8.0   // 상자 안 글자 위아래 여백(한쪽)
	divGap    = 4.0   // 머리와 구분선 사이·구분선과 설명 사이
	minW      = 56.0  // 상태 상자의 최소 폭
	roundR    = 10.0  // 상태 상자 모서리 반지름(원본 rx·ry 10)
	startD    = 14.0  // 시작 원 지름
	endD      = 16.0  // 끝 바깥 원 지름
	endInD    = 9.0   // 끝 안 원 지름
	choiceD   = 28.0  // 선택 마름모 대각선
	barLong   = 70.0  // 갈래·합류 막대 길이
	barShort  = 10.0  // 갈래·합류 막대 두께
	nodeSep   = 44.0  // 이웃 상자 사이
	rankSep   = 48.0  // 상자 계층 사이(라벨 계층의 반씩)
)

// box 는 상태 하나의 모양이다(가운데는 배치가 정한다).
type box struct {
	kind  Kind
	w, h  float64
	name  []string   // 이름 줄(머리) — 설명이 없으면 ID, 하나면 그 설명, 둘 이상이면 첫 설명
	descr [][]string // 머리 아래 설명 줄(설명이 둘 이상일 때 둘째부터 — 설명마다 줄바꿈한 줄)
	headH float64    // 머리 칸 높이(설명 줄이 있을 때 구분선 자리)
}

// laid 는 배치 결과다(최종 좌표).
type laid struct {
	W, H     float64
	boxes    []box
	at       []layered.Pt // 상자 가운데
	edges    []layered.EdgePath
	clusters []layered.ClusterBox
	vert     bool // 세로 흐름(TB·BT) — 갈래·합류 막대가 가로
}

func stateBox(st *node, vert bool) box {
	b := box{kind: st.Kind}
	switch st.Kind {
	case Start:
		b.w, b.h = startD, startD
		return b
	case End:
		b.w, b.h = endD, endD
		return b
	case Choice:
		b.w, b.h = choiceD, choiceD
		return b
	case Fork, Join:
		b.w, b.h = barLong, barShort
		if !vert {
			b.w, b.h = barShort, barLong
		}
		return b
	}
	head := st.ID
	if len(st.Descr) > 0 {
		head = st.Descr[0]
	}
	b.name = diagram.WrapLabel(head, nameWrap)
	w := diagram.LinesW(b.name)
	n := len(b.name)
	if len(st.Descr) > 1 {
		dn := 0
		for _, d := range st.Descr[1:] {
			ls := diagram.WrapLabel(d, nameWrap)
			b.descr = append(b.descr, ls)
			w = math.Max(w, diagram.LinesW(ls))
			dn += len(ls)
		}
		b.headH = float64(float64(n)*lineH) + padY + divGap
		b.h = math.Ceil(b.headH + float64(float64(dn)*lineH) + divGap + padY)
	} else {
		b.h = math.Ceil(float64(float64(n)*lineH) + 2*padY)
	}
	b.w = math.Ceil(math.Max(w+2*padX, minW))
	return b
}

func layout(c *chart) (*laid, error) {
	vert := c.Dir == TB || c.Dir == BT
	in := &layered.Graph{Dir: layered.Dir(c.Dir), NodeSep: nodeSep, RankSep: rankSep, // Dir 은 같은 차례(TB·BT·LR·RL)
		Nodes: make([]layered.Node, len(c.States)), Edges: make([]layered.Edge, len(c.Trans)), Clusters: make([]layered.Cluster, len(c.Comps))}
	l := &laid{boxes: make([]box, len(c.States)), vert: vert}
	for i, st := range c.States {
		l.boxes[i] = stateBox(st, vert)
		in.Nodes[i] = layered.Node{W: l.boxes[i].w, H: l.boxes[i].h, Cluster: st.Cluster}
	}
	for i, tr := range c.Trans {
		le := layered.Edge{From: tr.From, To: tr.To, Length: 1}
		if tr.Label != "" {
			le.Label = diagram.WrapLabel(tr.Label, labelWrap)
		}
		in.Edges[i] = le
	}
	for i, cp := range c.Comps {
		in.Clusters[i] = layered.Cluster{Title: cp.Title}
	}
	in.Clip = func(i int, b layered.Box, to layered.Pt) layered.Pt { return clipShape(l.boxes[i].kind, b, to) }
	res, err := layered.Layout(in)
	if err != nil {
		return nil, err
	}
	l.W, l.H = res.W, res.H
	l.edges = res.Edges
	l.clusters = res.Clusters
	l.at = make([]layered.Pt, len(res.Nodes))
	for i, n := range res.Nodes {
		l.at[i] = n.C
	}
	return l, nil
}

// clipShape 는 상자 b(가운데 C)에서 to 쪽으로 간 선이 모양의 테두리와 만나는 점이다 — 원(시작·끝)·마름모(선택)·사각형
// (갈래·합류 막대)·둥근 사각형(상태 — 모서리 원호).
func clipShape(k Kind, b layered.Box, to layered.Pt) layered.Pt {
	c := b.C
	dx, dy := to.X-c.X, to.Y-c.Y
	if dx == 0 && dy == 0 {
		return c
	}
	hw, hh := b.W/2, b.H/2
	switch k {
	case Start, End:
		d := hyp(dx, dy)
		return layered.Pt{X: c.X + float64(dx/d*hw), Y: c.Y + float64(dy/d*hw)}
	case Choice:
		t := 1 / (math.Abs(dx)/hw + math.Abs(dy)/hh)
		return layered.Pt{X: c.X + float64(dx*t), Y: c.Y + float64(dy*t)}
	case Fork, Join:
		return layered.ClipRect(b, to)
	}
	t := layered.RectT(hw, hh, dx, dy)
	r := math.Min(roundR, math.Min(hw, hh))
	px, py := float64(dx*t), float64(dy*t) // 가운데 기준 사각형 교점
	if math.Abs(px) > hw-r+1e-9 && math.Abs(py) > hh-r+1e-9 {
		// 모서리 원호: 가운데 (ox, oy) 반지름 r 과 반직선 s·(dx,dy) 의 큰 쪽 교점
		ox, oy := math.Copysign(hw-r, dx), math.Copysign(hh-r, dy)
		qa := float64(dx*dx) + float64(dy*dy)
		qb := -2 * (float64(ox*dx) + float64(oy*dy))
		qc := float64(ox*ox) + float64(oy*oy) - float64(r*r)
		if disc := float64(qb*qb) - float64(4*qa*qc); disc >= 0 {
			t = (-qb + math.Sqrt(disc)) / (2 * qa)
		}
	}
	return layered.Pt{X: c.X + float64(dx*t), Y: c.Y + float64(dy*t)}
}

// hyp 는 √(x²+y²) 다 — 곱셈-덧셈을 명시적 float64 변환으로 나눠 FMA 합치기를 막는다(layered 의 hyp 와 같다).
func hyp(x, y float64) float64 { return math.Sqrt(float64(x*x) + float64(y*y)) }
