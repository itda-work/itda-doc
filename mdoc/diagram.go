package mdoc

import (
	"container/list"
	"fmt"
	"strings"
	"sync"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/classdiag"
	"github.com/itda-work/itda-doc/diagram/er"
	"github.com/itda-work/itda-doc/diagram/flowchart"
	"github.com/itda-work/itda-doc/diagram/gantt"
	"github.com/itda-work/itda-doc/diagram/sequence"
	statediagram "github.com/itda-work/itda-doc/diagram/state"
	"github.com/itda-work/itda-doc/diagram/timeline"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// 그림(1.10, #117·#409). ```mermaid 코드 블록을 인라인 SVG 로 그린다 — 종류(frontmatter·주석·빈 줄을 건너뛴 첫 낱말)로
// 갈라 흐름도는 flowchart 패키지, 간트 차트는 gantt 패키지 …(renderDiagram)가 그린다(공개 꼴·공통 부분은 internal/diagram).
// 다른 뷰어(GitHub 등)에서는 mermaid 코드 블록 그대로라 같은 원문이 그림이 된다.
// 그리지 못하면(다른 그림 종류·미지원 문법·상한) 보통의 코드 블록으로 두고 줄 번호 경고를 남긴다(원문이 보인다).
// 모양을 문서에 적는 줄(style·classDef·%%{init}%% …)과 click 은 쓰지 않고 그림은 그린다 — 종류마다 첫 줄에 경고 한 번(frontmatter 설정은 키마다).

// 문서 하나의 상한. 넘는 그림은 코드 블록으로 둔다(경고 한 번). 그림 하나의 상한은 flowchart 의 Max* 다.
const (
	MaxDiagramsPerDoc     = 50      // 그림 수
	MaxDiagramBytesPerDoc = 4 << 20 // 그린 SVG 의 합(바이트)
)

// diagramCache 는 그림 원문 → 결과다. 같은 입력은 같은 출력이라 문서 사이에 나눠도 상태가 새지 않는다 —
// 편집 중 다시 렌더할 때 바뀐 그림만 배치한다. 개수·바이트 상한을 넘으면 오래 쓰지 않은 것부터 버린다.
var diagramCache = newLRU(512, 16<<20)

type diagramOut struct {
	res diagram.Result
	err error
}

// diagram 은 ```mermaid 블록 하나를 그린다. 그리지 못하면 경고를 남기고 거짓이다(부르는 쪽이 코드 블록으로 쓴다).
func (st *state) diagram(fc *ast.FencedCodeBlock, src []byte) (string, bool) {
	fence := st.lines.line(fc.Pos())
	st.diagramCount++
	if st.diagramFull {
		return "", false
	}
	if st.diagramCount > MaxDiagramsPerDoc {
		if st.diagramCount == MaxDiagramsPerDoc+1 {
			st.warn(fence, WarnDiagram, fmt.Sprintf("그림이 %d 개를 넘어 뒤의 mermaid 블록은 코드로 둠", MaxDiagramsPerDoc))
		}
		return "", false
	}
	// 원문 줄 → 문서 줄. 인용·목록 안의 블록도 줄마다 위치에서 센다
	var b strings.Builder
	lines := fc.Lines()
	docLine := make([]int, 0, lines.Len())
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
		docLine = append(docLine, st.lines.line(seg.Start))
	}
	at := func(l int) int { // flowchart 의 줄(1부터, 0 = 모름) → 문서 줄
		if l >= 1 && l <= len(docLine) {
			return docLine[l-1]
		}
		return fence
	}
	text := b.String()
	var out diagramOut
	if v, ok := diagramCache.get(text); ok {
		out = v.(diagramOut)
	} else {
		r, err := renderDiagram(text)
		out = diagramOut{res: r, err: err}
		diagramCache.put(text, out, len(text)+len(r.SVG))
	}
	if out.err != nil {
		line := fence
		msg := out.err.Error()
		if fe, ok := out.err.(*diagram.Error); ok {
			line, msg = at(fe.Line), fe.Msg
		}
		st.warn(line, WarnDiagram, "mermaid 그림을 그리지 않고 원문 그대로 둠: "+msg)
		return "", false
	}
	if st.diagramBytes += len(out.res.SVG); st.diagramBytes > MaxDiagramBytesPerDoc {
		if !st.diagramFull {
			st.diagramFull = true
			st.warn(fence, WarnDiagram, fmt.Sprintf("그림의 합이 %dMB 를 넘어 이 뒤의 mermaid 블록은 코드로 둠", MaxDiagramBytesPerDoc>>20))
		}
		return "", false
	}
	for _, is := range out.res.Issues {
		st.warn(at(is.Line), WarnDiagram, "mermaid 그림: "+is.Msg)
	}
	st.diagramPics[fc] = DiagramPic{SVG: out.res.SVG, Caption: out.res.Caption}
	var w strings.Builder
	w.WriteString(out.res.SVG)
	if out.res.Caption != "" {
		w.WriteString("\n<figcaption>")
		w.Write(util.EscapeHTML([]byte(out.res.Caption)))
		w.WriteString("</figcaption>")
	}
	return w.String(), true
}

// renderDiagram 은 그림 종류로 갈라 그린다 — 간트 차트(gantt)는 internal/gantt(#409 ①), 타임라인(timeline)은
// internal/timeline(#409 ②), 순서도(sequenceDiagram)는 internal/sequence(#409 ③ — timeline·sequence 의 머리 낱말은
// 원본 lexer 처럼 대소문자를 가리지 않는다. 흐름도·gantt 의 머리 판정은 대소문자를 가리는 지금 규칙 그대로다),
// ER 다이어그램(erDiagram)은 internal/er(#409 ④ — 원본 detector /^\s*erDiagram/ 처럼 대소문자를 가린다), 상태도
// (stateDiagram·stateDiagram-v2)는 internal/state(#409 ④-2 — 원본 detector /^\s*stateDiagram/ 처럼 앞머리·대소문자를 가린다.
// 머리가 정확히 그 둘이 아니면 state 가 거부한다), 클래스도(classDiagram·classDiagram-v2)는 internal/classdiag(#409 ④-3 —
// 같은 규칙, 머리가 정확히 그 둘이 아니면 classdiag 가 거부한다), 나머지(흐름도와
// 그리지 않는 종류의 경고)는 internal/flowchart 가 맡는다. 공개 꼴은 같다(internal/diagram).
func renderDiagram(src string) (diagram.Result, error) {
	kind := diagram.Kind(src)
	switch {
	case kind == "gantt":
		return gantt.Render(src)
	case strings.EqualFold(kind, "timeline"):
		return timeline.Render(src)
	case strings.EqualFold(kind, "sequenceDiagram"):
		return sequence.Render(src)
	case kind == "erDiagram":
		return er.Render(src)
	case strings.HasPrefix(kind, "stateDiagram"):
		return statediagram.Render(src)
	case strings.HasPrefix(kind, "classDiagram"):
		return classdiag.Render(src)
	}
	return flowchart.Render(src)
}

// ---- 결과 기억(LRU)

type lru struct {
	mu       sync.Mutex
	max      int
	maxBytes int
	bytes    int
	ll       *list.List
	m        map[string]*list.Element
}

type lruEntry struct {
	key  string
	val  any
	size int
}

func newLRU(max, maxBytes int) *lru {
	return &lru{max: max, maxBytes: maxBytes, ll: list.New(), m: map[string]*list.Element{}}
}

func (c *lru) get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[k]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(*lruEntry).val, true
	}
	return nil, false
}

func (c *lru) put(k string, v any, size int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if size > c.maxBytes/8 {
		return // 너무 큰 것은 기억하지 않는다
	}
	if e, ok := c.m[k]; ok {
		c.ll.MoveToFront(e)
		return
	}
	c.m[k] = c.ll.PushFront(&lruEntry{key: k, val: v, size: size})
	c.bytes += size
	for c.ll.Len() > c.max || c.bytes > c.maxBytes {
		e := c.ll.Back()
		en := e.Value.(*lruEntry)
		c.ll.Remove(e)
		delete(c.m, en.key)
		c.bytes -= en.size
	}
}
