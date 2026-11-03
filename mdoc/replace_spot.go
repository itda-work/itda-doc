package mdoc

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// 「스냅샷 바꾸기」(itda-hyve#365 ③ 조각 D)의 범위 판정 — 읽기 전용이다. 렌더 경로·옵션은 바꾸지 않는다(InsertSpotAt 과 같은 읽기 파싱 parseSpots,
// 차트 값 읽기는 렌더의 transform 을 고치지 않고 그대로 부른다).

// ReplaceFitsAt 은 원문 from~to 줄(1부터, 양끝 포함 — 기록 주석 줄부터 블록 끝 줄까지)을 block(끝 줄바꿈 없는 세 층 글 — 기록 주석
// 한 줄·표 또는 :::chart·캡션)으로 바꿔도 되는지다. 받는 조건은 넷이다:
//
//   - 바꾸기 전: from 줄이 홀로 한 줄인 최상위 HTML 블록이고, from+1 줄에서 시작하는 최상위 표(캡션 포함)·닫힌 :::chart 가 정확히 to
//     줄에서 끝난다. 목록·인용·컨테이너(다단 포함)·각주 정의 안이면 최상위가 아니라 받지 않는다.
//   - 그 범위가 각주 정의 구간과 겹치지 않고 frontmatter 밖이다.
//   - 바꾼 뒤: from 줄이 다시 홀로 한 줄인 최상위 HTML 블록이고 from+1 줄에서 최상위 표·차트가 시작한다.
//   - 범위 밖 원문 구조(블록 꼴·조상·순서·글, 각주 정의 구간 — InsertFitsAt 의 outline)가 그대로다.
//
// src 는 편집기 글이다(정규화해 읽는다 — 줄 번호는 편집기 줄과 같다). 읽기 전용이고 예기치 못한 멈춤(panic)은 거짓이다.
func ReplaceFitsAt(src []byte, from, to int, block string) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	if from < 1 || to <= from || strings.ContainsRune(block, '\r') {
		return false
	}
	p, ok := parseSpots(src)
	if !ok || to > len(p.st.lines) || p.inFrontmatter(from) {
		return false
	}
	if !recordRange(p.src, from, to) {
		return false
	}
	for _, sp := range p.spans {
		if sp.start <= to && from <= sp.end {
			return false
		}
	}
	s := string(p.src)
	pos, end := lineStart(s, from), lineStart(s, to+1)
	if end > pos && end <= len(s) && s[end-1] == '\n' {
		end-- // to 줄의 줄바꿈은 남긴다 — 바꾸는 것은 줄 글뿐이다
	}
	out := s[:pos] + block + s[end:]
	n := strings.Count(block, "\n") + 1
	if !recordRange([]byte(out), from, from+n-1) {
		return false
	}
	q, ok := parseSpots([]byte(out))
	if !ok {
		return false
	}
	before, ok1 := p.outline(func(l int) (int, bool) { return l, l < from || l > to })
	after, ok2 := q.outline(replacedLines(from, to, n))
	return ok1 && ok2 && slices.Equal(before, after)
}

// recordRange 는 src 의 from 줄이 홀로 한 줄인 최상위 HTML 블록이고 from+1 줄에서 시작하는 최상위 표·닫힌 차트가 to 줄에서 끝나는지다.
func recordRange(src []byte, from, to int) bool {
	hb, ok := TopBlockAt(src, from)
	if !ok || hb.Kind != "html" || hb.Start != from || hb.End != from {
		return false
	}
	tb, ok := TopBlockAt(src, from+1)
	return ok && (tb.Kind == "table" || tb.Kind == "chart") && tb.Closed && tb.Start == from+1 && tb.End == to
}

// lineStart 는 s 의 l 줄(1부터) 첫 바이트 자리다 — 줄 수 + 1 이면 len(s).
func lineStart(s string, l int) int {
	pos := 0
	for i := 1; i < l; i++ {
		j := strings.IndexByte(s[pos:], '\n')
		if j < 0 {
			return len(s)
		}
		pos += j + 1
	}
	return pos
}

// replacedLines 는 from~to 줄을 n 줄 글로 바꾼 글의 줄 번호를 원문 줄 번호로 되돌린다 — 새 글 줄은 ok 가 거짓이다.
func replacedLines(from, to, n int) func(int) (int, bool) {
	return func(l int) (int, bool) {
		switch {
		case l < 1:
			return l, true // 자리를 모르는 블록(blockLine 0)은 그대로 견준다
		case l < from:
			return l, true
		case l < from+n:
			return 0, false
		default:
			return l - n + (to - from + 1), true
		}
	}
}

// ChartSeries 는 렌더러가 그리는 계열 하나다. Col 은 그 계열을 읽은 표의 열(0 부터 — 0 은 항목 열)이고, Values[j] 는 j 번째 항목 값,
// Has[j] 가 거짓이면 빈 칸(읽지 못한 칸)이다. 원·도넛은 첫 계열만 그리므로 계열이 하나다 — 그 규칙은 렌더러가 정하고 여기는 그 결과다.
type ChartSeries struct {
	Col    int
	Values []float64
	Has    []bool
}

// ChartValues 는 원문 src 의 첫 최상위 차트를 렌더와 같은 판정(transform — 단위·보조 축 unit2·% 추론·그리는 계열)으로 읽은 계열이다.
// 차트로 그리지 않은 블록(표로 남음)·산점도·버블은 ok 가 거짓이다. 「스냅샷 바꾸기」(itda-hyve#365 ③ D)가 보존한 차트 속성이 새 값 읽기를
// 바꾸는지 보려고 쓴다 — 읽기 전용.
func ChartValues(src []byte) (series []ChartSeries, ok bool) {
	defer func() {
		if recover() != nil {
			series, ok = nil, false
		}
	}()
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 {
		return nil, false
	}
	st.transform(doc.(*ast.Document))
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		ch, isChart := n.(*Chart)
		if !isChart {
			continue
		}
		if ch.Scatter != nil {
			return nil, false
		}
		for _, s := range ch.Series {
			series = append(series, ChartSeries{Col: s.Col, Values: append([]float64(nil), s.Values...), Has: append([]bool(nil), s.Has...)})
		}
		return series, true
	}
	return nil, false
}
