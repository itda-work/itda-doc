// Package layered 는 그림 종류가 함께 쓰는 계층 배치(Sugiyama)다(itda-hyve#409 ④ — 흐름도 diagram/flowchart 의 배치를 꺼냈다).
// 입력은 상자(노드)의 크기·subgraph 소속, 간선의 양끝·길이·라벨 줄, 방향이고, 출력은 상자 가운데·간선이 지나는 점·라벨
// 자리다. 모양은 모른다 — 간선 끝을 상자 테두리에서 자르는 일은 부르는 쪽의 Clip(없으면 사각형 ClipRect)이 한다.
// 흐름도(flowchart)·ER(er)이 쓰고, 상태도·클래스도가 같은 입구를 쓴다.
//
// 계산은 "위 → 아래" 공간에서 한다 — 계층 축(rank, 아래로 y)과 순서 축(order, 오른쪽 x).
// LR·RL·BT 는 마지막에 축을 바꾸거나 뒤집는다(toFinal).
//
//  1. 순환 끊기: 선언 차례의 깊이 우선 탐색에서 뒤로 가는 간선을 거꾸로 센다(그릴 때는 원래 방향).
//  2. 계층: 간선마다 최소 길이 2×길이(dagre 처럼 계층을 두 배로 — 사이 계층에 간선 라벨이 더미 노드로 들어간다)로
//     가장 긴 경로, 그다음 나가는 간선이 들어오는 간선보다 많은 노드를 아래로 당긴다.
//  3. 더미: 두 계층 넘게 걸친 간선은 계층마다 더미. 라벨은 가운데 더미가 라벨 크기를 갖는다. subgraph 의 계층 범위에서
//     구성원이 없는 계층에는 너비 0 의 경계 더미를 넣어 순서에서 자리를 지킨다.
//  4. 순서: 초기 차례(깊이 우선) → 위·아래로 번갈아 무게중심 정렬(이웃 없는 노드는 자리 지킴) → subgraph 구성원을 붙이고
//     subgraph 끼리의 차례를 모든 계층에서 같게 → 교차 수(펜윅 트리)가 가장 적은 차례를 남긴다.
//  5. 좌표: 계층마다 이웃의 가중 평균을 바라는 자리로, 간격 제약 안에서 제곱 오차가 가장 작은 자리(가중 등위 회귀, PAVA).
//     긴 간선의 더미끼리는 무게를 크게(곧게). 그다음 subgraph 상자를 통째로 오른쪽으로 밀어 겹침을 푼다.
//
// 계산량: 점(노드 + 더미) V, 간선 조각 E 에 대해 순서는 반복 24번 × O(E log V), 좌표는 반복 12번 × O(V + E).
// V 는 MaxPoints 로 묶인다. 같은 입력은 같은 출력이다(map 순회로 차례를 정하지 않는다). 곱셈-덧셈은 명시적 float64(…)
// 변환으로 FMA 합치기를 막는다 — 아키텍처마다 끝자리가 같다(흐름도 TestCorpusHash).
package layered

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/itda-work/itda-doc/diagram"
)

// MaxPoints 는 배치에 쓰는 점(노드 + 간선이 지나는 더미)의 상한이다 — 긴 간선이 많은 그림도 계산 시간이 묶이게.
const MaxPoints = 12000

const (
	lineH      = diagram.LineH // 공통 — subgraph 제목 띠
	edgeSep    = 14.0
	clusterPad = 14.0
	margin     = 8.0
)

// Dir 는 그리는 방향이다(흐름도 flowchart.Dir 와 같은 차례).
type Dir int

const (
	TB Dir = iota // 위 → 아래
	BT
	LR
	RL
)

// Pt 는 점이다.
type Pt struct{ X, Y float64 }

// Node 는 배치할 상자 하나다(W·H 는 최종 좌표의 너비·높이). Cluster 는 들어 있는 subgraph(Graph.Clusters 의 차례), 없으면 -1.
type Node struct {
	W, H    float64
	Cluster int
}

// Edge 는 간선 하나다. Length 는 1 이상(계층 2×Length 만큼 떨어진다). Label 은 줄로 나눈 라벨(없으면 nil) — 가운데
// 더미가 그 크기(LabelSize)를 갖는다. Hidden 은 배치에만 쓰고 그리지 않는 간선이다(자기 간선이면 고리를 만들지 않는다).
type Edge struct {
	From, To int
	Length   int
	Label    []string
	Hidden   bool
}

// Cluster 는 subgraph 하나다(제목은 줄바꿈 전 글).
type Cluster struct {
	Title string
}

// Graph 는 배치 입력이다. NodeSep 은 순서 축에서 이웃한 상자 사이, RankSep 은 노드 계층 사이(가운데 라벨 계층의 반씩)다.
// LoopOut 은 자기 간선 고리가 노드의 계층 축 끝보다 더 나가는 거리다(0 이면 고리 꼭대기가 노드 끝과 같은 높이 — 흐름도.
// ER 은 고리 끝 기호가 들어갈 자리만큼 띄운다). Clip 은 상자 i(최종 좌표 b)의 가운데에서 to 쪽으로 간 선이 모양의 테두리와
// 만나는 점이다(nil 이면 ClipRect).
type Graph struct {
	Dir      Dir
	Nodes    []Node
	Edges    []Edge
	Clusters []Cluster
	NodeSep  float64
	RankSep  float64
	LoopOut  float64
	Clip     func(i int, b Box, to Pt) Pt
}

// Box 는 배치된 상자다(가운데 C).
type Box struct {
	C    Pt
	W, H float64
}

// EdgePath 는 배치된 간선이다. Pts 는 지나는 점(원래 방향 — From 쪽이 처음, 양 끝은 상자 테두리)이고 자기 간선은
// 일곱 점의 고리다(Self). LC 는 라벨 가운데, LW·LH 는 라벨 상자 크기(LabelSize).
type EdgePath struct {
	Pts    []Pt
	Self   bool
	Label  []string
	LC     Pt
	LW, LH float64
}

// ClusterBox 는 배치된 subgraph 상자다.
type ClusterBox struct {
	X, Y, W, H float64
	Title      []string
	Show       bool
}

// Result 는 배치 결과다(최종 좌표, 모든 것이 margin 부터 시작한다).
type Result struct {
	W, H     float64
	Nodes    []Box
	Edges    []EdgePath
	Clusters []ClusterBox
}

// LabelSize 는 간선 라벨 상자(바탕 포함)의 크기다.
func LabelSize(lines []string) (float64, float64) {
	return diagram.LinesW(lines) + 8, float64(float64(len(lines))*lineH) + 4
}

// lnode 는 배치용 점이다: 실제 노드, 간선 더미, 라벨 더미, subgraph 경계 더미.
type lnode struct {
	real    int // 실제 노드 차례, 아니면 -1
	label   bool
	self    bool    // 자기 간선의 더미(노드 바로 오른쪽, 고리가 이 더미를 감싸고 라벨이 그 가운데)
	loopDy  float64 // 자기 간선 고리의 계층 축 반 높이
	border  bool
	cluster int
	lo, ro  float64 // 순서 축의 왼쪽·오른쪽 반 너비
	hr      float64 // 계층 축의 높이
	rank    int
	order   int
	x       float64
	ups     []int
	downs   []int
}

type lgraph struct {
	g                *Graph
	nodeSep, rankSep float64
	n                []lnode
	layers           [][]int
	chains           [][]int // 간선마다 위 → 아래의 점 차례(자기 간선은 nil)
	rev              []bool
	nc               int // subgraph 수
	cOrder           []float64
	vert             bool // 계층 축이 세로(TB·BT)

	selfEdges [][]int // 노드마다 자기 간선(그리는 것만, 원문 차례)
	selfDummy []int   // 자기 간선 → 그 더미

	// rankAxis 가 정한다
	band, y  []float64  // 계층마다 띠 높이·가운데
	clo, chi []int      // subgraph 의 계층 범위(구성원이 없으면 chi < 0)
	titles   [][]string // subgraph 제목 줄
	titleHt  []float64  // 제목 띠 높이
}

// Layout 은 그래프를 배치한다. 점 수가 MaxPoints 를 넘으면 *diagram.Error 다.
func Layout(g *Graph) (*Result, error) {
	lg := &lgraph{g: g, nodeSep: g.NodeSep, rankSep: g.RankSep, nc: len(g.Clusters), vert: g.Dir == TB || g.Dir == BT}
	sizes := make([][2]float64, len(g.Nodes))
	for i, n := range g.Nodes {
		sizes[i] = [2]float64{n.W, n.H}
	}
	edgeLines := make([][]string, len(g.Edges))
	for i, e := range g.Edges {
		edgeLines[i] = e.Label
	}
	lg.acyclic()
	rank := lg.ranks()
	if err := lg.build(rank, sizes, edgeLines); err != nil {
		return nil, err
	}
	lg.order()
	if err := lg.insertSelf(edgeLines); err != nil {
		return nil, err
	}
	lg.rankAxis()
	lg.position()
	return lg.finish(sizes, edgeLines), nil
}

// ---- 1. 순환 끊기

func (lg *lgraph) acyclic() {
	g := lg.g
	lg.rev = make([]bool, len(g.Edges))
	out := make([][]int, len(g.Nodes))
	for i, e := range g.Edges {
		if e.From != e.To {
			out[e.From] = append(out[e.From], i)
		}
	}
	state := make([]int8, len(g.Nodes)) // 0 안 감, 1 스택 위, 2 끝남
	type frame struct{ v, next int }
	for root := range g.Nodes {
		if state[root] != 0 {
			continue
		}
		stack := []frame{{root, 0}}
		state[root] = 1
		for len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.next >= len(out[f.v]) {
				state[f.v] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			ei := out[f.v][f.next]
			f.next++
			w := g.Edges[ei].To
			switch state[w] {
			case 1:
				lg.rev[ei] = true
			case 0:
				state[w] = 1
				stack = append(stack, frame{w, 0})
			}
		}
	}
}

// ends 는 배치에서의 위·아래 끝이다(거꾸로 센 간선은 뒤집어서).
func (lg *lgraph) ends(i int) (int, int) {
	e := lg.g.Edges[i]
	if lg.rev[i] {
		return e.To, e.From
	}
	return e.From, e.To
}

// ---- 2. 계층

func (lg *lgraph) ranks() []int {
	g := lg.g
	n := len(g.Nodes)
	rank := make([]int, n)
	indeg := make([]int, n)
	ins := make([][]int, n)
	outs := make([][]int, n)
	for i, e := range g.Edges {
		if e.From == e.To {
			continue
		}
		a, b := lg.ends(i)
		indeg[b]++
		ins[b] = append(ins[b], i)
		outs[a] = append(outs[a], i)
	}
	var topo, queue []int
	for v := 0; v < n; v++ {
		if indeg[v] == 0 {
			queue = append(queue, v)
		}
	}
	for head := 0; head < len(queue); head++ {
		v := queue[head]
		topo = append(topo, v)
		for _, ei := range outs[v] {
			_, b := lg.ends(ei)
			if r := rank[v] + 2*g.Edges[ei].Length; r > rank[b] {
				rank[b] = r
			}
			if indeg[b]--; indeg[b] == 0 {
				queue = append(queue, b)
			}
		}
	}
	// 아래로 당기기: 나가는 간선이 더 많은 노드(들어오는 간선이 없으면 늘)를 가장 가까운 자식 쪽으로 — 들어오는 간선은
	// 늘고 나가는 간선은 줄어 전체 간선 길이가 준다. 뒤 차례부터라 자식은 이미 자리를 잡았다.
	for k := len(topo) - 1; k >= 0; k-- {
		v := topo[k]
		if len(outs[v]) == 0 || len(outs[v]) <= len(ins[v]) && len(ins[v]) > 0 {
			continue
		}
		hi := math.MaxInt
		for _, ei := range outs[v] {
			_, b := lg.ends(ei)
			hi = min(hi, rank[b]-2*g.Edges[ei].Length)
		}
		if hi > rank[v] {
			rank[v] = hi
		}
	}
	lo := math.MaxInt
	for _, r := range rank {
		lo = min(lo, r)
	}
	for i := range rank {
		rank[i] -= lo
	}
	return rank
}

// ---- 3. 더미

func (lg *lgraph) add(v lnode) int {
	lg.n = append(lg.n, v)
	return len(lg.n) - 1
}

func (lg *lgraph) link(a, b int) {
	lg.n[a].downs = append(lg.n[a].downs, b)
	lg.n[b].ups = append(lg.n[b].ups, a)
}

func (lg *lgraph) build(rank []int, sizes [][2]float64, edgeLines [][]string) error {
	g := lg.g
	extent := func(w, h float64) (lo, ro, hr float64) {
		if lg.vert {
			return w / 2, w / 2, h
		}
		return h / 2, h / 2, w
	}
	// 자기 간선은 순서를 정한 뒤 노드 바로 오른쪽에 간선마다 더미를 하나씩 끼운다(insertSelf) — 여기서는 모아만 둔다
	lg.selfEdges = make([][]int, len(g.Nodes))
	for i, e := range g.Edges {
		if e.From == e.To && !e.Hidden {
			lg.selfEdges[e.From] = append(lg.selfEdges[e.From], i)
		}
	}
	lg.selfDummy = make([]int, len(g.Edges))
	maxRank := 0
	for v, nd := range g.Nodes {
		lo, ro, hr := extent(sizes[v][0], sizes[v][1])
		lg.add(lnode{real: v, cluster: nd.Cluster, lo: lo, ro: ro, hr: hr, rank: rank[v]})
		maxRank = max(maxRank, rank[v])
	}
	lg.chains = make([][]int, len(g.Edges))
	for i, e := range g.Edges {
		if e.From == e.To {
			continue
		}
		a, b := lg.ends(i)
		ra, rb := rank[a], rank[b]
		cl := -1
		if c := g.Nodes[a].Cluster; c >= 0 && c == g.Nodes[b].Cluster {
			cl = c
		}
		mid := ra + (rb-ra)/2
		chain := []int{a}
		prev := a
		for r := ra + 1; r < rb; r++ {
			d := lnode{real: -1, cluster: cl, rank: r}
			if r == mid && len(edgeLines[i]) > 0 {
				d.label = true
				d.lo, d.ro, d.hr = extent(LabelSize(edgeLines[i]))
			}
			id := lg.add(d)
			lg.link(prev, id)
			chain = append(chain, id)
			prev = id
		}
		lg.link(prev, b)
		lg.chains[i] = append(chain, b)
		if len(lg.n) > MaxPoints {
			return &diagram.Error{Msg: fmt.Sprintf("그림이 너무 큼(배치 점이 %d 개를 넘음 — 노드·긴 화살표를 줄인다)", MaxPoints)}
		}
	}
	// subgraph 경계 더미
	if lg.nc > 0 {
		lo := make([]int, lg.nc)
		hi := make([]int, lg.nc)
		for c := range lo {
			lo[c], hi[c] = math.MaxInt, -1
		}
		has := make(map[[2]int]bool)
		for _, v := range lg.n {
			if v.cluster >= 0 {
				lo[v.cluster] = min(lo[v.cluster], v.rank)
				hi[v.cluster] = max(hi[v.cluster], v.rank)
				has[[2]int{v.cluster, v.rank}] = true
			}
		}
		for c := 0; c < lg.nc; c++ {
			for r := lo[c]; r <= hi[c]; r++ {
				if !has[[2]int{c, r}] {
					lg.add(lnode{real: -1, border: true, cluster: c, rank: r})
				}
			}
		}
		if len(lg.n) > MaxPoints {
			return &diagram.Error{Msg: fmt.Sprintf("그림이 너무 큼(배치 점이 %d 개를 넘음 — 노드·긴 화살표를 줄인다)", MaxPoints)}
		}
	}
	lg.layers = make([][]int, maxRank+1)
	return nil
}

// ---- 4. 순서

func (lg *lgraph) order() {
	// 초기 차례: 계층·선언 차례로 실제 노드에서 아래로 깊이 우선
	seen := make([]bool, len(lg.n))
	var visit func(v int)
	visit = func(v int) {
		if seen[v] {
			return
		}
		seen[v] = true
		lg.layers[lg.n[v].rank] = append(lg.layers[lg.n[v].rank], v)
		for _, d := range lg.n[v].downs {
			visit(d)
		}
	}
	roots := make([]int, len(lg.g.Nodes))
	for i := range roots {
		roots[i] = i
	}
	sort.SliceStable(roots, func(i, j int) bool { return lg.n[roots[i]].rank < lg.n[roots[j]].rank })
	for _, v := range roots {
		visit(v)
	}
	for v := range lg.n {
		visit(v) // 경계 더미
	}
	lg.renumber()
	lg.clusterOrder()
	for r := range lg.layers {
		lg.group(r)
	}
	best := lg.snapshot()
	bestC := lg.crossings()
	bestCO := append([]float64(nil), lg.cOrder...)
	stale := 0
	for it := 0; it < 24 && bestC > 0 && stale < 4; it++ {
		lg.clusterOrder()
		if it%2 == 0 {
			for r := 1; r < len(lg.layers); r++ {
				lg.sortLayer(r, true)
			}
		} else {
			for r := len(lg.layers) - 2; r >= 0; r-- {
				lg.sortLayer(r, false)
			}
		}
		if c := lg.crossings(); c < bestC {
			best, bestC, stale = lg.snapshot(), c, 0
			bestCO = append(bestCO[:0], lg.cOrder...)
		} else {
			stale++
		}
	}
	lg.layers = best
	lg.renumber()
	// 쓸기는 한쪽 끝 계층(위로 쓸 때 맨 위, 아래로 쓸 때 맨 아래)을 다시 묶지 않아, 그 계층만 앞 반복의 subgraph 차례로 남을 수
	// 있다 — 계층끼리 subgraph 차례가 어긋나면 unoverlapClusters 의 제약에 순환이 생겨 상자가 겹친다. 고른 차례(best)를 만든
	// 반복의 subgraph 차례로 두 끝 계층을 다시 묶는다(이미 그 차례면 group 은 아무것도 바꾸지 않는다 — itda-hyve#409 ④-1 후속
	// diagram/layered 자체 시험 TestRandomProperties 가 찾음).
	if lg.nc > 0 && len(lg.layers) > 0 {
		lg.cOrder = bestCO
		lg.group(0)
		lg.group(len(lg.layers) - 1)
	}
}

func (lg *lgraph) renumber() {
	for _, l := range lg.layers {
		for i, v := range l {
			lg.n[v].order = i
		}
	}
}

func (lg *lgraph) renumberLayer(r int) {
	for i, v := range lg.layers[r] {
		lg.n[v].order = i
	}
}

func (lg *lgraph) snapshot() [][]int {
	out := make([][]int, len(lg.layers))
	for i, l := range lg.layers {
		out[i] = append([]int(nil), l...)
	}
	return out
}

// sortLayer 는 계층 r 을 위(up) 또는 아래 계층 이웃의 무게중심으로 정렬한다. 이웃 없는 점은 자리를 지킨다.
func (lg *lgraph) sortLayer(r int, up bool) {
	l := lg.layers[r]
	bc := make([]float64, len(l))
	fixed := make([]bool, len(l))
	for i, v := range l {
		nb := lg.n[v].downs
		if up {
			nb = lg.n[v].ups
		}
		if len(nb) == 0 {
			fixed[i] = true
			continue
		}
		s := 0.0
		for _, u := range nb {
			s += float64(lg.n[u].order)
		}
		bc[i] = s / float64(len(nb))
	}
	var movable []int
	for i := range l {
		if !fixed[i] {
			movable = append(movable, i)
		}
	}
	sorted := append([]int(nil), movable...)
	sort.SliceStable(sorted, func(a, b int) bool { return bc[sorted[a]] < bc[sorted[b]] })
	out := append([]int(nil), l...)
	for k, slot := range movable {
		out[slot] = l[sorted[k]]
	}
	lg.layers[r] = out
	lg.renumberLayer(r)
	lg.group(r)
}

// clusterOrder 는 subgraph 끼리의 차례 열쇠다(구성원의 계층 안 상대 자리 평균) — 모든 계층에서 같은 차례로 둔다.
func (lg *lgraph) clusterOrder() {
	if lg.nc == 0 {
		return
	}
	sum := make([]float64, lg.nc)
	cnt := make([]float64, lg.nc)
	for _, l := range lg.layers {
		for i, v := range l {
			if c := lg.n[v].cluster; c >= 0 {
				sum[c] += float64(i+1) / float64(len(l)+1)
				cnt[c]++
			}
		}
	}
	lg.cOrder = make([]float64, lg.nc)
	for c := range sum {
		if cnt[c] > 0 {
			lg.cOrder[c] = sum[c] / cnt[c]
		}
	}
}

// group 은 계층 r 에서 subgraph 구성원을 붙이고(덩이의 자리는 구성원 자리의 평균) subgraph 덩이들을 cOrder 차례로 둔다.
func (lg *lgraph) group(r int) {
	if lg.nc == 0 {
		return
	}
	l := lg.layers[r]
	type block struct {
		c       int
		members []int
		key     float64
	}
	var blocks []*block
	byC := map[int]*block{}
	for i, v := range l {
		c := lg.n[v].cluster
		if c < 0 {
			blocks = append(blocks, &block{c: -1, members: []int{v}, key: float64(i)})
			continue
		}
		b := byC[c]
		if b == nil {
			b = &block{c: c}
			byC[c] = b
			blocks = append(blocks, b)
		}
		b.members = append(b.members, v)
		b.key += float64(i)
	}
	if len(byC) == 0 {
		return
	}
	for _, b := range blocks {
		if b.c >= 0 {
			b.key /= float64(len(b.members))
		}
	}
	sort.SliceStable(blocks, func(i, j int) bool { return blocks[i].key < blocks[j].key })
	var slots []int
	var cbs []*block
	for i, b := range blocks {
		if b.c >= 0 {
			slots = append(slots, i)
			cbs = append(cbs, b)
		}
	}
	sort.SliceStable(cbs, func(i, j int) bool {
		if lg.cOrder[cbs[i].c] != lg.cOrder[cbs[j].c] {
			return lg.cOrder[cbs[i].c] < lg.cOrder[cbs[j].c]
		}
		return cbs[i].c < cbs[j].c
	})
	for k, s := range slots {
		blocks[s] = cbs[k]
	}
	out := l[:0:0]
	for _, b := range blocks {
		out = append(out, b.members...)
	}
	lg.layers[r] = out
	lg.renumberLayer(r)
}

// crossings 는 이웃 계층 사이 간선 조각의 교차 수다(정렬 + 펜윅 트리, O(E log V)).
func (lg *lgraph) crossings() int {
	total := 0
	for r := 0; r+1 < len(lg.layers); r++ {
		var pairs [][2]int
		for _, v := range lg.layers[r] {
			for _, d := range lg.n[v].downs {
				pairs = append(pairs, [2]int{lg.n[v].order, lg.n[d].order})
			}
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i][0] != pairs[j][0] {
				return pairs[i][0] < pairs[j][0]
			}
			return pairs[i][1] < pairs[j][1]
		})
		size := len(lg.layers[r+1]) + 1
		tree := make([]int, size+1)
		seen := 0
		for _, p := range pairs {
			// 지금까지 넣은 것 가운데 아래 끝이 p 보다 오른쪽인 수
			le := 0
			for i := p[1] + 1; i > 0; i -= i & -i {
				le += tree[i]
			}
			total += seen - le
			for i := p[1] + 1; i <= size; i += i & -i {
				tree[i]++
			}
			seen++
		}
	}
	return total
}

// insertSelf 는 자기 간선마다 더미를 노드 바로 오른쪽(순서 축 + 쪽)에 원문 차례로 끼운다(dagre 의 self edge 와 같다).
// 더미는 라벨 크기를 가져 라벨의 두 축 크기가 간격·계층 높이에 들어가고, 고리는 그 더미를 감싼다. 같은 노드의 k 번째 고리는
// 앞 고리보다 더 멀리(더미가 더 오른쪽), 더 높게(반 높이 + 8k) 돌아 앞 고리의 라벨 위아래를 지난다.
// 순서를 정한 뒤에 끼운다 — 이웃이 없어 교차 수에 무관하고, 노드와 떨어지지 않게 place 가 붙여 둔다.
func (lg *lgraph) insertSelf(edgeLines [][]string) error {
	any := false
	for _, es := range lg.selfEdges {
		any = any || len(es) > 0
	}
	if !any {
		return nil
	}
	for r, l := range lg.layers {
		var out []int
		for _, v := range l {
			out = append(out, v)
			if v >= len(lg.g.Nodes) || len(lg.selfEdges[v]) == 0 {
				continue
			}
			nd := lg.n[v]
			maxLh := 0.0
			for _, ei := range lg.selfEdges[v] {
				if ls := edgeLines[ei]; len(ls) > 0 {
					_, lh := LabelSize(ls)
					if !lg.vert {
						lh, _ = LabelSize(ls)
					}
					maxLh = math.Max(maxLh, lh)
				}
			}
			base := math.Max(nd.hr/2+lg.g.LoopOut, maxLh/2+6)
			for k, ei := range lg.selfEdges[v] {
				d := lnode{real: -1, self: true, cluster: nd.cluster, rank: nd.rank, lo: 5, ro: 5}
				if ls := edgeLines[ei]; len(ls) > 0 {
					w, h := LabelSize(ls)
					d.label = true
					if lg.vert {
						d.lo, d.ro = w/2, w/2
					} else {
						d.lo, d.ro = h/2, h/2
					}
				}
				d.loopDy = base + float64(8*k)
				d.hr = float64(2*d.loopDy) + 4
				id := lg.add(d)
				lg.selfDummy[ei] = id
				out = append(out, id)
			}
		}
		lg.layers[r] = out
		lg.renumberLayer(r)
	}
	if len(lg.n) > MaxPoints {
		return &diagram.Error{Msg: fmt.Sprintf("그림이 너무 큼(배치 점이 %d 개를 넘음 — 노드·긴 화살표를 줄인다)", MaxPoints)}
	}
	return nil
}

// ---- 5. 좌표

func (lg *lgraph) isDummy(v int) bool { return lg.n[v].real < 0 }

// sep 은 이웃한 두 점 a(왼쪽)·b 의 가운데 사이 최소 거리다.
func (lg *lgraph) sep(a, b int) float64 {
	na, nb := &lg.n[a], &lg.n[b]
	var gap float64
	switch da, db := lg.isDummy(a) && !na.label, lg.isDummy(b) && !nb.label; {
	case da && db:
		gap = edgeSep
	case da || db:
		gap = (edgeSep + lg.nodeSep) / 2
	default:
		gap = lg.nodeSep
	}
	if na.cluster != nb.cluster {
		if na.cluster >= 0 {
			gap += clusterPad
		}
		if nb.cluster >= 0 {
			gap += clusterPad
			if !lg.vert {
				gap += lg.titleHt[nb.cluster] // LR·RL 은 subgraph 제목이 순서 축(위)에 선다
			}
		}
	}
	return na.ro + nb.lo + gap
}

// weight 는 간선 조각의 무게다 — 긴 간선(더미끼리)을 곧게.
func (lg *lgraph) weight(a, b int) float64 {
	switch da, db := lg.isDummy(a), lg.isDummy(b); {
	case da && db:
		return 8
	case da || db:
		return 2
	}
	return 1
}

func (lg *lgraph) position() {
	for _, l := range lg.layers {
		x := 0.0
		for i, v := range l {
			if i == 0 {
				x = lg.n[v].lo
			} else {
				x += lg.sep(l[i-1], v)
			}
			lg.n[v].x = x
		}
	}
	for it := 0; it < 12; it++ {
		switch {
		case it >= 9: // 마지막은 위아래 이웃을 함께
			for r := range lg.layers {
				lg.place(r, true, true)
			}
		case it%2 == 0:
			for r := 1; r < len(lg.layers); r++ {
				lg.place(r, true, false)
			}
		default:
			for r := len(lg.layers) - 2; r >= 0; r-- {
				lg.place(r, false, true)
			}
		}
	}
	lg.unoverlapClusters()
}

// place 는 계층 r 의 점을 이웃(위·아래)의 가중 평균 쪽으로, 간격을 지키며 제곱 오차가 가장 작게 놓는다(가중 PAVA).
func (lg *lgraph) place(r int, up, down bool) {
	l := lg.layers[r]
	if len(l) == 0 {
		return
	}
	n := len(l)
	off := make([]float64, n)
	for i := 1; i < n; i++ {
		off[i] = off[i-1] + lg.sep(l[i-1], l[i])
	}
	type blk struct {
		sw, swt float64
		n       int
	}
	stack := make([]blk, 0, n)
	for i, v := range l {
		sw, swx := 0.0, 0.0
		acc := func(nb []int) {
			for _, u := range nb {
				w := lg.weight(v, u)
				sw += w
				swx += float64(w * lg.n[u].x)
			}
		}
		if up {
			acc(lg.n[v].ups)
		}
		if down {
			acc(lg.n[v].downs)
		}
		want := lg.n[v].x
		switch {
		case lg.n[v].self && i > 0: // 자기 간선 더미는 왼쪽 이웃(노드 또는 앞 고리의 더미)에 붙는다
			want, sw = lg.n[l[i-1]].x+(off[i]-off[i-1]), 4
		case sw > 0:
			want = swx / sw
		default:
			sw = 0.05 // 이웃 없는 점은 제자리를 약하게
		}
		b := blk{sw: sw, swt: float64(sw * (want - off[i])), n: 1}
		for len(stack) > 0 && stack[len(stack)-1].swt/stack[len(stack)-1].sw > b.swt/b.sw {
			t := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			b = blk{sw: b.sw + t.sw, swt: b.swt + t.swt, n: b.n + t.n}
		}
		stack = append(stack, b)
	}
	i := 0
	for _, b := range stack {
		y := b.swt / b.sw
		for k := 0; k < b.n; k++ {
			lg.n[l[i]].x = y + off[i]
			i++
		}
	}
}

// cbox 는 subgraph 상자의 순서 축 범위(여백 포함)다. 경계 더미는 넣지 않는다.
func (lg *lgraph) cbox(c int) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range lg.n {
		if v.cluster != c || v.border {
			continue
		}
		lo, hi, ok = math.Min(lo, v.x-v.lo), math.Max(hi, v.x+v.ro), true
	}
	if !ok {
		return 0, 0, false
	}
	lo -= clusterPad
	hi += clusterPad
	if !lg.vert {
		lo -= lg.titleHt[c]
	}
	if lg.vert {
		if tw := diagram.LinesW(lg.titles[c]) + 2*8; hi-lo < tw {
			m := (lo + hi) / 2
			lo, hi = m-tw/2, m+tw/2
		}
	}
	return lo, hi, true
}

// unoverlapClusters 는 subgraph 상자가 같은 계층의 다른 점·상자와 겹치지 않게 덩이(subgraph 통째, 또는 밖의 점 하나)를
// 오른쪽으로 민다. 덩이의 차례는 계층마다 같은 방향이라(subgraph 차례가 모든 계층에서 같다) 제약은 순환이 없다.
func (lg *lgraph) unoverlapClusters() {
	if lg.nc == 0 {
		return
	}
	unit := func(v int) int { // 덩이 번호: subgraph 는 0…nc-1, 밖의 점은 nc + v
		if c := lg.n[v].cluster; c >= 0 {
			return c
		}
		return lg.nc + v
	}
	nu := lg.nc + len(lg.n)
	lo := make([]float64, nu)
	hi := make([]float64, nu)
	valid := make([]bool, nu)
	for c := 0; c < lg.nc; c++ {
		lo[c], hi[c], valid[c] = lg.cbox(c)
	}
	for v := range lg.n {
		if lg.n[v].cluster < 0 {
			u := lg.nc + v
			lo[u], hi[u], valid[u] = lg.n[v].x-lg.n[v].lo, lg.n[v].x+lg.n[v].ro, true
		}
	}
	type con struct {
		to  int
		gap float64
	}
	succ := make([][]con, nu)
	indeg := make([]int, nu)
	seen := map[[2]int]bool{}
	for _, l := range lg.layers {
		for i := 1; i < len(l); i++ {
			a, b := unit(l[i-1]), unit(l[i])
			if a == b || !valid[a] || !valid[b] || seen[[2]int{a, b}] {
				continue
			}
			seen[[2]int{a, b}] = true
			gap := 10.0
			if a >= lg.nc && b >= lg.nc {
				gap = lg.sep(l[i-1], l[i]) - lg.n[l[i-1]].ro - lg.n[l[i]].lo
			}
			succ[a] = append(succ[a], con{b, gap})
			indeg[b]++
		}
	}
	shift := make([]float64, nu)
	var queue []int
	for u := 0; u < nu; u++ {
		if indeg[u] == 0 {
			queue = append(queue, u)
		}
	}
	for head := 0; head < len(queue); head++ {
		a := queue[head]
		for _, c := range succ[a] {
			if need := hi[a] + shift[a] + c.gap - lo[c.to]; need > shift[c.to] {
				shift[c.to] = need
			}
			if indeg[c.to]--; indeg[c.to] == 0 {
				queue = append(queue, c.to)
			}
		}
	}
	// 순환이 있으면(있으면 안 된다) 남은 덩이는 밀지 않는다 — 겹칠 수는 있어도 끝난다
	for v := range lg.n {
		lg.n[v].x += shift[unit(v)]
	}
}

// ---- 계층 축 좌표(순서 축과 무관하다)

// titleWrapW 는 세로 배치(TB·BT)에서 subgraph 제목의 줄 너비다 — 넘으면 줄을 바꾼다(상자가 끝없이 넓어지지 않게).
// 가로 배치(LR·RL)는 상자의 계층 축 길이에 맞춰 바꾼다 — 상자를 넓히면 옆 계층과 겹친다.
const titleWrapW = 264.0

// rankAxis 는 계층마다의 띠 높이·가운데 좌표, subgraph 의 계층 범위와 제목 줄을 정한다.
func (lg *lgraph) rankAxis() {
	g := lg.g
	nr := len(lg.layers)
	lg.band = make([]float64, nr)
	for _, v := range lg.n {
		lg.band[v.rank] = math.Max(lg.band[v.rank], v.hr)
	}
	lg.clo, lg.chi = make([]int, lg.nc), make([]int, lg.nc)
	for c := range lg.clo {
		lg.clo[c], lg.chi[c] = math.MaxInt, -1
	}
	for _, v := range lg.n {
		if v.cluster >= 0 && !v.border {
			lg.clo[v.cluster] = min(lg.clo[v.cluster], v.rank)
			lg.chi[v.cluster] = max(lg.chi[v.cluster], v.rank)
		}
	}
	lg.titles, lg.titleHt = make([][]string, lg.nc), make([]float64, lg.nc)
	setTitle := func(c int, width float64) {
		if t := g.Clusters[c].Title; t != "" {
			lg.titles[c] = diagram.WrapLabel(t, width)
			lg.titleHt[c] = float64(float64(len(lg.titles[c]))*lineH) + 4
		}
	}
	if lg.vert {
		for c := range lg.titles {
			setTitle(c, titleWrapW)
		}
	}
	startAt := make([]float64, nr+1) // 계층 r 앞에 더할 간격
	for c := 0; c < lg.nc; c++ {
		if lg.chi[c] < 0 {
			continue
		}
		top, bot := clusterPad, clusterPad
		switch g.Dir {
		case TB:
			top += lg.titleHt[c]
		case BT:
			bot += lg.titleHt[c]
		}
		startAt[lg.clo[c]] = math.Max(startAt[lg.clo[c]], top)
		startAt[lg.chi[c]+1] = math.Max(startAt[lg.chi[c]+1], bot)
	}
	lg.y = make([]float64, nr)
	for r := 0; r < nr; r++ {
		if r == 0 {
			lg.y[r] = startAt[0] + lg.band[0]/2
			continue
		}
		lg.y[r] = lg.y[r-1] + lg.band[r-1]/2 + lg.rankSep/2 + startAt[r] + lg.band[r]/2
	}
	if !lg.vert {
		for c := 0; c < lg.nc; c++ {
			if lg.chi[c] >= 0 {
				ext := lg.y[lg.chi[c]] + lg.band[lg.chi[c]]/2 - lg.y[lg.clo[c]] + lg.band[lg.clo[c]]/2 + float64(2*clusterPad)
				setTitle(c, math.Max(ext-16, 48))
			}
		}
	}
}

// straighten 은 한 직선 위의 가운데 점을 뺀다(긴 간선의 곧은 더미 줄) — 같은 점들의 B-스플라인은 같은 직선이라
// 모양은 그대로이고 경로 글자만 준다. 양 끝은 남긴다.
func straighten(p []Pt) []Pt {
	if len(p) < 3 {
		return p
	}
	out := []Pt{p[0]}
	for i := 1; i+1 < len(p); i++ {
		a, b, c := out[len(out)-1], p[i], p[i+1]
		cross := float64((b.X-a.X)*(c.Y-a.Y)) - float64((b.Y-a.Y)*(c.X-a.X))
		dot := float64((b.X-a.X)*(c.X-b.X)) + float64((b.Y-a.Y)*(c.Y-b.Y))
		if math.Abs(cross) <= 0.5*hyp(c.X-a.X, c.Y-a.Y) && dot >= 0 {
			continue // b 는 a–c 선분 위(0.5 안)
		}
		out = append(out, b)
	}
	return append(out, p[len(p)-1])
}

// ---- 마무리: 최종 좌표, 간선 점

func (lg *lgraph) finish(sizes [][2]float64, edgeLines [][]string) *Result {
	g := lg.g
	band, y, clo, chi := lg.band, lg.y, lg.clo, lg.chi
	toFinal := func(x, yy float64) Pt {
		switch g.Dir {
		case BT:
			return Pt{x, -yy}
		case LR:
			return Pt{yy, x}
		case RL:
			return Pt{-yy, x}
		}
		return Pt{x, yy}
	}
	out := &Result{Nodes: make([]Box, len(g.Nodes)), Edges: make([]EdgePath, len(g.Edges)), Clusters: make([]ClusterBox, lg.nc)}
	for v := range g.Nodes {
		ln := &lg.n[v]
		out.Nodes[v] = Box{C: toFinal(ln.x, y[ln.rank]), W: sizes[v][0], H: sizes[v][1]}
	}
	for c := 0; c < lg.nc; c++ {
		lo, hi, ok := lg.cbox(c)
		if !ok {
			continue
		}
		r0 := y[clo[c]] - band[clo[c]]/2 - clusterPad
		r1 := y[chi[c]] + band[chi[c]]/2 + clusterPad
		switch g.Dir {
		case TB:
			r0 -= lg.titleHt[c]
		case BT:
			r1 += lg.titleHt[c]
		}
		a, b := toFinal(lo, r0), toFinal(hi, r1)
		out.Clusters[c] = ClusterBox{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y), W: math.Abs(a.X - b.X), H: math.Abs(a.Y - b.Y),
			Title: lg.titles[c], Show: true}
	}
	for i, e := range g.Edges {
		ep := EdgePath{Label: edgeLines[i]}
		if len(ep.Label) > 0 {
			ep.LW, ep.LH = LabelSize(ep.Label)
		}
		if e.From == e.To {
			if e.Hidden {
				continue
			}
			// 고리: 노드 → 더미 위 → 더미 가운데(라벨) → 더미 아래 → 노드 (dagre positionSelfEdges 와 같은 다섯 점)
			nv, d := &lg.n[e.From], &lg.n[lg.selfDummy[i]]
			yy, dx, dy := y[nv.rank], d.x-nv.x, d.loopDy
			pts := []Pt{{}, toFinal(nv.x+dx*2/3, yy-dy), toFinal(nv.x+dx*5/6, yy-dy), toFinal(d.x, yy),
				toFinal(nv.x+dx*5/6, yy+dy), toFinal(nv.x+dx*2/3, yy+dy), {}}
			n := out.Nodes[e.From]
			pts[0] = lg.clip(e.From, n, pts[1])
			pts[6] = lg.clip(e.From, n, pts[5])
			ep.Self, ep.Pts, ep.LC = true, pts, pts[3]
			out.Edges[i] = ep
			continue
		}
		chain := lg.chains[i]
		pts := make([]Pt, len(chain))
		for k, v := range chain {
			ln := &lg.n[v]
			pts[k] = toFinal(ln.x, y[ln.rank])
			if ln.label {
				ep.LC = pts[k]
			}
		}
		if lg.rev[i] {
			for a, b := 0, len(pts)-1; a < b; a, b = a+1, b-1 {
				pts[a], pts[b] = pts[b], pts[a]
			}
		}
		pts[0] = lg.clip(e.From, out.Nodes[e.From], pts[1])
		pts[len(pts)-1] = lg.clip(e.To, out.Nodes[e.To], pts[len(pts)-2])
		ep.Pts = straighten(pts)
		out.Edges[i] = ep
	}
	normalize(out)
	return out
}

func (lg *lgraph) clip(i int, b Box, to Pt) Pt {
	if lg.g.Clip != nil {
		return lg.g.Clip(i, b, to)
	}
	return ClipRect(b, to)
}

// ClipRect 는 상자 가운데에서 to 쪽으로 간 선이 사각형 테두리와 만나는 점이다.
func ClipRect(b Box, to Pt) Pt {
	c := b.C
	dx, dy := to.X-c.X, to.Y-c.Y
	if dx == 0 && dy == 0 {
		return c
	}
	t := RectT(b.W/2, b.H/2, dx, dy)
	return Pt{c.X + float64(dx*t), c.Y + float64(dy*t)}
}

// RectT 는 가운데에서 (dx,dy) 방향의 반직선이 반 너비 hw·반 높이 hh 사각형 테두리와 만나는 t 다(둘 다 0 이면 +Inf).
func RectT(hw, hh, dx, dy float64) float64 {
	t := math.Inf(1)
	if dx != 0 {
		t = math.Min(t, hw/math.Abs(dx))
	}
	if dy != 0 {
		t = math.Min(t, hh/math.Abs(dy))
	}
	return t
}

// normalize 는 모든 것이 margin 부터 시작하게 옮기고 전체 크기를 정한다.
func normalize(l *Result) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	addR := func(x0, y0, x1, y1 float64) {
		minX, minY = math.Min(minX, x0), math.Min(minY, y0)
		maxX, maxY = math.Max(maxX, x1), math.Max(maxY, y1)
	}
	for _, n := range l.Nodes {
		addR(n.C.X-n.W/2, n.C.Y-n.H/2, n.C.X+n.W/2, n.C.Y+n.H/2)
	}
	for _, c := range l.Clusters {
		if c.Show {
			addR(c.X, c.Y, c.X+c.W, c.Y+c.H)
		}
	}
	for _, e := range l.Edges {
		for _, p := range e.Pts {
			addR(p.X-5, p.Y-5, p.X+5, p.Y+5)
		}
		if len(e.Label) > 0 {
			addR(e.LC.X-e.LW/2, e.LC.Y-e.LH/2, e.LC.X+e.LW/2, e.LC.Y+e.LH/2)
		}
	}
	dx, dy := margin-minX, margin-minY
	for i := range l.Nodes {
		l.Nodes[i].C.X += dx
		l.Nodes[i].C.Y += dy
	}
	for i := range l.Clusters {
		l.Clusters[i].X += dx
		l.Clusters[i].Y += dy
	}
	for i := range l.Edges {
		e := &l.Edges[i]
		for k := range e.Pts {
			e.Pts[k].X += dx
			e.Pts[k].Y += dy
		}
		e.LC.X += dx
		e.LC.Y += dy
	}
	l.W = math.Ceil(maxX - minX + 2*margin)
	l.H = math.Ceil(maxY - minY + 2*margin)
}

func p2(p Pt) string { return diagram.F1(p.X) + " " + diagram.F1(p.Y) }

// Basis 는 점들을 지나는 B-스플라인(d3 curveBasis — Mermaid 흐름도·ER 의 기본 곡선)의 SVG 경로다. 두 점이면 직선.
func Basis(p []Pt) string {
	var b strings.Builder
	b.WriteString("M" + p2(p[0]))
	if len(p) == 2 {
		b.WriteString(" L" + p2(p[1]))
		return b.String()
	}
	x0, x1 := p[0], p[1]
	bez := func(x Pt) {
		b.WriteString(" C" + p2(Pt{(float64(2*x0.X) + x1.X) / 3, (float64(2*x0.Y) + x1.Y) / 3}) + " " +
			p2(Pt{(x0.X + float64(2*x1.X)) / 3, (x0.Y + float64(2*x1.Y)) / 3}) + " " +
			p2(Pt{(x0.X + float64(4*x1.X) + x.X) / 6, (x0.Y + float64(4*x1.Y) + x.Y) / 6}))
	}
	b.WriteString(" L" + p2(Pt{(float64(5*x0.X) + x1.X) / 6, (float64(5*x0.Y) + x1.Y) / 6}))
	for _, x := range p[2:] {
		bez(x)
		x0, x1 = x1, x
	}
	bez(x1)
	b.WriteString(" L" + p2(x1))
	return b.String()
}

// hyp 는 √(x²+y²) 다 — 곱셈-덧셈을 명시적 float64 변환으로 나눠 FMA 합치기를 막는다(flowchart 의 hyp 와 같다).
func hyp(x, y float64) float64 { return math.Sqrt(float64(x*x) + float64(y*y)) }
