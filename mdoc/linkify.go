package mdoc

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// linkifyGuard 는 goldmark 의 자동 링크(linkify) 파서를 감싸 이차 시간을 막는다.
//
// linkify 는 공백·* · _ · ~ · ( 마다 불려 그 뒤를 이메일로 읽어 본다(util.FindEmailIndex) — 이메일 앞부분에 쓰이는 글자의
// 줄을 끝까지 읽고 @ 와 도메인을 찾는다. `a_b_c_…` 처럼 그 글자만으로 된 긴 줄에서는 _ 마다 같은 줄을 끝까지 다시 읽어
// 112KB 문단 하나에 0.55초, 448KB 에 수 초가 걸렸다(수식 리뷰 R1 의 재현에서 함께 드러났다 — 수식과 무관한 기존 결함).
//
// 같은 줄 안의 뒤 자리에서 읽어도 이메일의 뒷부분(@·도메인)은 같으므로 결과가 같다. 그래서 줄 하나에서 이메일이 되지 않았으면
// 그 줄의 [시작, 끝) 을 기억하고, 그 안의 뒤 자리에서 불리면 다시 읽지 않고 넘긴다. URL 접두사(http:·https:·ftp:·www.)로
// 시작하는 자리는 URL 만 따로 본다 — 원래 파서에 넘기는 것과 같은 정규식으로 맞으면 원래 파서에 넘기고(링크가 된다),
// 맞지 않으면 원래 파서도 이어서 이메일을 읽다 같은 자리에서 실패하므로 넘긴다(리뷰 R2: `www.1_` 반복이 이차였다).
// 결과가 원래 파서와 같다(TestLinkifyGuardSame).
type linkifyGuard struct {
	inner          parser.InlineParser
	runStart, stop int // 이메일이 되지 않은 글자 줄(원문의 절대 위치)
}

// goldmark extension/linkify.go 의 기본 정규식 그대로(MIT). 원래 파서에도 이것을 옵션으로 준다 — 덮개와 파서가 같은 정규식을 쓴다.
var (
	linkifyWWW = regexp.MustCompile(`^www\.[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+(?:[/#?][-a-zA-Z0-9@:%_\+.~#!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`)
	linkifyURL = regexp.MustCompile(`^(?:http|https|ftp)://[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+(?::\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`)
)

func newLinkifyGuard() *linkifyGuard {
	inner := extension.NewLinkifyParser(extension.WithLinkifyURLRegexp(linkifyURL), extension.WithLinkifyWWWRegexp(linkifyWWW))
	return &linkifyGuard{inner: inner, runStart: -1, stop: -1}
}

func (g *linkifyGuard) Trigger() []byte { return g.inner.Trigger() }

// 원래 파서가 URL 정규식을 돌리는 접두사(protoHTTP·protoHTTPS·protoFTP·domainWWW)
var (
	urlPrefixes = [][]byte{[]byte("http:"), []byte("https:"), []byte("ftp:")}
	wwwPrefix   = []byte("www.")
)

// urlAt 은 원래 파서가 이 자리를 URL 링크로 읽는지다(정규식이 0 에서 맞음 — 둘 다 ^ 로 묶였다).
// 맞는지만 보면 되므로 앞 urlProbe 바이트만 본다: 맞으려면 접두사(8 이하)·도메인 글자 256 이하·"." 과 글자 하나가
// 그 안에 있어야 하고, 뒤의 포트·경로는 선택이라 맞음 여부를 바꾸지 않는다.
const urlProbe = 300

func urlAt(rest []byte) bool {
	probe := rest[:min(len(rest), urlProbe)]
	for _, p := range urlPrefixes {
		if bytes.HasPrefix(rest, p) {
			k := bytes.Index(probe, []byte("://"))
			return k >= 0 && dotLetter(probe[k+3:]) && linkifyURL.Match(probe)
		}
	}
	return bytes.HasPrefix(rest, wwwPrefix) && dotLetter(probe[len(wwwPrefix):]) && linkifyWWW.Match(probe)
}

// dotLetter 는 두 정규식이 맞기 위해 꼭 있어야 하는 것(도메인 글자 하나 뒤의 "." + 소문자)이 b 에 있는지다.
// 정규식을 돌리기 전의 싼 거름 — 없으면 정규식도 맞지 않는다(www.1_ 반복에서 자리마다 정규식을 돌리지 않게).
func dotLetter(b []byte) bool {
	for j := 1; j+1 < len(b); j++ {
		if b[j] == '.' && b[j+1] >= 'a' && b[j+1] <= 'z' {
			return true
		}
	}
	return false
}

func (g *linkifyGuard) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) == 0 {
		return g.inner.Parse(parent, block, pc)
	}
	start, rest := seg.Start, line
	if c := line[0]; c == ' ' || c == '*' || c == '_' || c == '~' || c == '(' {
		start, rest = start+1, line[1:]
	}
	if start > g.runStart && start < g.stop {
		// 이미 이메일이 되지 않은 줄의 뒤 자리 — URL 이 아니면 원래 파서도 nil 이다(이메일 판정은 줄 끝이 같아 같다)
		if urlAt(rest) {
			return g.inner.Parse(parent, block, pc)
		}
		return nil
	}
	n := g.inner.Parse(parent, block, pc)
	// 원래 파서가 이메일을 읽어 보고 실패한 때만 기억한다. 첫 글자가 문장부호이거나(-a_b@x.com 의 뒤 _ 에서는 이메일이 된다)
	// 링크 글 안이면 이메일을 읽지 않고 nil 이라 같은 줄의 뒤 자리에 대해 아무것도 알 수 없다
	if n == nil && !pc.IsInLinkLabel() && len(rest) > 0 && !util.IsPunct(rest[0]) {
		i := 0
		for i < len(rest) && emailLocal(rest[i]) {
			i++
		}
		if i > 0 {
			g.runStart, g.stop = start, start+i
		}
	}
	return n
}

// emailLocal 은 goldmark util 의 이메일 앞부분 글자 표(emailTable)와 같다: 영문·숫자와 !#$%&'*+-./=?^_`{|}~
func emailLocal(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return bytes.IndexByte([]byte("!#$%&'*+-./=?^_`{|}~"), c) >= 0
}
