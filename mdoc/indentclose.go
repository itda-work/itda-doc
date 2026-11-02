package mdoc

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// 들여쓴 닫는 줄(#494 안 C). 닫는 `:::` 는 0~3칸 들여쓰기까지만 닫는 줄이다(container.go Continue 의 w >= 4) — 파싱 규칙은
// 그대로 두고, 닫히지 않은 컨테이너가 바로 품은 글 블록(문단·목록의 글·들여쓴 코드)에 4칸 이상 들여쓴 콜론 줄이 있으면
// 그 줄이 닫으려던 줄로 보고 알린다. 목록 바로 뒤에 빈 줄 없이 `:::` 를 쓰면 포매터(Prettier)가 그 줄을 목록 문단의
// 이어지는 줄로 보고 들여써서 생긴다. 고칠 내용(Fix)은 데이터로만 싣는다 — 파일은 화면이 사용자의 확인을 받아 고친다.

// Fix 는 경고를 없애는 원문 바꿈 하나다: Line 줄의 글이 Old 와 같으면 그 줄을 New 로 바꾼다(줄 끝 \n 은 둘 다 넣지 않는다 —
// New 의 \n 은 줄을 하나 더 넣는다는 뜻이다). mdoc 은 파일을 쓰지 않는다.
type Fix struct {
	Line int
	Old  string
	New  string
}

// WarnIndentedClose(mdoc.go)는 그 컨테이너의 unclosed-container 경고를 대신한다 — 같은 원인 하나에 경고 하나, 고치면 함께 사라진다.

// indentedCloseLine 은 줄(줄 끝 \n 없음)이 4칸 이상 들여쓴 닫는 줄 꼴(`^[ \t]+:{3,}[ \t]*$`, 들여쓰기 너비 4 이상)인지다.
// 너비는 goldmark 처럼 탭을 다음 4칸 자리까지로 센다(util.IndentWidth) — 탭 하나로 들여쓴 줄도 Continue 의 w >= 4 에 걸려 닫지 못한다.
func indentedCloseLine(s string) bool {
	if indentWidth(s, 0) < 4 {
		return false
	}
	return isCloseLine([]byte(strings.TrimLeft(s, " \t")))
}

// srcLine 은 1부터 센 l 줄의 글이다(줄 끝 \n 없음). 범위 밖이면 "".
func (st *state) srcLine(l int) string {
	if l < 1 || l > len(st.lines) {
		return ""
	}
	start := st.lines[l-1]
	end := len(st.src)
	if l < len(st.lines) {
		end = st.lines[l] - 1
	}
	if start > end {
		return ""
	}
	return string(st.src[start:end])
}

// indentedClose 는 닫히지 않은 컨테이너 n 의 들여쓴 닫는 줄 경고를 낸다(container.go Close). 후보가 없으면 거짓 — 그때는
// 보통의 unclosed-container 경고를 낸다. 후보는 n 이 바로 품은 글 블록의 줄만 본다: 펜스 코드·수식 블록의 줄은 코드다,
// 안쪽 컨테이너 안의 줄은 그 컨테이너가(닫혔으면 아예) 맡는다 — 바꾸면 안쪽이 먼저 닫혀 뜻이 달라진다. 여럿이면 첫 줄 하나만
// 낸다(고친 뒤 다시 그리면 남은 것이 다시 드러난다).
func (st *state) indentedClose(n *Container) bool {
	line := 0
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if line > 0 {
			return ast.WalkStop, nil
		}
		if !entering {
			return ast.WalkContinue, nil
		}
		if c == n {
			return ast.WalkContinue, nil
		}
		// 줄을 보는 것은 글 블록(문단·목록의 글·들여쓴 코드)뿐이다 — 펜스 코드·수식 블록·HTML 블록의 줄은 보지 않는다
		switch c.(type) {
		case *Container:
			return ast.WalkSkipChildren, nil
		case *ast.Paragraph, *ast.TextBlock, *ast.CodeBlock:
			ls := c.Lines()
			for i := 0; i < ls.Len(); i++ {
				l := st.lines.line(ls.At(i).Start)
				if indentedCloseLine(st.srcLine(l)) {
					line = l
					break
				}
			}
			return ast.WalkSkipChildren, nil
		}
		if c.Type() != ast.TypeBlock {
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	if line == 0 {
		return false
	}
	// 그 줄 뒤에서 이 컨테이너에 들어간 제목(h1~h6, 안쪽 컨테이너 안 것 포함)
	heads := 0
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := c.(*ast.Heading); ok {
			if st.blockLine(h) > line {
				heads++
			}
			return ast.WalkSkipChildren, nil
		}
		if c.Type() != ast.TypeBlock {
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	name := n.Name
	if name == "" {
		name = "{…}"
	}
	until := "감싼 블록 끝까지"
	if _, ok := n.Parent().(*ast.Document); ok {
		until = "문서 끝까지"
	}
	msg := fmt.Sprintf("%d줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — %d줄의 :::%s 가 %s 이어짐", line, st.lines.line(n.Pos()), name, until)
	if heads > 0 {
		msg += fmt.Sprintf("(뒤 제목 %d개가 그 안에 들어감)", heads)
	}
	// 의도한 들여쓴 코드·목록 글도 후보가 되므로 조건부로 안내한다(#494 리뷰 1회차)
	fix := st.indentedCloseFix(n, line)
	if fix != nil {
		msg += ". 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다"
	} else {
		msg += ". 이 줄로 닫으려던 것이라면 들여쓰기를 여는 ::: 와 같은 칸으로 줄이고 앞에 빈 줄을 둔다"
	}
	st.warns = append(st.warns, Warning{Line: line, Code: WarnIndentedClose, Message: msg, Fix: fix})
	return true
}

// indentedCloseFix 는 line 줄을 컨테이너 n 을 닫는 줄로 바꾸는 Fix 다. 넘친 들여쓰기만 지우고 여는 줄의 앞 공백(부모 목록·각주가
// 쓰는 들여쓰기)은 그대로 둔다 — 지우면 닫는 줄이 부모 밖으로 나가 부모가 먼저 끝난다. 안전한 접두를 정할 수 없으면 nil(경고만):
// 여는 줄이 공백 뒤 바로 `:` 로 시작하지 않을 때(인용 `>`·목록 기호 `- :::` 처럼 접두가 글자일 때), 그 줄이 여는 줄의 앞 공백
// 그대로로 시작하지 않을 때, 접두 뒤 들여쓰기가 4칸이 안 될 때(그런 줄은 이미 닫는 줄이다 — 다른 까닭으로 못 닫은 것).
func (st *state) indentedCloseFix(n *Container, line int) *Fix {
	open := st.srcLine(st.lines.line(n.Pos()))
	pre := open[:len(open)-len(strings.TrimLeft(open, " \t"))]
	if !strings.HasPrefix(open[len(pre):], ":") {
		return nil
	}
	old := st.srcLine(line)
	if !strings.HasPrefix(old, pre) {
		return nil
	}
	rest := old[len(pre):]
	if indentWidth(rest, indentWidth(pre, 0)) < 4 {
		return nil
	}
	fix := pre + strings.TrimLeft(rest, " \t")
	if strings.TrimSpace(st.srcLine(line-1)) != "" {
		fix = "\n" + fix
	}
	return &Fix{Line: line, Old: old, New: fix}
}

// indentWidth 는 col 칸에서 시작한 s 의 앞 공백·탭 너비다(탭은 다음 4칸 자리까지, util.IndentWidth 와 같다).
func indentWidth(s string, col int) int {
	w := col
	for i := 0; i < len(s) && (s[i] == ' ' || s[i] == '\t'); i++ {
		if s[i] == '\t' {
			w += 4 - w%4
		} else {
			w++
		}
	}
	return w - col
}
