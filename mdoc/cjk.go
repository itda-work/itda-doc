package mdoc

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// CJK-friendly 강조(설계 문서 「열린 문제」의 한글 강조 경계).
//
// CommonMark 는 닫는 ** 앞이 문장부호이고 뒤가 글자면 닫지 않는다. 그래서 **"인용"**은 · **결론:**입니다 가
// 굵게 되지 않는다. 한국어 문서(특히 AI 가 쓴 문서)에 아주 흔한 모양이라 CommonMark CJK-friendly amendments 를 따른다.
//
//	명세: https://github.com/tats-u/markdown-cjk-friendly/blob/main/specification.md
//	참조 구현: 같은 저장소의 micromark-extension-cjk-friendly(강조), …-gfm-strikethrough(취소선)
//
// 규칙을 새로 만들지 않는다. 바뀐 것은 left/right-flanking 의 정의 둘과 _ 의 "문장부호" 가 "문장부호 시퀀스" 가 된 것뿐이고,
// CJK 글자가 끼지 않은 입력에서는 goldmark 기본과 결과가 같다(TestCommonMarkSpecUnchanged).
// goldmark 의 ScanDelimiter 는 판정식을 바꿀 수 없어, 같은 일을 하는 파서를 우선순위를 높여 앞에 끼운다.

// 앞뒤 글자의 분류. 참조 구현의 classifyCharacter 와 같은 비트다.
type charClass uint8

const (
	clsSpace charClass = 1 << iota
	clsPunct
	clsCJK
	clsIVS // Ideographic Variation Selector U+E0100–U+E01EF
	clsSVS // Non-emoji General-use Variation Selector U+FE00–U+FE0E
)

func (c charClass) space() bool       { return c&clsSpace != 0 }
func (c charClass) nonCJKPunct() bool { return c&clsPunct != 0 && c&clsCJK == 0 }
func (c charClass) cjk() bool         { return c&clsCJK != 0 }
func (c charClass) cjkOrIVS() bool    { return c&(clsCJK|clsIVS) != 0 }
func (c charClass) spaceOrPunct() bool {
	return c&(clsSpace|clsPunct) != 0
}

func isCJK(r rune) bool {
	i := sort.Search(len(cjkRanges), func(i int) bool { return cjkRanges[i][1] >= r })
	return i < len(cjkRanges) && cjkRanges[i][0] <= r
}

// classify 는 글자 하나를 분류한다. 공백·문장부호 판정은 goldmark 와 같은 함수를 쓴다 — CJK 가 아닌 입력에서 결과가 같아야 한다.
func classify(r rune) charClass {
	if util.IsSpaceRune(r) {
		return clsSpace
	}
	if r >= 0xFE00 && r <= 0xFE0E {
		return clsSVS
	}
	if r >= 0xE0100 && r <= 0xE01EF {
		return clsIVS
	}
	var c charClass
	if r >= 0x1100 && isCJK(r) {
		c |= clsCJK
	}
	if util.IsPunctRune(r) {
		c |= clsPunct
	}
	return c
}

// CJK ambiguous punctuation sequence: ‘ ’ “ ” + U+FE01 (StandardizedVariants.txt 의 fullwidth form)
func isCJKAmbiguousPunct(base, vs rune) bool {
	return vs == 0xFE01 && (base == 0x2018 || base == 0x2019 || base == 0x201C || base == 0x201D)
}

// precedingClass 는 구분자 앞 글자의 분류다. 앞 글자가 이체자 선택자(SVS)면 그 앞 글자를 본다
// (참조 구현의 classifyPrecedingCharacter).
func precedingClass(block text.Reader, before rune) charClass {
	c := classify(before)
	if c != clsSVS {
		return c
	}
	_, seg := block.Position()
	src := block.Source()
	end := seg.Start - utf8.RuneLen(before)
	if end <= 0 || end > len(src) {
		return c
	}
	two, _ := utf8.DecodeLastRune(src[:end])
	tc := classify(two)
	switch {
	case tc.space():
		return c
	case isCJKAmbiguousPunct(two, before):
		return clsCJK | clsPunct
	}
	return tc &^ clsIVS
}

// flanking 은 CJK-friendly 명세 6.2 의 두 정의다.
//
//	left : (1) 뒤가 공백 아님, (2a) 뒤가 non-CJK 문장부호 아님 또는
//	       (2b) 앞이 공백·non-CJK 문장부호 시퀀스·CJK 시퀀스·IVS
//	right: (1) 앞이 공백 아님, (2a) 앞이 non-CJK 문장부호 시퀀스 아님 또는
//	       (2b) 뒤가 공백·non-CJK 문장부호·CJK 글자
func flanking(before, after charClass) (left, right bool) {
	left = !after.space() && (!after.nonCJKPunct() || before.space() || before.nonCJKPunct() || before.cjkOrIVS())
	right = !before.space() && (!before.nonCJKPunct() || after.space() || after.nonCJKPunct() || after.cjk())
	return
}

// scanDelimiter 는 goldmark parser.ScanDelimiter 와 같은 일을 하되 판정만 CJK-friendly 로 한다.
func scanDelimiter(block text.Reader, processor parser.DelimiterProcessor) *parser.Delimiter {
	line, _ := block.PeekLine()
	c := line[0]
	if !processor.IsDelimiter(c) {
		return nil
	}
	j := 0
	for j < len(line) && line[j] == c {
		j++
	}
	afterRune := ' '
	if j < len(line) {
		afterRune = util.ToRune(line, j)
	}
	before := precedingClass(block, block.PrecendingCharacter())
	after := classify(afterRune)
	left, right := flanking(before, after)

	var canOpen, canClose bool
	if c == '_' {
		// _ 는 단어 안에서 열고 닫지 않는다. "문장부호" 가 "문장부호 시퀀스" 가 된 것만 다르다
		canOpen = left && (!right || before.spaceOrPunct())
		canClose = right && (!left || after.spaceOrPunct())
	} else {
		canOpen = left
		canClose = right
	}
	return parser.NewDelimiter(canOpen, canClose, j, c, processor)
}

// ---- * _ 강조

type emphasisProcessor struct{}

func (emphasisProcessor) IsDelimiter(b byte) bool { return b == '*' || b == '_' }
func (emphasisProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}
func (emphasisProcessor) OnMatch(consumes int) ast.Node { return ast.NewEmphasis(consumes) }

type cjkEmphasisParser struct{ st *state }

func (cjkEmphasisParser) Trigger() []byte { return []byte{'*', '_'} }

func (p cjkEmphasisParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	_, segment := block.PeekLine()
	d := scanDelimiter(block, emphasisProcessor{})
	if d == nil {
		return nil
	}
	if t := p.st.tooManyDelims(parent, block, segment, d.OriginalLength); t != nil {
		return t
	}
	d.Segment = segment.WithStop(segment.Start + d.OriginalLength)
	block.Advance(d.OriginalLength)
	pc.PushDelimiter(d)
	return d
}

// ---- ~~ 취소선 (GFM). goldmark 처럼 ~ 한두 개만, 앞이 ~ 이면 받지 않는다.

type strikeProcessor struct{}

func (strikeProcessor) IsDelimiter(b byte) bool { return b == '~' }
func (strikeProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}
func (strikeProcessor) OnMatch(consumes int) ast.Node { return extast.NewStrikethrough() }

type cjkStrikeParser struct{ st *state }

func (cjkStrikeParser) Trigger() []byte { return []byte{'~'} }

func (p cjkStrikeParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	_, segment := block.PeekLine()
	d := scanDelimiter(block, strikeProcessor{})
	if d == nil {
		return nil
	}
	if d.OriginalLength > 2 || before == '~' {
		// 취소선이 될 수 없는 ~ 묶음(셋 이상, 또는 ~ 바로 뒤)은 묶음 통째로 글자다. nil 을 돌려주면 goldmark 가 다음 ~ 에서
		// 같은 묶음을 다시 읽어(기본 취소선 파서도) 긴 묶음 하나가 길이의 제곱이었다 — `~` 128KB 3.9초(리뷰 R4, 기존 결함)
		block.Advance(d.OriginalLength)
		return ast.NewTextSegment(segment.WithStop(segment.Start + d.OriginalLength))
	}
	if t := p.st.tooManyDelims(parent, block, segment, d.OriginalLength); t != nil {
		return t
	}
	d.Segment = segment.WithStop(segment.Start + d.OriginalLength)
	block.Advance(d.OriginalLength)
	pc.PushDelimiter(d)
	return d
}

// ---- 구분자 수 상한

// MaxDelimitersPerBlock 은 문단(블록) 하나에서 강조·취소선 구분자로 읽는 *·_·~ 의 묶음 수 상한이다. 넘는 것은 글자로 둔다(경고 한 번).
// goldmark 의 구분자 처리(ProcessDelimiters)는 서로 짝이 될 수 없는 구분자가 번갈아 쌓이면(`_a*_a*…`) 구분자 수의 제곱에 비례한다 —
// 128KB 문단 하나가 9초였다(수식 속성 시험이 찾은 기존 결함, 순수 goldmark 에서도 같다). 사람이 쓰는 문단은 구분자가 수십 개다.
const MaxDelimitersPerBlock = 1000

// tooManyDelims 는 이 블록의 구분자가 상한을 넘었으면 n 바이트를 글자로 읽은 노드를, 아니면 nil 을 돌려준다(셈은 늘린다).
func (st *state) tooManyDelims(parent ast.Node, block text.Reader, segment text.Segment, n int) ast.Node {
	if st == nil {
		return nil // 상한 없이(CommonMark 예제 시험이 파서만 따로 쓴다)
	}
	st.delims[parent]++
	if st.delims[parent] <= MaxDelimitersPerBlock {
		return nil
	}
	if st.delims[parent] == MaxDelimitersPerBlock+1 {
		st.warn(st.lines.line(segment.Start), WarnTooComplex,
			fmt.Sprintf("강조 표시(* _ ~)가 %d 개를 넘어 이 문단의 뒤의 것은 글자로 둠", MaxDelimitersPerBlock))
	}
	block.Advance(n)
	return ast.NewTextSegment(segment.WithStop(segment.Start + n))
}
