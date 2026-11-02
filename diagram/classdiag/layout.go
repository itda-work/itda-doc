package classdiag

import (
	"math"
	"sort"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 배치 상수(viewBox 단위, 글자 14 기준). 클래스 상자는 원본 v3 classBox 의 꼴 — 이름 칸(주석 «이름» 줄 + 굵은 이름)·
// 속성 칸·메서드 칸을 구분선 둘로 나눠 쌓는다. 빈 칸도 낮은 띠로 그린다(원본 renderExtraBox — hideEmptyMembersBox 기본 거짓).
const (
	fontSize  = diagram.FontSize
	lineH     = diagram.LineH
	nameWrap  = 260.0 // 클래스 이름·주석 줄바꿈 폭
	memWrap   = 320.0 // 멤버 줄바꿈 폭
	labelWrap = 140.0 // 관계 라벨 줄바꿈 폭(흐름도 화살표 글과 같다)
	padX      = 10.0  // 상자 안 글자 좌우 여백(한쪽)
	padY      = 7.0   // 이름 칸 위아래 여백(한쪽)
	secPad    = 5.0   // 멤버 칸 위아래 여백(한쪽)
	emptySec  = 12.0  // 빈 멤버 칸 높이
	minW      = 80.0  // 클래스 상자 최소 폭
	boldK     = 1.1   // 굵은 이름 글자 폭 어림 배수(TextW 는 보통 굵기)
	nodeSep   = 50.0  // 이웃 상자 사이
	rankSep   = 56.0  // 상자 계층 사이(라벨 계층의 반씩)
	loopOut   = 18.0  // 자기 관계 고리가 상자 끝보다 더 나가는 거리(끝 모양 자리)
	cardPad   = 3.0   // 수량 글 상자와 계층 띠 사이
	cardGap   = 6.0   // 같은 자리의 수량 글 상자 사이
	cardSide  = 5.0   // 수량 글 상자와 관계 선 사이
	cardBoxH  = lineH + 2
)

// box 는 클래스 하나의 모양이다(가운데는 배치가 정한다). 칸 높이는 위에서부터 이름·속성·메서드다.
type box struct {
	w, h         float64
	annot        []string // «주석» 줄(없으면 nil)
	name         []string
	attrs, meths [][]string // 멤버마다 줄바꿈한 줄
	nameH, attrH float64
	methH        float64
	attrM, methM []member
}

// card 는 양끝 수량 글 하나의 자리다(가운데 C, 상자 W·H — 바탕 없이 글자만 그리지만 겹침 판정은 이 상자로 한다).
type card struct {
	rel  int  // 관계 차례
	from bool // From 쪽(Card1)
	text string
	C    layered.Pt
	W, H float64
}

// laid 는 배치 결과다(최종 좌표).
type laid struct {
	W, H  float64
	boxes []box
	at    []layered.Pt // 상자 가운데
	edges []layered.EdgePath
	cards []card
}

func memberLines(ms []member) [][]string {
	out := make([][]string, len(ms))
	for i, m := range ms {
		out[i] = diagram.WrapLabel(m.Text, memWrap)
	}
	return out
}

// secH 는 멤버 칸 하나의 높이다 — 빈 칸은 emptySec.
func secH(ls [][]string) float64 {
	n := 0
	for _, l := range ls {
		n += len(l)
	}
	if n == 0 {
		return emptySec
	}
	return float64(float64(n)*lineH) + 2*secPad
}

func classBox(cl *classNode) box {
	b := box{name: diagram.WrapLabel(cl.Title(), nameWrap), attrM: cl.Attrs, methM: cl.Methods}
	w := float64(diagram.LinesW(b.name) * boldK)
	if cl.Annot != "" {
		b.annot = diagram.WrapLabel("«"+cl.Annot+"»", nameWrap)
		w = math.Max(w, diagram.LinesW(b.annot))
	}
	b.attrs, b.meths = memberLines(cl.Attrs), memberLines(cl.Methods)
	for _, ls := range [][][]string{b.attrs, b.meths} {
		for _, l := range ls {
			w = math.Max(w, diagram.LinesW(l))
		}
	}
	b.nameH = float64(float64(len(b.annot)+len(b.name))*lineH) + 2*padY
	b.attrH, b.methH = secH(b.attrs), secH(b.meths)
	b.w = math.Ceil(math.Max(w+2*padX, minW))
	b.h = math.Ceil(b.nameH + b.attrH + b.methH)
	return b
}

// cardSize 는 수량 글 상자의 크기다.
func cardSize(s string) (float64, float64) {
	return math.Ceil(diagram.TextW(s, fontSize) + 4), cardBoxH
}

func layout(c *chart) (*laid, error) {
	vert := c.Dir == TB || c.Dir == BT
	// 수량 글은 상자 계층 띠 바로 바깥(계층 사이 틈)에 둔다 — 틈(RankSep/2)이 한 틈의 양쪽 수량 글을 겹치지 않게 담을 만큼
	// 계층 사이를 넓힌다(수량 글이 없으면 기본 rankSep).
	ext := 0.0
	for _, r := range c.Rels {
		for _, s := range []string{r.Card1, r.Card2} {
			if s == "" {
				continue
			}
			w, h := cardSize(s)
			if vert {
				ext = math.Max(ext, h)
			} else {
				ext = math.Max(ext, w)
			}
		}
	}
	rs := rankSep
	if ext > 0 {
		rs = math.Max(rs, math.Ceil(float64(4*(ext+cardPad))+2))
	}
	in := &layered.Graph{Dir: layered.Dir(c.Dir), NodeSep: nodeSep, RankSep: rs, LoopOut: loopOut, // Dir 은 같은 차례(TB·BT·LR·RL)
		Nodes: make([]layered.Node, len(c.Classes)), Edges: make([]layered.Edge, len(c.Rels))}
	l := &laid{boxes: make([]box, len(c.Classes))}
	for i, cl := range c.Classes {
		l.boxes[i] = classBox(cl)
		in.Nodes[i] = layered.Node{W: l.boxes[i].w, H: l.boxes[i].h, Cluster: -1}
	}
	for i, r := range c.Rels {
		le := layered.Edge{From: r.From, To: r.To, Length: 1}
		if r.Label != "" {
			le.Label = diagram.WrapLabel(r.Label, labelWrap)
		}
		in.Edges[i] = le
	}
	res, err := layered.Layout(in)
	if err != nil {
		return nil, err
	}
	l.W, l.H = res.W, res.H
	l.edges = res.Edges
	l.at = make([]layered.Pt, len(res.Nodes))
	for i, n := range res.Nodes {
		l.at[i] = n.C
	}
	if ext > 0 {
		placeCards(c, l, res, vert)
	}
	return l, nil
}

// rankOf·crossOf 는 점의 계층 축·순서 축 좌표다(세로 흐름이면 y·x).
func rankOf(p layered.Pt, vert bool) float64 {
	if vert {
		return p.Y
	}
	return p.X
}

func crossOf(p layered.Pt, vert bool) float64 {
	if vert {
		return p.X
	}
	return p.Y
}

func mkPt(rank, cross float64, vert bool) layered.Pt {
	if vert {
		return layered.Pt{X: cross, Y: rank}
	}
	return layered.Pt{X: rank, Y: cross}
}

// bandHalves 는 수량 글이 붙는 끝 클래스의 계층 가운데마다 계층 띠의 반 길이를 한 번에 모아 돌려준다 — 같은 계층 가운데
// (±0.01 — 같은 계층의 점은 가운데가 같다, layered rankAxis)의 상자·라벨 반 높이와 그 계층 고리 점까지의 거리(+2) 가운데 큰
// 값이다. 가운데 C 개를 정렬해 두고 상자·간선·고리 점을 한 번씩 훑으며 이분 탐색으로 맞는 가운데를 찾는다 — 상자 V·간선 E·
// 고리 점 P 에 O(C log C + (V + E + P) log C). 예전처럼 수량 글마다 전체를 다시 훑지 않는다(값은 같다 — 같은 원소들의 최댓값).
func bandHalves(c *chart, l *laid, res *layered.Result, vert bool) func(float64) float64 {
	var centers []float64
	for _, r := range c.Rels {
		if r.Card1 != "" {
			centers = append(centers, rankOf(l.at[r.From], vert))
		}
		if r.Card2 != "" {
			centers = append(centers, rankOf(l.at[r.To], vert))
		}
	}
	sort.Float64s(centers)
	uniq := centers[:0]
	for k, v := range centers {
		if k == 0 || v != uniq[len(uniq)-1] {
			uniq = append(uniq, v)
		}
	}
	centers = uniq
	hs := make([]float64, len(centers))
	// each 는 가운데가 v 와 같다고 보는(±0.01) 가운데마다 f(차례)를 부른다.
	each := func(v float64, f func(k int)) {
		k := sort.SearchFloat64s(centers, v-0.01)
		for ; k < len(centers) && centers[k]-v < 0.01; k++ {
			if math.Abs(v-centers[k]) < 0.01 {
				f(k)
			}
		}
	}
	for _, n := range res.Nodes {
		hv := n.W / 2
		if vert {
			hv = n.H / 2
		}
		each(rankOf(n.C, vert), func(k int) { hs[k] = math.Max(hs[k], hv) })
	}
	for _, e := range res.Edges {
		if len(e.Label) > 0 {
			hv := e.LW / 2
			if vert {
				hv = e.LH / 2
			}
			each(rankOf(e.LC, vert), func(k int) { hs[k] = math.Max(hs[k], hv) })
		}
		if e.Self && len(e.Pts) > 0 {
			// 고리는 상자 계층에 있다 — 가운데(LC)가 상자 계층의 가운데다
			each(rankOf(e.LC, vert), func(k int) {
				for _, p := range e.Pts {
					hs[k] = math.Max(hs[k], math.Abs(rankOf(p, vert)-centers[k])+2)
				}
			})
		}
	}
	return func(center float64) float64 {
		k := sort.SearchFloat64s(centers, center)
		return hs[k] // center 는 위에서 모은 가운데 가운데 하나다
	}
}

// placeCards 는 양끝 수량 글의 자리를 정한다. 수량 글은 끝 클래스가 놓인 계층 띠(같은 계층의 상자·라벨·고리를 모두 덮는
// 띠)의 바깥, 관계가 나가는 쪽 틈에 둔다 — 틈 안이라 상자·라벨과 겹치지 않는다. 계층 축 자리는 띠 끝 + cardPad 부터 글
// 상자 길이만큼이고(같은 띠·같은 쪽이면 같은 구간), 순서 축 자리는 그 구간에서 관계 선의 옆(cardSide)이다. 같은 띠·같은
// 쪽의 수량 글은 순서 축에서 차례로 밀어 겹치지 않게 한다(1차원 채우기 — cardGap). 틈 하나의 양쪽 구간은 layout 이
// 계층 사이를 넓혀(4×(길이 + cardPad)) 겹치지 않는다. 마지막에 그림 크기를 수량 글까지 넓힌다.
//
// 계산량: 수량 글 K 개, 상자 V·간선 E·고리 점 P, 수량 글이 붙은 계층 가운데 C(≤ K)에 띠 반 길이 모으기 O(C log C + (V + E + P) log C)
// (bandHalves), 수량 글마다 띠 찾기 O(log C)·관계 선 자리 O(1), 자리 정렬 O(K log K), 채우기·그림 넓히기 O(K + V + E·점 수).
// 선형이 아니다 — 정렬·이분 탐색의 log 가 붙는다(배치 layered 자체의 계산량은 그와 별개다).
func placeCards(c *chart, l *laid, res *layered.Result, vert bool) {
	half := bandHalves(c, l, res, vert)
	type slot struct {
		key   [2]float64 // 띠 가운데·쪽
		want  float64    // 바라는 순서 축 가운데
		r0    float64    // 계층 축 구간 시작
		ci    int        // l.cards 차례
		order int        // 같은 바람일 때의 차례(관계 차례·From 먼저)
	}
	var slots []slot
	for i, r := range c.Rels {
		pts := l.edges[i].Pts
		if len(pts) < 2 {
			continue
		}
		for _, end := range []struct {
			s    string
			from bool
		}{{r.Card1, true}, {r.Card2, false}} {
			if end.s == "" {
				continue
			}
			node, p, q := r.From, pts[0], pts[1]
			if !end.from {
				node, p, q = r.To, pts[len(pts)-1], pts[len(pts)-2]
			}
			center := rankOf(l.at[node], vert)
			sign := 1.0
			if rankOf(q, vert) < center {
				sign = -1
			}
			edge := center + sign*half(center)
			w, h := cardSize(end.s)
			ext, crossHalf := h, w/2
			if !vert {
				ext, crossHalf = w, h/2
			}
			r0 := edge + sign*cardPad
			r1 := r0 + sign*ext
			// 구간 양 끝에서 관계 선의 순서 축 자리(p→q 직선) — 큰 쪽 옆에 둔다
			at := func(rv float64) float64 {
				pr, qr := rankOf(p, vert), rankOf(q, vert)
				pc, qc := crossOf(p, vert), crossOf(q, vert)
				if qr == pr {
					return pc
				}
				t := (rv - pr) / (qr - pr)
				return pc + float64((qc-pc)*t)
			}
			lc := math.Max(at(r0), at(r1))
			ci := len(l.cards)
			l.cards = append(l.cards, card{rel: i, from: end.from, text: end.s, W: w, H: h})
			slots = append(slots, slot{key: [2]float64{center, sign}, want: lc + cardSide + crossHalf, r0: math.Min(r0, r1), ci: ci, order: ci})
		}
	}
	sort.SliceStable(slots, func(a, b int) bool {
		x, y := slots[a], slots[b]
		if x.key != y.key {
			if x.key[0] != y.key[0] {
				return x.key[0] < y.key[0]
			}
			return x.key[1] < y.key[1]
		}
		if x.want != y.want {
			return x.want < y.want
		}
		return x.order < y.order
	})
	prevEnd := math.Inf(-1)
	for k, s := range slots {
		if k == 0 || slots[k-1].key != s.key {
			prevEnd = math.Inf(-1)
		}
		cd := &l.cards[s.ci]
		ext, crossHalf := cd.H, cd.W/2
		if !vert {
			ext, crossHalf = cd.W, cd.H/2
		}
		cc := math.Max(s.want, prevEnd+cardGap+crossHalf)
		prevEnd = cc + crossHalf
		cd.C = mkPt(s.r0+ext/2, cc, vert)
	}
	// 그림 크기를 수량 글까지 넓힌다(왼쪽·위로 넘으면 전체를 민다).
	const margin = 8.0
	minX, minY, maxX, maxY := margin, margin, l.W-margin, l.H-margin
	for _, cd := range l.cards {
		minX, minY = math.Min(minX, cd.C.X-cd.W/2), math.Min(minY, cd.C.Y-cd.H/2)
		maxX, maxY = math.Max(maxX, cd.C.X+cd.W/2), math.Max(maxY, cd.C.Y+cd.H/2)
	}
	dx, dy := margin-minX, margin-minY
	if dx != 0 || dy != 0 {
		mv := func(p *layered.Pt) { p.X += dx; p.Y += dy }
		for i := range l.at {
			mv(&l.at[i])
		}
		for i := range l.edges {
			e := &l.edges[i]
			for k := range e.Pts {
				mv(&e.Pts[k])
			}
			mv(&e.LC)
		}
		for i := range l.cards {
			mv(&l.cards[i].C)
		}
	}
	l.W = math.Ceil(maxX - minX + 2*margin)
	l.H = math.Ceil(maxY - minY + 2*margin)
}
