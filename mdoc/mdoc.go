// Package mdoc 는 마크다운 원문을 문서 HTML 로 바꾼다.
//
// 문법의 정본은 안내서(입구 spec/markdown.md 와 주제별 안내서 — 표는 시험이 이 패키지와 묶는다), 결정·까닭은
// https://github.com/itda-work/itda-hyve/blob/main/docs/DESIGN-markdown-docs.md 1장이다. 이 패키지는 창·파일·네트워크를 모른다.
// 원문을 받아 문서 정보(Meta)·본문 HTML·목차·경고를 돌려주는 순수 함수 하나다.
// 같은 입력은 같은 출력이다 — testdata 의 golden 파일로 고정한다.
//
// 표지·머리글·목차 자리·테마는 다음 단계(문서 창)의 일이다. 여기서는 뜻만 HTML 로 옮긴다.
//
// 보안: raw HTML 은 허용 목록 몇 개 말고는 내보내지 않는다(1.9). goldmark 의 WithUnsafe 를 켜지 않고,
// 사용자가 적은 속성은 data-* 로만 나간다. style 이 나가는 곳은 길이 형식을 검사한 width 뿐이다.
package mdoc

import (
	"bytes"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// Options 는 렌더 동작을 바꾸는 값이다. 0 값이 안전한 기본이다.
type Options struct {
	// AllowRemoteImages 가 참이면 http(s) 이미지의 src 를 그대로 낸다.
	// 기본은 거짓이다 — 주소를 data-remote-src 로 옮겨 표시만 남긴다(메일에서 온 문서의 추적 픽셀을 막는다).
	AllowRemoteImages bool
	// LocalImagePrefix 는 상대 경로 이미지의 src 앞에 붙인다(예: "/_doc/"). 문서 창은 앱 화면과 같은 출처에서
	// 문서 폴더의 그림을 따로 내주므로 주소 공간을 나눠야 한다. 비면 원문 그대로다. data: 이미지에는 붙이지 않는다.
	LocalImagePrefix string
	// DropNotes 가 참이면 발표자 노트 — `:::notes` 와 `{.notes}` 클래스의 컨테이너 — 의 안에 든 것(중첩 포함)을 결과에서 빼고
	// 빈 `<div class="notes">` 만 그 자리에 남긴다(맨 위 블록 구조가 호스트와 같다). 노트의 다른 class·id·속성은 이 값과 상관없이
	// 경고와 함께 버리므로 노트 자리의 태그·속성은 호스트와 같다(transform.go notes). 제목의 `{.notes}` 는 노트가 아니다 —
	// 이 값과 상관없이 경고와 함께 class 를 버린다(headings.go dropHeadingNotes). 빼기는 다른 변환 전에 AST 에서 하므로 노트 안
	// 제목은 TOC·번호에도, 노트 안 그림·링크는 경고에도 남지 않는다. 노트에서만 닿는 각주도 빠지고(목록이 비면 빈 목록 자리만),
	// 공개 본문에서 (각주를 거쳐) 닿는 각주는 남는다(transform.go notes·pruneFootnotes).
	// 공유 화면이 「노트 함께 보이기」 를 끈 동안 쓴다(#344 결정 7). 기본은 거짓 — 노트를 그대로 낸다.
	DropNotes bool
	// FolderMeta 는 문서 위 폴더의 `_metadata.yml` 이다(#474, inherit.go — 먼 폴더부터 가까운 순).
	// 문서 창이 경로를 알아 읽어 넘긴다 — Render 는 경로를 모르는 순수 함수 그대로다. 이 값을 문서
	// frontmatter 의 기본값으로 합친다(문서가 이긴다). 비면 상속이 없다(붙여넣은 원문 등).
	FolderMeta []FolderMeta
}

// Result 는 렌더 결과다.
type Result struct {
	EditorTokens []EditorCodeBlock // 편집기 전용 토큰; HTML·TOC 와 별개

	Meta Meta
	// HTML 은 본문이다. 표지·머리글·바닥글은 넣지 않는다.
	HTML string
	// TOC 는 목차에 들어갈 제목 트리다. toc 설정과 무관하게 늘 채운다(toc-depth·.unlisted 적용).
	TOC []TOCItem
	// TOCPlaced 는 본문의 [TOC] 자리에 목차를 이미 넣었는지다.
	// 거짓인데 Meta.TOC 가 참이면 목차 자리는 다음 단계가 정한다(RenderTOC 로 그린다).
	TOCPlaced bool
	Warnings  []Warning
	// SlideLevel 은 발표에서 슬라이드를 여는 제목 단계다(Pandoc 규칙: 바로 아래에 제목이 아닌 내용이 오는 가장 높은 단계).
	// 제목이 없으면 0. frontmatter slides.level 이 있으면 화면이 그것을 쓴다. DropNotes 와 상관없이 같은 값이다 — 뺀 노트는 빈
	// 컨테이너로 자리에 남는다(transform.go notes).
	SlideLevel int
	// FolderMeta 는 이 문서에 적용된 폴더 `_metadata.yml` 의 문서 기준 상대 경로다(#474 — 가까운 것부터).
	// 문서 창이 출처 표시에 쓴다. 깨져서 건너뛴 파일은 여기 없고 경고로만 알린다.
	FolderMeta []string
}

// Warning 은 렌더하면서 버리거나 고친 것 하나다. 편집기가 "렌더하지 않은 HTML n곳" 처럼 모아 보여 준다.
type Warning struct {
	Line    int    // 원문 줄 번호(1부터). 모르면 0
	Code    string // 아래 Warn* 상수
	Message string
	// Fix 는 이 경고를 없애는 원문 바꿈이다(있을 때만 — indented-close, indentclose.go). 화면이 확인을 받아 편집 버퍼에 넣는다.
	Fix *Fix `json:",omitempty"`
	// Key 는 경고가 말하는 정규 키 경로다("theme"·"slides.font-scale" — frontmatter 값 경고에만 쓴다).
	// 편집 화면(#474 ②)의 내부 보조 정보다 — ① 의 경고 JSON 은 34cd175 와 같은 모양 그대로다(직렬화에서 뺀다).
	Key string `json:"-"`
}

// 경고 코드. 화면은 코드로 묶고, 문장은 사람에게 보여 준다.
const (
	WarnRawHTML            = "raw-html"             // 렌더하지 않은 HTML
	WarnFrontmatterInvalid = "frontmatter-invalid"  // YAML 이 깨져 본문으로 렌더함
	WarnFrontmatterValue   = "frontmatter-value"    // 아는 키의 값 형식이 틀려 무시함
	WarnUnsafeURL          = "unsafe-url"           // 막은 링크·이미지 주소
	WarnRemoteImage        = "remote-image"         // 불러오지 않은 원격 이미지
	WarnBadAttribute       = "bad-attribute"        // 알아보지 못한 {…} 속성
	WarnUnclosedContainer  = "unclosed-container"   // 문서 끝에서 닫힌 ::: 컨테이너
	WarnDuplicateID        = "duplicate-id"         // 직접 적은 id 가 겹침
	WarnTooComplex         = "too-complex"          // 너무 많은 강조·링크 표시를 글자로 둠, 또는 블록 중첩이 너무 깊어 문서 전체를 원문 글자로 보임(렌더 시간 상한 — DropNotes 면 원문 대신 안내만, nesting.go withheldRaw)
	WarnMath               = "math"                 // 그리지 않은 수식(원문 그대로) — 미지원·막은 명령·문법 오류·상한·닫히지 않은 $$. 또는 그린 수식에서 TeX 주석(%)이 지운 글(percentLoss, #180)
	WarnDiagram            = "diagram"              // 그리지 않은 mermaid 그림(코드 그대로) 또는 그림에서 쓰지 않은 줄(style·click …)
	WarnTildeRange         = "tilde-range"          // 범위로 보이는 단일 ~ 가 취소선이 됨(3~5명 … 10~20명) — 렌더는 그대로(#140)
	WarnDateList           = "date-list"            // 줄 머리 날짜(2026. 10. 14.)가 번호 목록이 됨 — 렌더는 그대로(#140)
	WarnListMarker         = "list-marker"          // 목록 항목에 기호(□ ○)·항목 번호(가. (1))를 글자로 적음 — list-style 로(#141)
	WarnIndentedClose      = "indented-close"       // 닫히지 않은 ::: 안의 4칸 이상 들여쓴 닫는 줄 — 그 컨테이너의 unclosed-container 대신, Fix 를 싣는다(indentclose.go, #494)
	WarnFolderSlidesUnused = "folder-slides-unused" // 폴더 _metadata.yml 의 slides 설정을 문서에 slides: 가 없어 쓰지 않음 — 렌더는 그대로(inherit.go folderSlidesUnused, #543)
)

// Render 는 마크다운 원문을 렌더한다. 오류는 내부 렌더 실패뿐이다 — 문법 문제는 Warnings 로 알린다.
func Render(src []byte, opts Options) (Result, error) {
	d, err := parseLimit(src, opts, MaxNestingDepth)
	if err != nil {
		return Result{}, err
	}
	return d.Result, nil
}

// renderLimit 은 블록 중첩 상한 limit 으로 렌더한다(시험은 상한 없이 견준다).
func renderLimit(src []byte, opts Options, limit int) (Result, error) {
	d, err := parseLimit(src, opts, limit)
	if err != nil {
		return Result{}, err
	}
	return d.Result, nil
}

// parseLimit 은 Parse 의 몸이다 — 파싱·변환 뒤 HTML 까지 그려 Result 를 채우고, 변환된 AST 와 그리며 정한 노드 정보를 함께 쥔다(doc.go).
func parseLimit(src []byte, opts Options, limit int) (*Doc, error) {
	src = normalize(src)
	meta, body, warns, folder := parseFrontmatter(src, opts.FolderMeta)
	st := newState(body, meta, opts)
	st.nestingLimit = limit
	st.warns = append(st.warns, warns...)

	// 블록 중첩이 상한을 넘으면 파싱을 멈추고 문서 전체를 원문 글자로 — 멈춘 파싱의 AST·경고는 버린다(nesting.go)
	doc, over := st.parseGuarded(body)
	if over > 0 {
		// 원문 글자에서는 노트를 뺄 수 없다 — 노트를 빼야 하는 렌더(공유)에는 원문 대신 안내만 낸다(nesting.go withheldRaw, #344)
		var r Result
		if opts.DropNotes {
			r = withheldRaw(meta, warns, over)
		} else {
			r = renderRaw(meta, body, warns, over, frontmatterLines(src, meta))
		}
		r.FolderMeta = folder
		return &Doc{Result: r, body: body, rawSkip: frontmatterLines(src, meta), withheld: opts.DropNotes}, nil
	}
	st.transform(doc.(*ast.Document))
	if _, end, ok := splitFrontmatter(src); ok && meta.Present {
		// 본문 파싱에는 빈 줄로 대체되므로 원문 머리말 범위는 여기서 더한다.
		n := bytes.Count(src[:end], []byte("\n"))
		if end > 0 && src[end-1] != '\n' {
			n++
		}
		h := fnv.New32a()
		_, _ = h.Write(src[:end])
		st.editorTokens = append(st.editorTokens, EditorCodeBlock{Kind: "front", Line: 1, End: n, Hash: h.Sum32()})
	}

	var buf bytes.Buffer
	if err := st.md.Renderer().Render(&buf, body, doc); err != nil {
		return nil, err
	}

	sort.SliceStable(st.warns, func(i, j int) bool { return st.warns[i].Line < st.warns[j].Line })
	return &Doc{Result: Result{
		Meta:         meta,
		EditorTokens: st.editorTokens,
		HTML:         buf.String(),
		TOC:          st.toc,
		TOCPlaced:    st.tocPlaced,
		Warnings:     st.warns,
		SlideLevel:   slideLevel(doc),
		FolderMeta:   folder,
	}, root: doc.(*ast.Document), body: body, st: st}, nil
}

// normalize 는 BOM 을 떼고 줄 끝을 \n 하나로 맞춘다. 줄 수는 바뀌지 않는다 — data-line 이 원문 줄을 가리켜야 한다.
func normalize(src []byte) []byte {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	if bytes.IndexByte(src, '\r') < 0 {
		return src
	}
	s := strings.ReplaceAll(string(src), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return []byte(s)
}

// lineIndex 는 바이트 위치를 줄 번호로 바꾼다.
type lineIndex []int

func newLineIndex(src []byte) lineIndex {
	idx := lineIndex{0}
	for i, b := range src {
		if b == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

// line 은 1부터 센 줄 번호다. 위치를 모르면(음수) 0 이다.
func (l lineIndex) line(pos int) int {
	if pos < 0 {
		return 0
	}
	return sort.Search(len(l), func(i int) bool { return l[i] > pos })
}

// slideLevel 은 Pandoc 의 슬라이드 단계다: 문서 맨 위 블록 가운데, 바로 다음 블록이 제목·쪽 나눔·가로줄이 아닌 내용인
// 제목의 가장 높은(숫자가 작은) 단계. 그런 제목이 없으면 가장 높은 제목 단계, 제목이 없으면 0.
func slideLevel(doc ast.Node) int {
	best, top := 0, 0
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok {
			continue
		}
		if top == 0 || h.Level < top {
			top = h.Level
		}
		switch c.NextSibling().(type) {
		case nil, *ast.Heading, *PageBreak, *ast.ThematicBreak, *TOCNav:
			continue
		}
		if best == 0 || h.Level < best {
			best = h.Level
		}
	}
	if best == 0 {
		return top
	}
	return best
}
