package layered

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/internal/timingtest"
)

// 공통 배치 자체 시험(itda-hyve#409 ④-1 후속) — 흐름도·ER 을 거치지 않고 Layout 을 직접 부른다. 정답은 구현 상수·도우미가 아니라
// 성질이다: 그림 안, 겹침 없음, 간선 끝이 테두리 위, 방향대로 나아감, 묶음이 구성원을 감쌈, 결정성, 상한, 시간.

var dirs = []Dir{TB, BT, LR, RL}

func (d Dir) String() string { return [...]string{"TB", "BT", "LR", "RL"}[d] }

// rect 는 [x0,x1]×[y0,y1] 이다.
type rect struct{ x0, y0, x1, y1 float64 }

func boxRect(b Box) rect { return rect{b.C.X - b.W/2, b.C.Y - b.H/2, b.C.X + b.W/2, b.C.Y + b.H/2} }

func labelRect(e EdgePath) rect {
	return rect{e.LC.X - e.LW/2, e.LC.Y - e.LH/2, e.LC.X + e.LW/2, e.LC.Y + e.LH/2}
}

// overlaps 는 두 상자가 tol 넘게 파고드는지 본다(맞닿음은 겹침이 아니다).
func (a rect) overlaps(b rect, tol float64) bool {
	return a.x0 < b.x1-tol && b.x0 < a.x1-tol && a.y0 < b.y1-tol && b.y0 < a.y1-tol
}

// contains 는 b 가 a 안에 있는지(tol 만큼 너그럽게) 본다.
func (a rect) contains(b rect, tol float64) bool {
	return b.x0 >= a.x0-tol && b.y0 >= a.y0-tol && b.x1 <= a.x1+tol && b.y1 <= a.y1+tol
}

// rectDist 는 점 p 와 사각형 테두리 사이의 거리다(안이든 밖이든).
func rectDist(r rect, p Pt) float64 {
	if p.X >= r.x0 && p.X <= r.x1 && p.Y >= r.y0 && p.Y <= r.y1 {
		return math.Min(math.Min(p.X-r.x0, r.x1-p.X), math.Min(p.Y-r.y0, r.y1-p.Y))
	}
	dx := math.Max(math.Max(r.x0-p.X, 0), p.X-r.x1)
	dy := math.Max(math.Max(r.y0-p.Y, 0), p.Y-r.y1)
	return math.Hypot(dx, dy)
}

// word 는 라벨 낱말 하나다(한글·영문 섞어서, 폭이 다양하게).
func word(rng *rand.Rand, n int) string {
	const pool = "가나다라마바사아자차카타파하abcdefghijklmnopqrstuvwxyzWM0123456789"
	r := []rune(pool)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(r[rng.Intn(len(r))])
	}
	return b.String()
}

func label(rng *rand.Rand, maxLines, maxLen int) []string {
	out := make([]string, 1+rng.Intn(maxLines))
	for i := range out {
		out[i] = word(rng, 1+rng.Intn(maxLen))
	}
	return out
}

// genOpt 는 무작위 그래프의 모양이다.
type genOpt struct {
	nodes, edges, clusters int
	dag                    bool // 간선을 앞 노드 → 뒤 노드로만(순환 없음)
	local                  int  // 0 이 아니면 간선의 끝을 시작 노드 뒤 local 개 안에서 고른다(긴 간선을 줄여 점 상한 밑에)
}

func randGraph(rng *rand.Rand, o genOpt, dir Dir) *Graph {
	g := &Graph{Dir: dir, NodeSep: 10 + float64(rng.Intn(50)), RankSep: 10 + float64(rng.Intn(50)), LoopOut: float64(rng.Intn(3) * 6)}
	for c := 0; c < o.clusters; c++ {
		t := ""
		if rng.Intn(3) > 0 {
			t = strings.Join(label(rng, 3, 30), " ")
		}
		g.Clusters = append(g.Clusters, Cluster{Title: t})
	}
	for i := 0; i < o.nodes; i++ {
		n := Node{W: 8 + float64(rng.Intn(2400))/10, H: 8 + float64(rng.Intn(1000))/10, Cluster: -1}
		if rng.Intn(25) == 0 { // 큰 상자
			n.W, n.H = 400+float64(rng.Intn(400)), 200+float64(rng.Intn(200))
		}
		if o.clusters > 0 && rng.Intn(5) < 2 {
			n.Cluster = rng.Intn(o.clusters)
		}
		g.Nodes = append(g.Nodes, n)
	}
	for k := 0; k < o.edges && o.nodes > 0; k++ {
		var e Edge
		switch {
		case k > 0 && rng.Intn(10) == 0: // 같은 쌍 여러 간선
			e = g.Edges[rng.Intn(len(g.Edges))]
		case rng.Intn(20) == 0: // 자기 간선
			v := rng.Intn(o.nodes)
			e = Edge{From: v, To: v}
		default:
			a, b := rng.Intn(o.nodes), rng.Intn(o.nodes)
			if o.local > 0 {
				b = min(o.nodes-1, a+rng.Intn(o.local+1))
				if rng.Intn(2) == 0 && !o.dag {
					a, b = b, a
				}
			}
			if o.dag && a > b {
				a, b = b, a
			}
			e = Edge{From: a, To: b}
		}
		e.Length = 1 + rng.Intn(3)
		if rng.Intn(4) == 0 {
			e.Length = 1
		}
		e.Label, e.Hidden = nil, false
		if rng.Intn(5) < 2 {
			e.Label = label(rng, 3, 24)
		}
		if rng.Intn(10) == 0 {
			e.Hidden = true
		}
		g.Edges = append(g.Edges, e)
	}
	return g
}

// copyGraph 는 입력을 깊이 복사한다(Layout 이 입력을 바꾸지 않는지도 함께 본다).
func copyGraph(g *Graph) *Graph {
	c := *g
	c.Nodes = append([]Node(nil), g.Nodes...)
	c.Clusters = append([]Cluster(nil), g.Clusters...)
	c.Edges = make([]Edge, len(g.Edges))
	for i, e := range g.Edges {
		e.Label = append([]string(nil), e.Label...)
		if g.Edges[i].Label == nil {
			e.Label = nil
		}
		c.Edges[i] = e
	}
	return &c
}

// sccs 는 자기 간선을 뺀 그래프의 강한 연결 요소 번호다(타잔 — 구현의 순환 끊기와 따로 계산한다).
func sccs(g *Graph) []int {
	n := len(g.Nodes)
	out := make([][]int, n)
	for _, e := range g.Edges {
		if e.From != e.To {
			out[e.From] = append(out[e.From], e.To)
		}
	}
	idx, low, comp := make([]int, n), make([]int, n), make([]int, n)
	on := make([]bool, n)
	for i := range idx {
		idx[i], comp[i] = -1, -1
	}
	var stack []int
	next, nc := 0, 0
	type frame struct{ v, k int }
	for s := 0; s < n; s++ {
		if idx[s] >= 0 {
			continue
		}
		call := []frame{{s, 0}}
		idx[s], low[s] = next, next
		next++
		stack = append(stack, s)
		on[s] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			if f.k < len(out[f.v]) {
				w := out[f.v][f.k]
				f.k++
				if idx[w] < 0 {
					idx[w], low[w] = next, next
					next++
					stack = append(stack, w)
					on[w] = true
					call = append(call, frame{w, 0})
				} else if on[w] {
					low[f.v] = min(low[f.v], idx[w])
				}
				continue
			}
			v := f.v
			call = call[:len(call)-1]
			if len(call) > 0 {
				p := call[len(call)-1].v
				low[p] = min(low[p], low[v])
			}
			if low[v] == idx[v] {
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					on[w] = false
					comp[w] = nc
					if w == v {
						break
					}
				}
				nc++
			}
		}
	}
	return comp
}

// along 은 방향 d 의 계층 축에서 상자가 차지하는 구간이다 — 나아가는 쪽이 커지게(BT·RL 은 부호를 뒤집는다).
func along(d Dir, b Box) (near, far float64) {
	switch d {
	case TB:
		return b.C.Y - b.H/2, b.C.Y + b.H/2
	case BT:
		return -(b.C.Y + b.H/2), -(b.C.Y - b.H/2)
	case LR:
		return b.C.X - b.W/2, b.C.X + b.W/2
	}
	return -(b.C.X + b.W/2), -(b.C.X - b.W/2)
}

// checkProps 는 배치 결과 r 이 입력 g 에 대해 지켜야 할 성질을 본다. onBorder 는 간선 끝이 상자 테두리에서 떨어져도 되는
// 거리다(Clip 이 사각 자르기일 때).
func checkProps(t *testing.T, name string, g *Graph, r *Result) {
	t.Helper()
	const tol, onBorder = 0.5, 0.05
	fail := func(f string, a ...any) { t.Errorf("%s: %s", name, fmt.Sprintf(f, a...)) }
	if len(r.Nodes) != len(g.Nodes) || len(r.Edges) != len(g.Edges) || len(r.Clusters) != len(g.Clusters) {
		fail("결과 개수 노드 %d·간선 %d·묶음 %d, 입력 %d·%d·%d", len(r.Nodes), len(r.Edges), len(r.Clusters), len(g.Nodes), len(g.Edges), len(g.Clusters))
		return
	}
	all := rect{0, 0, r.W, r.H}
	for i, b := range r.Nodes {
		if b.W != g.Nodes[i].W || b.H != g.Nodes[i].H {
			fail("노드 %d 크기 %v×%v, 입력 %v×%v", i, b.W, b.H, g.Nodes[i].W, g.Nodes[i].H)
		}
		if !all.contains(boxRect(b), 0.01) {
			fail("노드 %d 가 그림(%v×%v) 밖 %+v", i, r.W, r.H, boxRect(b))
		}
		for j := i + 1; j < len(r.Nodes); j++ {
			if boxRect(b).overlaps(boxRect(r.Nodes[j]), tol) {
				fail("노드 %d·%d 겹침", i, j)
			}
		}
	}
	var labels []rect
	var lk []int
	for k, e := range r.Edges {
		in := g.Edges[k]
		if in.From == in.To && in.Hidden {
			if len(e.Pts) != 0 {
				fail("투명 자기 간선 %d 에 점이 있음", k)
			}
			continue
		}
		if in.From == in.To != e.Self {
			fail("간선 %d 의 Self %v", k, e.Self)
		}
		if e.Self && len(e.Pts) != 7 {
			fail("자기 간선 %d 의 점 %d 개", k, len(e.Pts))
		}
		if len(e.Pts) < 2 {
			fail("간선 %d 의 점 %d 개", k, len(e.Pts))
			continue
		}
		for _, p := range e.Pts {
			if !all.contains(rect{p.X, p.Y, p.X, p.Y}, 0.01) {
				fail("간선 %d 의 점 %v 가 그림 밖", k, p)
			}
		}
		// 첫 점은 From, 끝 점은 To 의 테두리 위(원래 방향 — 순환을 끊으려 뒤집은 간선도)
		for _, end := range []struct {
			p Pt
			v int
		}{{e.Pts[0], in.From}, {e.Pts[len(e.Pts)-1], in.To}} {
			if d := rectDist(boxRect(r.Nodes[end.v]), end.p); d > onBorder {
				fail("간선 %d 의 끝 %v 가 노드 %d 테두리에서 %.3f", k, end.p, end.v, d)
			}
		}
		if len(in.Label) > 0 {
			if !reflect.DeepEqual(e.Label, in.Label) {
				fail("간선 %d 라벨 %q, 입력 %q", k, e.Label, in.Label)
			}
			w, h := LabelSize(in.Label)
			if e.LW != w || e.LH != h {
				fail("간선 %d 라벨 상자 %v×%v, LabelSize %v×%v", k, e.LW, e.LH, w, h)
			}
			lr := labelRect(e)
			if !all.contains(lr, 0.01) {
				fail("간선 %d 라벨이 그림 밖", k)
			}
			labels, lk = append(labels, lr), append(lk, k)
		}
	}
	for a := range labels {
		for b := a + 1; b < len(labels); b++ {
			if labels[a].overlaps(labels[b], tol) {
				fail("간선 %d·%d 라벨 겹침", lk[a], lk[b])
			}
		}
		for i, n := range r.Nodes {
			if labels[a].overlaps(boxRect(n), tol) {
				fail("간선 %d 라벨과 노드 %d 겹침", lk[a], i)
			}
		}
	}
	// 방향: 같은 강한 연결 요소가 아닌 두 노드를 잇는 간선은 순환 끊기가 뒤집지 않는다 — From 의 먼 끝에서 To 의 가까운 끝까지
	// 계층 축으로 적어도 Length×RankSep(계층 2×Length 칸, 칸마다 RankSep/2 이상)
	comp := sccs(g)
	for k, e := range g.Edges {
		if e.From == e.To || comp[e.From] == comp[e.To] {
			continue
		}
		_, far := along(g.Dir, r.Nodes[e.From])
		near, _ := along(g.Dir, r.Nodes[e.To])
		if gap := near - far; gap < float64(e.Length)*g.RankSep-0.01 {
			fail("간선 %d(%d→%d, 길이 %d)가 %s 방향으로 %.1f 만 나아감(기대 %.1f 이상)", k, e.From, e.To, e.Length, g.Dir, gap, float64(e.Length)*g.RankSep)
		}
	}
	// 묶음: 구성원이 있으면 보이고 구성원을 감싼다. 묶음끼리·밖의 노드와 겹치지 않는다. 구성원이 없으면 보이지 않는다.
	members := make([]int, len(g.Clusters))
	for _, n := range g.Nodes {
		if n.Cluster >= 0 {
			members[n.Cluster]++
		}
	}
	for c, cb := range r.Clusters {
		if cb.Show != (members[c] > 0) {
			fail("묶음 %d(구성원 %d)의 Show %v", c, members[c], cb.Show)
			continue
		}
		if !cb.Show {
			continue
		}
		box := rect{cb.X, cb.Y, cb.X + cb.W, cb.Y + cb.H}
		if !all.contains(box, 0.01) {
			fail("묶음 %d 상자가 그림 밖", c)
		}
		for d := c + 1; d < len(r.Clusters); d++ {
			if o := r.Clusters[d]; o.Show && box.overlaps(rect{o.X, o.Y, o.X + o.W, o.Y + o.H}, tol) {
				fail("묶음 %d·%d 상자 겹침", c, d)
			}
		}
		for i, n := range r.Nodes {
			in := g.Nodes[i].Cluster == c
			if in && !box.contains(boxRect(n), tol) {
				fail("묶음 %d 상자가 구성원 노드 %d 를 담지 않음", c, i)
			}
			if !in && box.overlaps(boxRect(n), tol) {
				fail("묶음 %d 상자가 밖의 노드 %d 와 겹침", c, i)
			}
		}
	}
}

// layoutOK 는 배치하고 오류 꼴을 본다 — 오류는 *diagram.Error 뿐이다(패닉 없음).
func layoutOK(t *testing.T, name string, g *Graph) (*Result, bool) {
	t.Helper()
	r, err := layoutRaw(t, name, g)
	if err != nil {
		var de *diagram.Error
		if !errors.As(err, &de) {
			t.Fatalf("%s: *diagram.Error 가 아닌 오류 %T %v", name, err, err)
		}
		return nil, false
	}
	return r, true
}

// layoutRaw 는 패닉을 실패로 바꿔 배치한다.
func layoutRaw(t *testing.T, name string, g *Graph) (r *Result, err error) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("%s: 패닉 %v", name, p)
		}
	}()
	return Layout(g)
}

// mustLayout 은 점 상한 밑 입력을 배치한다 — 어떤 오류든(점 상한 오류 꼴이어도) 실패다. 상한을 넘을 수 있는 입력은 시험 안에서
// 점 수 하한을 따로 세어 TestMaxPoints 처럼 layoutOK 로 본다(오류 타입·문구로 건너뛰지 않는다 — 후속 리뷰 1회차 [1]).
func mustLayout(t *testing.T, name string, g *Graph) *Result {
	t.Helper()
	r, err := layoutRaw(t, name, g)
	if err != nil {
		t.Fatalf("%s: 점 상한 밑 입력인데 오류 %v", name, err)
	}
	return r
}

// 무작위 그래프 — 크기·밀도·묶음·방향을 섞는다. 같은 입력 두 번은 같은 결과, 입력은 바뀌지 않는다, 노드 차례를 섞은 입력도
// 오류 없이 같은 성질을 지킨다. 생성기 입력은 모두 점 상한 밑이라(큰 그림은 간선 끝을 가까이 — local) 어떤 오류든 실패다.
func TestRandomProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(409))
	const rounds = 240
	for it := 0; it < rounds; it++ {
		var o genOpt
		switch it % 6 {
		case 0, 1, 2: // 작은 그림
			o = genOpt{nodes: 1 + rng.Intn(25), edges: rng.Intn(45), clusters: rng.Intn(4)}
		case 3: // 중간, 묶음 많이
			o = genOpt{nodes: 20 + rng.Intn(60), edges: rng.Intn(120), clusters: rng.Intn(11), local: 6}
		case 4: // 순환 없음
			o = genOpt{nodes: 2 + rng.Intn(60), edges: rng.Intn(100), clusters: rng.Intn(5), dag: true, local: 8}
		default: // 큰 그림 — 간선을 가까이 이어 점 상한 밑에
			o = genOpt{nodes: 100 + rng.Intn(101), edges: 200 + rng.Intn(201), clusters: rng.Intn(11), local: 3}
		}
		dir := dirs[it%4]
		g := randGraph(rng, o, dir)
		name := fmt.Sprintf("판 %d(%s, 노드 %d·간선 %d·묶음 %d)", it, dir, len(g.Nodes), len(g.Edges), len(g.Clusters))
		before := copyGraph(g)
		r := mustLayout(t, name, g)
		if !reflect.DeepEqual(copyGraph(g), before) {
			t.Fatalf("%s: Layout 이 입력을 바꿈", name)
		}
		checkProps(t, name, g, r)
		r2 := mustLayout(t, name, copyGraph(before))
		if !reflect.DeepEqual(r, r2) {
			t.Errorf("%s: 같은 입력 두 번이 다름", name)
		}
		// 노드 차례를 섞는다(간선·묶음 소속을 따라 옮긴다)
		perm := rng.Perm(len(g.Nodes))
		pg := copyGraph(g)
		for i, n := range g.Nodes {
			pg.Nodes[perm[i]] = n
		}
		for k := range pg.Edges {
			pg.Edges[k].From, pg.Edges[k].To = perm[g.Edges[k].From], perm[g.Edges[k].To]
		}
		checkProps(t, name+" 섞음", pg, mustLayout(t, name+" 섞음", pg))
		if t.Failed() {
			return
		}
	}
}

// 극단 — 노드 하나, 간선 없음, 작은 완전 그래프, 긴 사슬, 넓은 별, 큰 라벨, 자기 간선만, 같은 쌍 여러 간선.
func TestExtremes(t *testing.T) {
	type mk func(d Dir) *Graph
	nodes := func(n int, w, h float64) []Node {
		out := make([]Node, n)
		for i := range out {
			out[i] = Node{W: w, H: h, Cluster: -1}
		}
		return out
	}
	base := func(d Dir, ns []Node) *Graph { return &Graph{Dir: d, Nodes: ns, NodeSep: 30, RankSep: 40} }
	cases := map[string]mk{
		"노드 하나": func(d Dir) *Graph { return base(d, nodes(1, 40, 20)) },
		"간선 없음": func(d Dir) *Graph { return base(d, nodes(30, 50, 30)) },
		"완전 그래프 6": func(d Dir) *Graph {
			g := base(d, nodes(6, 60, 30))
			for a := 0; a < 6; a++ {
				for b := 0; b < 6; b++ {
					if a != b {
						g.Edges = append(g.Edges, Edge{From: a, To: b, Length: 1, Label: []string{fmt.Sprintf("%d→%d", a, b)}})
					}
				}
			}
			return g
		},
		"긴 사슬 200": func(d Dir) *Graph {
			g := base(d, nodes(200, 40, 20))
			for i := 0; i+1 < 200; i++ {
				g.Edges = append(g.Edges, Edge{From: i, To: i + 1, Length: 1})
			}
			return g
		},
		"넓은 별 200": func(d Dir) *Graph {
			g := base(d, nodes(200, 40, 20))
			for i := 1; i < 200; i++ {
				g.Edges = append(g.Edges, Edge{From: 0, To: i, Length: 1 + i%3, Label: []string{"x"}})
			}
			return g
		},
		"큰 라벨": func(d Dir) *Graph {
			g := base(d, nodes(3, 900, 400))
			big := []string{strings.Repeat("가", 200), strings.Repeat("W", 200), strings.Repeat("나", 50)}
			g.Edges = []Edge{{From: 0, To: 1, Length: 1, Label: big}, {From: 0, To: 1, Length: 1, Label: big}, {From: 1, To: 1, Length: 1, Label: big},
				{From: 2, To: 0, Length: 3, Label: big}}
			return g
		},
		"자기 간선만": func(d Dir) *Graph {
			g := base(d, nodes(2, 40, 20))
			g.LoopOut = 12
			for k := 0; k < 6; k++ {
				g.Edges = append(g.Edges, Edge{From: k % 2, To: k % 2, Length: 1, Label: []string{word(rand.New(rand.NewSource(int64(k))), 1+k*3)}})
			}
			g.Edges = append(g.Edges, Edge{From: 0, To: 0, Length: 1, Hidden: true})
			return g
		},
		"같은 쌍 20 간선·순환": func(d Dir) *Graph {
			g := base(d, nodes(2, 40, 20))
			for k := 0; k < 20; k++ {
				g.Edges = append(g.Edges, Edge{From: k % 2, To: 1 - k%2, Length: 1 + k%3, Label: []string{fmt.Sprint(k)}})
			}
			return g
		},
		"묶음 10·모두 구성원": func(d Dir) *Graph {
			g := base(d, nodes(40, 50, 25))
			for c := 0; c < 10; c++ {
				g.Clusters = append(g.Clusters, Cluster{Title: strings.Repeat("제목 ", 1+c*8)})
			}
			for i := range g.Nodes {
				g.Nodes[i].Cluster = i % 10
			}
			for i := 0; i+1 < 40; i++ {
				g.Edges = append(g.Edges, Edge{From: i, To: (i * 7) % 40, Length: 1})
			}
			return g
		},
		"빈 묶음": func(d Dir) *Graph {
			g := base(d, nodes(3, 40, 20))
			g.Clusters = []Cluster{{Title: "빈"}, {Title: "찬"}}
			g.Nodes[1].Cluster = 1
			g.Edges = []Edge{{From: 0, To: 1, Length: 1}, {From: 1, To: 2, Length: 1}}
			return g
		},
	}
	for name, f := range cases {
		for _, d := range dirs {
			g := f(d)
			n := fmt.Sprintf("%s(%s)", name, d)
			r := mustLayout(t, n, g)
			checkProps(t, n, g, r)
		}
	}
}

// 사용자 Clip — 간선 끝은 부르는 쪽이 준 점이다(여기서는 상자에 내접한 타원 위). 콜백은 그 끝의 노드 차례로 불린다.
func TestCustomClip(t *testing.T) {
	ellipse := func(b Box, to Pt) Pt {
		dx, dy := to.X-b.C.X, to.Y-b.C.Y
		a, c := b.W/2, b.H/2
		k := 1 / math.Sqrt(dx*dx/(a*a)+dy*dy/(c*c))
		return Pt{b.C.X + dx*k, b.C.Y + dy*k}
	}
	rng := rand.New(rand.NewSource(7))
	for it := 0; it < 40; it++ {
		g := randGraph(rng, genOpt{nodes: 2 + rng.Intn(20), edges: 1 + rng.Intn(40), clusters: rng.Intn(3)}, dirs[it%4])
		called := map[int]bool{}
		g.Clip = func(i int, b Box, to Pt) Pt {
			called[i] = true
			if b.W != g.Nodes[i].W || b.H != g.Nodes[i].H {
				t.Errorf("Clip(%d) 의 상자 %v×%v, 노드 %v×%v", i, b.W, b.H, g.Nodes[i].W, g.Nodes[i].H)
			}
			return ellipse(b, to)
		}
		r := mustLayout(t, "clip", g)
		for k, e := range r.Edges {
			in := g.Edges[k]
			if len(e.Pts) == 0 {
				continue
			}
			for _, end := range []struct {
				p Pt
				v int
			}{{e.Pts[0], in.From}, {e.Pts[len(e.Pts)-1], in.To}} {
				b := r.Nodes[end.v]
				if !called[end.v] {
					t.Fatalf("판 %d: 노드 %d 의 Clip 이 불리지 않음", it, end.v)
				}
				u, v := (end.p.X-b.C.X)/(b.W/2), (end.p.Y-b.C.Y)/(b.H/2)
				if d := math.Abs(math.Hypot(u, v) - 1); d > 1e-6 {
					t.Fatalf("판 %d: 간선 %d 끝 %v 가 노드 %d 의 타원 위가 아님(%.4f)", it, k, end.p, end.v, d)
				}
			}
		}
	}
}

// 점 상한 — 계층 축으로 긴 사슬(노드 200, 길이 3 → 계층 차이 6×199 이상) 끝끼리를 잇는 간선 10개는 그 하나만으로도
// 더미가 6×199−1 개라 노드 200 + 사슬 더미 199×5 + 10×1193 = 13125 > MaxPoints. 오류여야 하고(패닉 아님), 같은 사슬에
// 그 간선이 없으면 그린다.
func TestMaxPoints(t *testing.T) {
	g := &Graph{Dir: TB, NodeSep: 30, RankSep: 40}
	for i := 0; i < 200; i++ {
		g.Nodes = append(g.Nodes, Node{W: 30, H: 20, Cluster: -1})
	}
	for i := 0; i+1 < 200; i++ {
		g.Edges = append(g.Edges, Edge{From: i, To: i + 1, Length: 3})
	}
	if lower := 200 + 199*5 + 10*(6*199-1); lower <= MaxPoints {
		t.Fatalf("시험 그래프의 점 하한 %d 가 상한 %d 밑", lower, MaxPoints)
	}
	if _, ok := layoutOK(t, "사슬", copyGraph(g)); !ok {
		t.Fatal("사슬만으로 점 상한 오류")
	}
	for k := 0; k < 10; k++ {
		g.Edges = append(g.Edges, Edge{From: 0, To: 199, Length: 1, Label: []string{"긴"}})
	}
	if _, ok := layoutOK(t, "사슬 + 긴 간선", g); ok {
		t.Fatal("점 상한을 넘는 그래프를 그림")
	}
	// 묶음 경계 더미로 넘는 길도 오류다(구성원 둘이 사슬 양 끝)
	g.Edges = g.Edges[:199]
	g.Clusters = make([]Cluster, 10)
	for c := 0; c < 10; c++ {
		g.Nodes[c].Cluster, g.Nodes[199-c].Cluster = c, c
	}
	if lower := 200 + 199*5 + 10*(6*(199-2*9)-1); lower <= MaxPoints {
		t.Fatalf("묶음 시험 그래프의 점 하한 %d 가 상한 %d 밑", lower, MaxPoints)
	}
	if _, ok := layoutOK(t, "사슬 + 묶음", g); ok {
		t.Fatal("묶음 경계 더미로 점 상한을 넘는 그래프를 그림")
	}
}

// 시간 — 상한 근처의 큰 입력(노드 200·간선 400·묶음 10, 라벨 많이)도 감당할 수 있는 시간 안. 두 배씩 키워 이차 증가가 없는지도.
func TestTime(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for _, d := range dirs {
		g := randGraph(rng, genOpt{nodes: 200, edges: 400, clusters: 10, local: 3}, d)
		var err error
		el, ok := timingtest.Within(3*time.Second, func() { _, err = Layout(copyGraph(g)) })
		t.Logf("%s: %v, 오류 %v", d, el, err)
		if err != nil {
			t.Errorf("%s: 노드 200·간선 400(점 상한 밑) 배치 오류 %v", d, err)
		}
		if !ok {
			t.Errorf("%s: 노드 200·간선 400 배치가 %v", d, el)
		}
	}
	// 점 상한 바로 밑 — 사슬 150(계층 2i) + 첫 노드에서 끝 쪽 노드 46 개로 가는 긴 라벨 간선: 점 150 + 149 + Σ(2(149−i)−1) = 11891
	g := &Graph{Dir: TB, NodeSep: 30, RankSep: 40}
	for i := 0; i < 150; i++ {
		g.Nodes = append(g.Nodes, Node{W: 40, H: 20, Cluster: -1})
	}
	for i := 0; i < 149; i++ {
		g.Edges = append(g.Edges, Edge{From: i, To: i + 1, Length: 1})
	}
	for i := 0; i < 46; i++ {
		g.Edges = append(g.Edges, Edge{From: 0, To: 149 - i, Length: 1, Label: []string{fmt.Sprintf("라벨 %d", i)}})
	}
	var r *Result
	var err error
	el, ok := timingtest.Within(3*time.Second, func() { r, err = Layout(copyGraph(g)) })
	t.Logf("점 상한 바로 밑: %v, 오류 %v", el, err)
	if err != nil || !ok {
		t.Errorf("점 상한 바로 밑: %v, 오류 %v", el, err)
	} else {
		checkProps(t, "점 상한 바로 밑", g, r)
	}
	timingtest.AssertDoubling(t, "사슬", []int{400, 800, 1600}, func(n int) func() {
		g := &Graph{Dir: TB, NodeSep: 30, RankSep: 40}
		for i := 0; i < n; i++ {
			g.Nodes = append(g.Nodes, Node{W: 30, H: 20, Cluster: -1})
		}
		for i := 0; i+1 < n; i++ {
			g.Edges = append(g.Edges, Edge{From: i, To: i + 1, Length: 1, Label: []string{"x"}})
		}
		return func() {
			if _, err := Layout(g); err != nil {
				t.Fatal(err)
			}
		}
	}, timingtest.Doubling{Limit: 3 * time.Second, Slack: 20 * time.Millisecond})
}

// ---- 도우미 단위 시험

func TestLabelSize(t *testing.T) {
	w0, h0 := LabelSize(nil)
	if w0 <= 0 || h0 <= 0 {
		t.Errorf("빈 라벨 상자 %v×%v — 바탕 여백이 있어야", w0, h0)
	}
	w1, h1 := LabelSize([]string{"가"})
	w2, h2 := LabelSize([]string{"가", "가"})
	_, h3 := LabelSize([]string{"가", "가", "가"})
	if !(h1 > h0 && h2 > h1) || math.Abs((h3-h2)-(h2-h1)) > 1e-9 || math.Abs((h2-h1)-(h1-h0)) > 1e-9 {
		t.Errorf("줄마다 같은 높이로 늘어야: %v %v %v %v", h0, h1, h2, h3)
	}
	if w1 != w2 || w1 <= w0 {
		t.Errorf("같은 줄을 더해도 폭은 같고 글자가 있으면 넓어야: %v %v %v", w0, w1, w2)
	}
	a, _ := LabelSize([]string{"짧음"})
	b, _ := LabelSize([]string{strings.Repeat("긺", 40)})
	ab, _ := LabelSize([]string{"짧음", strings.Repeat("긺", 40)})
	if !(b > a) || ab != b {
		t.Errorf("폭은 가장 긴 줄: %v %v %v", a, b, ab)
	}
	if e, _ := LabelSize([]string{""}); e != w0 {
		t.Errorf("빈 줄 하나의 폭 %v, 빈 라벨 %v", e, w0)
	}
}

func TestClipRect(t *testing.T) {
	b := Box{C: Pt{100, 50}, W: 40, H: 20}
	for _, c := range []struct {
		to, want Pt
	}{
		{Pt{100, 50}, Pt{100, 50}},   // 가운데 → 가운데
		{Pt{500, 50}, Pt{120, 50}},   // 오른쪽
		{Pt{-500, 50}, Pt{80, 50}},   // 왼쪽
		{Pt{100, 51}, Pt{100, 60}},   // 아래(상자 안의 점도 테두리까지)
		{Pt{100, -9}, Pt{100, 40}},   // 위
		{Pt{120, 60}, Pt{120, 60}},   // 모서리
		{Pt{140, 70}, Pt{120, 60}},   // 모서리 방향 멀리
		{Pt{160, 60}, Pt{120, 53.3}}, // 옆면(기울기 1/6)
		{Pt{110, 100}, Pt{102, 60}},  // 아랫면(기울기 5)
	} {
		got := ClipRect(b, c.to)
		if math.Abs(got.X-c.want.X) > 0.05 || math.Abs(got.Y-c.want.Y) > 0.05 {
			t.Errorf("ClipRect(%v) = %v, 기대 %v", c.to, got, c.want)
		}
	}
	if got := ClipRect(Box{C: Pt{3, 4}}, Pt{10, 10}); got != (Pt{3, 4}) {
		t.Errorf("크기 0 상자 %v", got)
	}
	if !math.IsInf(RectT(5, 5, 0, 0), 1) || RectT(0, 5, 1, 0) != 0 || RectT(4, 2, 2, 2) != 1 {
		t.Errorf("RectT 경계 %v %v %v", RectT(5, 5, 0, 0), RectT(0, 5, 1, 0), RectT(4, 2, 2, 2))
	}
}

func TestBasis(t *testing.T) {
	for _, c := range []struct {
		p    []Pt
		want string
	}{
		{[]Pt{{0, 0}, {10, 5}}, "M0 0 L10 5"},
		{[]Pt{{0.04, -0.04}, {1.25, 2.56}}, "M0 0 L1.3 2.6"}, // 소수 한 자리, -0 없음
		// 손으로 푼 d3 curveBasis: 세 점 (0,0)(6,0)(12,0)
		{[]Pt{{0, 0}, {6, 0}, {12, 0}}, "M0 0 L1 0 C2 0 4 0 6 0 C8 0 10 0 11 0 L12 0"},
		// 네 점 (0,0)(6,6)(12,0)(18,6)
		{[]Pt{{0, 0}, {6, 6}, {12, 0}, {18, 6}}, "M0 0 L1 1 C2 2 4 4 6 4 C8 4 10 2 12 2 C14 2 16 4 17 5 L18 6"},
	} {
		if got := Basis(c.p); got != c.want {
			t.Errorf("Basis(%v) = %q, 기대 %q", c.p, got, c.want)
		}
	}
}
