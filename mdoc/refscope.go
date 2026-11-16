package mdoc

import (
	"slices"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 노트 안 참조 정의의 범위(itda-doc#89).
//
// goldmark 은 링크·그림 참조 정의(`[이름]: 주소 "제목"`)를 블록 파싱 중 문단이 닫힐 때(LinkReferenceParagraphTransformer, 100) 문서 전역
// parser.Context 에 넣고, 인라인 파싱 때 `]` 에서 pc.Reference 로 찾는다. 그래서 :::notes 안의 정의가 노트 밖 참조에 쓰여 그 제목이 본문
// 그림 캡션(transform.go Figure.Caption)·링크 title 로 옮겨 가고, 파싱 뒤에 노트 자식을 지우는 DropNotes(transform.go notes)로는 이미 풀린
// 글을 지우지 못한다.
//
// 그래서 Context 를 감싸(refScopeContext) 정의마다 출처 — 그 정의 문단의 가장 가까운 노트 컨테이너(없으면 문서) — 를 남기고, 참조를 찾을 때
// 참조 자리에서 보이는 정의만 쓴다. 노트 안 정의는 그 노트(안쪽 노트 포함) 안의 참조에만 보이고, 노트 밖 정의는 어디서나 보인다. 이름이 같은
// 정의가 여럿이면 보이는 것 가운데 먼저 적은 것이다(CommonMark 의 「처음 것」 을 보이는 것 안에서). 노트 밖 참조가 노트 안 정의에만 맞으면
// 정의가 없을 때처럼 글자로 두고 경고(WarnNoteRefScope)한다. 노트를 싣든 빼든 같은 판정이다 — 노트를 싣는 렌더와 빼는 렌더의 본문이 같다.
//
// 노트 영역을 따로 파싱하는 길 대신 이 길을 고른 까닭: 노트는 ::: 컨테이너라 목록·인용 안에도 열리고 중첩되며, 따로 파싱하면 줄 번호·경고·
// 중첩 상한·passedBy 같은 컨테이너 파서 상태(container.go)를 두 번 맞춰야 한다. 출처 표시는 goldmark 의 공개 확장점(WithContext·
// 문단 변환기·인라인 파서)만 쓰고 파싱 결과의 모양을 바꾸지 않는다.

// refScopeContext 는 참조 정의에 출처를 다는 parser.Context 다(parseGuarded 가 파싱마다 새로 만든다).
type refScopeContext struct {
	parser.Context
	st   *state
	defs map[string][]scopedRef // 정규 키(util.ToLinkReference) → 적은 차례의 정의
	// defScope 는 지금 정의를 읽는 문단의 가장 가까운 노트다(refScopeMarker 가 LinkReferenceParagraphTransformer 바로 앞에서 정한다)
	defScope ast.Node
	// useNode·useLine 은 지금 인라인 파싱 중인 `]` 의 블록과 줄이다(refScopeProbe 가 링크 파서 바로 앞에서 정한다)
	useNode ast.Node
	useLine int
	warned  map[refWarnKey]bool
}

type scopedRef struct {
	ref   parser.Reference
	scope ast.Node // 정의가 든 가장 가까운 노트 컨테이너. nil 이면 문서(어디서나 보인다)
}

type refWarnKey struct {
	line  int
	label string
}

func newRefScopeContext(st *state) *refScopeContext {
	return &refScopeContext{Context: parser.NewContext(), st: st, defs: map[string][]scopedRef{}, warned: map[refWarnKey]bool{}}
}

// AddReference 는 정의를 출처와 함께 적는다. 감싼 Context 에도 넣는다(References 가 쓰는 목록 — 찾기는 아래 Reference 가 한다).
func (c *refScopeContext) AddReference(ref parser.Reference) {
	key := util.ToLinkReference(ref.Label())
	c.defs[key] = append(c.defs[key], scopedRef{ref: ref, scope: c.defScope})
	c.Context.AddReference(ref)
}

// Reference 는 지금 참조 자리(useNode)에서 보이는 정의 가운데 먼저 적은 것이다. label 은 goldmark 이 정규화한 키다(link.go).
func (c *refScopeContext) Reference(label string) (parser.Reference, bool) {
	list := c.defs[label]
	for _, d := range list {
		if d.scope == nil || isAncestorOrSelf(d.scope, c.useNode) {
			return d.ref, true
		}
	}
	if len(list) > 0 {
		k := refWarnKey{c.useLine, label}
		if !c.warned[k] {
			c.warned[k] = true
			c.st.warn(c.useLine, WarnNoteRefScope, "참조 ["+string(list[0].ref.Label())+"] 의 정의가 발표자 노트 안에만 있어 쓰지 않음 — 노트 안 정의는 그 노트 안에서만 씁니다(본문에서 쓰려면 정의를 노트 밖에)")
		}
	}
	return nil, false
}

// isAncestorOrSelf 는 a 가 n 이거나 n 의 조상인가다.
func isAncestorOrSelf(a, n ast.Node) bool {
	for p := n; p != nil; p = p.Parent() {
		if p == a {
			return true
		}
	}
	return false
}

// nearestNotes 는 n 의 조상(자신 포함) 가운데 가장 가까운 노트 컨테이너다 — 판정은 transform.go notes·headings.go inNotes 와 같은 classes() 의
// notes 다(파싱 중이라 이름이 아직 정리되기 전이어도 classes() 는 같다).
func nearestNotes(n ast.Node) ast.Node {
	for p := n; p != nil; p = p.Parent() {
		if c, ok := p.(*Container); ok && slices.Contains(c.classes(), "notes") {
			return c
		}
	}
	return nil
}

// refScopeMarker 는 LinkReferenceParagraphTransformer(100) 바로 앞에서 그 문단의 노트를 적어 두는 문단 변환기다. 문단은 바꾸지 않는다.
type refScopeMarker struct{}

func (refScopeMarker) Transform(node *ast.Paragraph, _ text.Reader, pc parser.Context) {
	if c, ok := pc.(*refScopeContext); ok {
		c.defScope = nearestNotes(node)
	}
}

// refScopeProbe 는 링크 파서(200)·linkCloserGuard(199)보다 앞에서 `]` 의 블록과 줄을 적어 두는 인라인 파서다. 아무것도 읽지 않는다(nil).
type refScopeProbe struct{ st *state }

func (refScopeProbe) Trigger() []byte { return []byte{']'} }

func (p refScopeProbe) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	if c, ok := pc.(*refScopeContext); ok {
		_, seg := block.PeekLine()
		c.useNode, c.useLine = parent, p.st.lines.line(seg.Start)
	}
	return nil
}
