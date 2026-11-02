package texmath

import (
	"fmt"
	"strings"
)

// 막는 명령. Temml 이 알지만 "문서에 모양을 적지 않는다"(설계 1.1) 와 1.9 에 어긋나는 것이다.
// 모르는 명령은 Temml 이 알린다(CodeUnsupported). 여기 없는 것이 결과에 모양을 남기면 sanitize 가 거부한다.
var blocked = map[string]string{}

func init() {
	groups := []struct {
		why   string
		names string
	}{
		{"색은 테마가 정한다", `\color \textcolor \colorbox \fcolorbox \definecolor \mathcolor \pagecolor \bgcolor`},
		{"글자 크기는 테마가 정한다", `\tiny \scriptsize \footnotesize \small \normalsize \large \Large \LARGE \huge \Huge \fontsize \selectfont`},
		{"HTML·주소는 수식에 넣지 않는다", `\href \url \includegraphics \htmlClass \htmlId \htmlStyle \htmlData \class \cssId \style \bbox`},
		{"배치를 직접 정하는 명령은 쓰지 않는다", `\rule \raisebox \llap \rlap \clap \mathllap \mathrlap \mathclap \hbox \vbox \vcenter`},
		{"식 번호는 붙이지 않는다", `\tag \label \ref \eqref \nonumber \notag`},
		{"매크로 이름을 만드는 명령은 쓰지 않는다", `\csname \endcsname \expandafter \string \noexpand \futurelet`},
	}
	for _, g := range groups {
		for _, n := range strings.Fields(g.names) {
			blocked[n] = g.why
		}
	}
}

// 환경. 번호를 붙이는 AMS 환경은 별표 판으로 바꿔 번호 없이 그린다(식 번호는 붙이지 않는다 — 결과가 같고 번호만 없다).
// 막는 환경은 Temml 이 style 로 배치하는 것이다.
var (
	numberedEnv = map[string]bool{"equation": true, "align": true, "gather": true, "alignat": true, "multline": true, "flalign": true}
	blockedEnv  = map[string]string{"CD": "가환 도표(CD)는 지원하지 않는다"}
)

// check 는 Temml 에 넘기기 전의 검사다. 넘길 TeX(번호 환경을 별표 판으로 바꾼 것)를 돌려준다.
func check(tex string) (string, error) {
	if len(tex) > MaxInputBytes {
		return "", &Error{Code: CodeLimit, Msg: fmt.Sprintf("수식이 너무 김(%d 바이트, 상한 %d)", len(tex), MaxInputBytes)}
	}
	if strings.TrimSpace(tex) == "" {
		return "", &Error{Code: CodeSyntax, Msg: "빈 수식"}
	}
	var out strings.Builder
	depth, maxDepth := 0, 0
	push := func() {
		depth++
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	for i := 0; i < len(tex); {
		c := tex[i]
		switch {
		case c == '{':
			push()
		case c == '}':
			depth--
		case c == '%':
			// 주석은 줄 끝까지다. 그 안의 명령은 Temml 도 읽지 않는다
			j := strings.IndexByte(tex[i:], '\n')
			if j < 0 {
				j = len(tex) - i
			}
			out.WriteString(tex[i : i+j])
			i += j
			continue
		case c == '\\':
			name, n := controlWord(tex[i:])
			if n == 0 {
				// \ 하나 뒤의 글자(\{ \$ \\ 등)는 명령 이름이 아니다
				w := 2
				if i+1 >= len(tex) {
					w = 1
				}
				out.WriteString(tex[i : i+w])
				i += w
				continue
			}
			if why, ok := blocked[name]; ok {
				return "", &Error{Code: CodeBlocked, Msg: "쓰지 않는 명령 " + name + " — " + why, Command: name}
			}
			switch name {
			case `\left`, `\begin`:
				push()
			case `\right`, `\end`:
				depth--
			}
			if name == `\begin` || name == `\end` {
				env, m := envName(tex[i+n:])
				if why, ok := blockedEnv[env]; ok {
					return "", &Error{Code: CodeBlocked, Msg: "쓰지 않는 환경 " + env + " — " + why, Command: env}
				}
				if numberedEnv[env] {
					out.WriteString(name + "{" + env + "*}")
					i += n + m
					continue
				}
			}
			out.WriteString(tex[i : i+n])
			i += n
			continue
		}
		out.WriteByte(c)
		i++
	}
	if maxDepth > MaxDepth {
		return "", &Error{Code: CodeLimit, Msg: fmt.Sprintf("수식의 중첩이 %d 단계를 넘음", MaxDepth)}
	}
	return out.String(), nil
}

// controlWord 는 s 가 \글자… 로 시작하면 "\이름" 과 바이트 수다(TeX 의 control word — 이름은 ASCII 글자뿐).
// \tag* 처럼 별표가 붙어도 이름은 별표를 뺀 것이다.
func controlWord(s string) (string, int) {
	j := 1
	for j < len(s) && (s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
		j++
	}
	if j == 1 {
		return "", 0
	}
	return s[:j], j
}

// envName 은 \begin 뒤의 {이름} 을 읽는다(앞 공백 허용). 없으면 "".
func envName(s string) (string, int) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	if i >= len(s) || s[i] != '{' {
		return "", 0
	}
	end := strings.IndexByte(s[i:], '}')
	if end < 0 {
		return "", 0
	}
	return strings.TrimSpace(s[i+1 : i+end]), i + end + 1
}
