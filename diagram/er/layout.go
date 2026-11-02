package er

import (
	"math"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 배치 상수(viewBox 단위, 글자 14 기준). 엔티티 상자는 원본 erBox 의 꼴 — 머리 칸 + 속성 표(열마다 가장 넓은 글 + 여백,
// 머리 이름이 열 합보다 넓으면 차이를 있는 열에 고루 나눈다). 속성 없는 엔티티는 머리만(원본 minEntityWidth 100).
const (
	fontSize = diagram.FontSize
	lineH    = diagram.LineH
	nameWrap = 220.0 // 머리 이름 줄바꿈 폭
	cellWrap = 180.0 // 타입·이름 칸 줄바꿈 폭
	cmtWrap  = 200.0 // 설명 칸 줄바꿈 폭
	roleWrap = 140.0 // 역할 글 줄바꿈 폭(흐름도 화살표 글과 같다)
	cellPadX = 8.0   // 칸 안 글자 좌우 여백(한쪽)
	rowPadY  = 3.0   // 속성 줄 위아래 여백(한쪽)
	headPadY = 6.0   // 머리 칸 위아래 여백(한쪽)
	namePadX = 12.0  // 머리 이름 좌우 여백(한쪽)
	bareMinW = 100.0 // 속성 없는 상자의 최소 폭(원본 minEntityWidth)
	barePadY = 14.0  // 속성 없는 상자의 위아래 여백(한쪽)
	nodeSep  = 56.0  // 이웃 상자 사이
	rankSep  = 72.0  // 상자 계층 사이(역할 글 계층의 반씩) — 양끝 기호(최대 markLen)가 들어가게
	markLen  = 25.0  // 끝 기호가 상자 테두리에서 선을 따라 차지하는 길이(동그라미 끝까지)
	loopOut  = 22.0  // 자기 관계 고리가 상자 끝보다 더 나가는 거리 — 고리가 상자 밖으로 나가 끝 기호가 선 위에 놓이게
	margin   = 8.0
)

// box 는 엔티티 상자 하나의 모양이다(가운데는 배치가 정한다).
type box struct {
	w, h   float64
	name   []string // 머리 이름 줄
	headH  float64
	cols   []float64 // 열 너비 — 타입·이름·(키)·(설명)
	hasKey bool
	hasCmt bool
	rows   []row
}

type row struct {
	h     float64
	cells [][]string // 열마다 줄(cols 와 같은 차례)
}

// laid 는 배치 결과다(최종 좌표).
type laid struct {
	W, H  float64
	boxes []box
	at    []layered.Pt // 상자 가운데
	edges []layered.EdgePath
	ends  [][2]end // 관계마다 A 쪽·B 쪽 끝 기호 자리
}

// end 는 끝 기호 하나의 자리다 — tip 은 선이 상자 테두리에 닿는 점, (ux,uy) 는 tip 에서 선을 따라 상자 밖으로 가는 단위 방향.
type end struct {
	tip    layered.Pt
	ux, uy float64
	card   Card
}

func entityBox(e *entity) box {
	label := e.Name
	if e.Alias != "" {
		label = e.Alias
	}
	b := box{name: diagram.WrapLabel(label, nameWrap)}
	nameW := diagram.LinesW(b.name)
	if len(e.Attrs) == 0 {
		b.w = math.Ceil(math.Max(nameW+2*namePadX, bareMinW))
		b.h = math.Ceil(float64(float64(len(b.name))*lineH) + 2*barePadY)
		b.headH = b.h
		return b
	}
	b.headH = float64(float64(len(b.name))*lineH) + 2*headPadY
	for _, a := range e.Attrs {
		b.hasKey = b.hasKey || len(a.Keys) > 0
		b.hasCmt = b.hasCmt || a.Comment != ""
	}
	ncol := 2
	if b.hasKey {
		ncol++
	}
	if b.hasCmt {
		ncol++
	}
	b.cols = make([]float64, ncol)
	for _, a := range e.Attrs {
		r := row{cells: [][]string{diagram.WrapLabel(a.Type, cellWrap), diagram.WrapLabel(a.Name, cellWrap)}}
		if b.hasKey {
			r.cells = append(r.cells, []string{keysText(a.Keys)})
		}
		if b.hasCmt {
			r.cells = append(r.cells, diagram.WrapLabel(a.Comment, cmtWrap))
		}
		n := 1
		for k, c := range r.cells {
			n = max(n, len(c))
			b.cols[k] = math.Max(b.cols[k], diagram.LinesW(c)+2*cellPadX)
		}
		r.h = float64(float64(n)*lineH) + 2*rowPadY
		b.rows = append(b.rows, r)
	}
	sum := 0.0
	for _, c := range b.cols {
		sum += c
	}
	if d := nameW + 2*namePadX - sum; d > 0 { // 머리 이름이 더 넓으면 차이를 열에 고루(원본 erBox 와 같다)
		for k := range b.cols {
			b.cols[k] += d / float64(ncol)
		}
		sum = nameW + 2*namePadX
	}
	for k := range b.cols {
		b.cols[k] = math.Ceil(b.cols[k])
	}
	b.w = 0
	for _, c := range b.cols {
		b.w += c
	}
	b.h = b.headH
	for _, r := range b.rows {
		b.h += r.h
	}
	b.h = math.Ceil(b.h)
	return b
}

// keysText 는 키 칸의 글이다(원본 attribute.keys.join() — 쉼표로 붙인다).
func keysText(keys []string) string {
	s := ""
	for i, k := range keys {
		if i > 0 {
			s += ","
		}
		s += k
	}
	return s
}

func layout(c *chart) (*laid, error) {
	in := &layered.Graph{Dir: layered.Dir(c.Dir), NodeSep: nodeSep, RankSep: rankSep, LoopOut: loopOut, // Dir 은 같은 차례(TB·BT·LR·RL)
		Nodes: make([]layered.Node, len(c.Entities)), Edges: make([]layered.Edge, len(c.Relations))}
	l := &laid{boxes: make([]box, len(c.Entities))}
	for i, e := range c.Entities {
		l.boxes[i] = entityBox(e)
		in.Nodes[i] = layered.Node{W: l.boxes[i].w, H: l.boxes[i].h, Cluster: -1}
	}
	for i, r := range c.Relations {
		le := layered.Edge{From: r.A, To: r.B, Length: 1}
		if r.Role != "" {
			le.Label = diagram.WrapLabel(r.Role, roleWrap)
		}
		in.Edges[i] = le
	}
	res, err := layered.Layout(in)
	if err != nil {
		return nil, err
	}
	l.edges = res.Edges
	l.at = make([]layered.Pt, len(res.Nodes))
	for i, n := range res.Nodes {
		l.at[i] = n.C
	}
	l.ends = make([][2]end, len(c.Relations))
	for i, r := range c.Relations {
		p := res.Edges[i].Pts
		rev := make([]layered.Pt, len(p))
		for k := range p {
			rev[k] = p[len(p)-1-k]
		}
		l.ends[i] = [2]end{endAt(p, r.Left), endAt(rev, r.Right)}
	}
	l.fit(res.W, res.H)
	return l, nil
}

// endAt 은 점 줄 p 의 첫 점(상자 테두리)에 놓는 끝 기호다 — 방향은 그다음의 떨어진 점 쪽이다(겹친 점은 건너뛴다).
func endAt(p []layered.Pt, c Card) end {
	tip := p[0]
	dx, dy, d := 0.0, 1.0, 1.0
	for _, q := range p[1:] {
		ex, ey := q.X-tip.X, q.Y-tip.Y
		if e := math.Sqrt(float64(ex*ex) + float64(ey*ey)); e > 0.5 {
			dx, dy, d = ex, ey, e
			break
		}
	}
	return end{tip: tip, ux: dx / d, uy: dy / d, card: c}
}

// fit 은 끝 기호(선을 따라 markLen, 옆으로 markHalf)까지 viewBox 안에 들게 전체를 옮기고 크기를 정한다 — 배치는 점마다
// 5 만큼만 여유를 두기 때문이다.
func (l *laid) fit(w, h float64) {
	minX, minY, maxX, maxY := 0.0, 0.0, w, h
	for _, es := range l.ends {
		for _, e := range es {
			for _, p := range markExtent(e) {
				minX, minY = math.Min(minX, p.X-margin), math.Min(minY, p.Y-margin)
				maxX, maxY = math.Max(maxX, p.X+margin), math.Max(maxY, p.Y+margin)
			}
		}
	}
	dx, dy := -minX, -minY
	if dx != 0 || dy != 0 {
		for i := range l.at {
			l.at[i].X += dx
			l.at[i].Y += dy
		}
		for i := range l.edges {
			e := &l.edges[i]
			for k := range e.Pts {
				e.Pts[k].X += dx
				e.Pts[k].Y += dy
			}
			e.LC.X += dx
			e.LC.Y += dy
		}
		for i := range l.ends {
			for k := range l.ends[i] {
				l.ends[i][k].tip.X += dx
				l.ends[i][k].tip.Y += dy
			}
		}
	}
	l.W, l.H = math.Ceil(maxX-minX), math.Ceil(maxY-minY)
}
