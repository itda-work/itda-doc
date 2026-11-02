package state

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// mermaid 그대로의 예제 문법 — 모르는 것은 *Error(줄 번호), 쓰지 않는 것은 Issue(경고)다.

// errAt 는 원문을 그려 *Error 의 줄·메시지 일부를 견준다(Render — 원문 상한 검사까지 지나간다).
func errAt(t *testing.T, src string, line int, msg string) {
	t.Helper()
	_, err := Render(src)
	var de *diagram.Error
	if !errors.As(err, &de) {
		t.Fatalf("Render(%q) 오류가 *Error 가 아님: %v", src, err)
	}
	if de.Line != line || !strings.Contains(de.Msg, msg) {
		t.Errorf("Render(%q) 오류 %d:%q, 기대 %d 에 %q", src, de.Line, de.Msg, line, msg)
	}
}

// drawn 은 원문을 그려 SVG·경고를 얻는다.
func drawn(t *testing.T, src string) (string, []diagram.Issue) {
	t.Helper()
	r, err := Render(src)
	if err != nil {
		t.Fatalf("Render(%q) 실패: %v", src, err)
	}
	return r.SVG, r.Issues
}

// summary 는 읽은 그림을 한 줄 글로 줄인다 — 상태(ID 종류 [설명/…] @묶음) · 전이(왼 > 오른 "라벨") · 묶음(ID "제목") · 방향 · 접근성.
func summary(c *chart) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dir=%d", c.Dir)
	for _, s := range c.States {
		b.WriteString(" " + s.ID)
		if s.Kind != Normal {
			b.WriteString("(" + s.Kind.String() + ")")
		}
		if len(s.Descr) > 0 {
			b.WriteString("[" + strings.Join(s.Descr, "/") + "]")
		}
		if s.Cluster >= 0 {
			b.WriteString("@" + c.Comps[s.Cluster].ID)
		}
	}
	b.WriteString(" |")
	for _, tr := range c.Trans {
		b.WriteString(" " + c.States[tr.From].ID + ">" + c.States[tr.To].ID)
		if tr.Label != "" {
			b.WriteString(`"` + tr.Label + `"`)
		}
	}
	b.WriteString(" |")
	for _, cp := range c.Comps {
		b.WriteString(" " + cp.ID + `"` + cp.Title + `"`)
	}
	if c.AccTitle != "" || c.AccDescr != "" {
		b.WriteString(" | acc=" + c.AccTitle + "/" + c.AccDescr)
	}
	return b.String()
}

func parsed(t *testing.T, src string) string {
	t.Helper()
	c, _, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return summary(c)
}

// 문장마다 값 — 원본 stateDb·dataFetcher 의 뜻(처음 나온 차례, [*] 의 범위별 id, 설명 누적, 묶음 소속, 마지막 방향).
func TestParseStatements(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"stateDiagram\nA", "dir=0 A | |"},
		{"stateDiagram-v2\nA", "dir=0 A | |"},
		{"stateDiagram-v2\n  A : 첫째  \nA : 둘째\nA: 셋째", "dir=0 A[첫째/둘째/셋째] | |"},
		{"stateDiagram-v2\nstate \"보이는 이름\" as A\nA : 덧붙임", "dir=0 A[보이는 이름/덧붙임] | |"},
		{"stateDiagram-v2\n[*] --> A\nA --> B : 다음\nB --> [*]\n[*] --> B", "dir=0 root_start(start) A B root_end(end) | root_start>A A>B\"다음\" B>root_end root_start>B |"},
		{"stateDiagram-v2\nA-->B\nA-->A\nA --> B:x\n[*]-->[*]", "dir=0 A B root_start(start) root_end(end) | A>B A>A A>B\"x\" root_start>root_end |"},
		{"stateDiagram-v2\nstate 갈래 <<fork>>\nstate 합류 <<JOIN>>\nstate 고름 <<choice>>\n갈래 --> 합류", "dir=0 갈래(fork) 합류(join) 고름(choice) | 갈래>합류 |"},
		{"stateDiagram-v2\ndirection LR\nA\ndirection bt", "dir=1 A | |"},
		{"stateDiagram-v2\n접수 --> 포장\nstate 배송 {\n  [*] --> 포장\n  포장 --> 출고\n  출고 --> [*]\n}\n출고 --> 완료",
			"dir=0 접수 포장@배송 배송_start(start)@배송 출고@배송 배송_end(end)@배송 완료 | 접수>포장 배송_start>포장 포장>출고 출고>배송_end 출고>완료 | 배송\"배송\""},
		{"stateDiagram-v2\nstate \"배송 단계\" as 배송 {\n  가\n}\nstate 나머지 {\nB\n}", "dir=0 가@배송 B@나머지 | | 배송\"배송 단계\" 나머지\"나머지\""},
		{"stateDiagram-v2\nA:::강조 --> B:::x : 라벨\nC:::y\nD:::z : 설명", "dir=0 A B C D[설명] | A>B\"라벨\" |"},
		{"stateDiagram-v2\nA : 비율 #35; 와 #quot;\nA --> B : a : b", "dir=0 A[비율 # 와 \"] B | A>B\"a : b\" |"},
		{"stateDiagram-v2\naccTitle: 제목 ; # 끝\naccDescr {\n 첫 줄\n %% 메모\n 둘째 }\nA", "dir=0 A | | | acc=제목 ; # 끝/첫 줄\n둘째"},
		{"---\ntitle: 캡션\n---\nstateDiagram-v2\n%% 주석\n%%\nA\n  %%  ", "dir=0 A | |"},
		{"stateDiagram-v2\nstate 묶음 {\n%%\nA\n}", "dir=0 A@묶음 | | 묶음\"묶음\""},
		// 받는 엔티티 — 정책 B 의 이름 7개·숫자(entity.go)
		{"stateDiagram-v2\nA : #lt;#gt;#amp;#apos;#num;#semi; #169; #8364;\naccTitle: 기호 #169;", "dir=0 A[<>&'#; © €] | | | acc=기호 ©/"},
	} {
		if got := parsed(t, c.src); got != c.want {
			t.Errorf("%q\n 얻음 %s\n 기대 %s", c.src, got, c.want)
		}
	}
	// 오류 줄 번호
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"stateDiagram-v2\nnote right of A : 메모", 2, "메모"},
		// classDef default — 원본 lexer DEFAULT\s+(jison:98)가 내는 토큰을 문법이 받지 않는다(리뷰 1회차 [1])
		{"stateDiagram-v2\nclassDef default fill:red\nA", 2, "classDef default"},
		{"stateDiagram-v2\nA\nclassDef DEFAULT x", 3, "classDef default"},
		{"stateDiagram-v2\nA\nclassDef Default\tx", 3, "classDef default"},
		{"stateDiagram-v2\nstate 묶음 {\nA\nclassDef default x\n}", 4, "classDef default"},
		// 공통 디코더가 풀지 못하는 엔티티 — 원본은 &copy; 로 되돌린다(리뷰 1회차 [2])
		{"stateDiagram-v2\nA : #copy;", 2, "엔티티 #copy;"},
		{"stateDiagram-v2\nA\nA --> B : #euro;", 3, "엔티티 #euro;"},
		{"stateDiagram-v2\nstate \"#hellip;\" as A", 2, "엔티티 #hellip;"},
		{"stateDiagram-v2\nstate \"묶음 #zzz;\" as M {\nA\n}", 2, "엔티티 #zzz;"},
		{"stateDiagram-v2\nA : #AMP;", 2, "엔티티 #AMP;"},
		{"stateDiagram-v2\nA : #x41;", 2, "엔티티 #x41;"},
		{"stateDiagram-v2\nA : #12345678;", 2, "엔티티"},
		{"stateDiagram-v2\nA : #31;", 2, "엔티티 #31;"},
		{"stateDiagram-v2\naccTitle: 기호 #copy;\nA", 2, "엔티티"},
		{"stateDiagram-v2\naccDescr: #euro;\nA", 2, "엔티티"},
		{"stateDiagram-v2\naccDescr {\n#hellip;\n}\nA", 4, "엔티티"},
		{"stateDiagram-v2\nA : #32;", 2, "엔티티 #32;"},
		{"stateDiagram-v2\nA\nnote \"떠 있음\" as N", 3, "메모"},
		{"stateDiagram-v2\n[*] --> 묶음\nstate 묶음 {\nA\n}", 2, "복합 상태"},
		{"stateDiagram-v2\nstate 묶음 {\nA\n}\nB --> 묶음", 5, "복합 상태"},
		{"stateDiagram-v2\nstate 묶음 {\nA\n}\n묶음 : 설명", 5, "복합 상태"},
		{"stateDiagram-v2\nstate 바깥 {\nstate 안 {\nA\n}\n}", 3, "중첩"},
		{"stateDiagram-v2\nstate 묶음 {\nA\n--\nB\n}", 4, "동시 영역"},
		{"stateDiagram-v2\nstate 묶음 {\n}", 2, "빈 복합"},
		{"stateDiagram-v2\nstate 묶음 {\nA", 2, "닫히지"},
		{"stateDiagram-v2\nA\n}", 3, "짝 없는"},
		{"stateDiagram-v2\nstate 묶음 {\ndirection LR\nA\n}", 3, "묶음 안의 방향"},
		{"stateDiagram-v2\nstate 묶음 {\naccTitle: x\n}", 3, "accTitle"},
		{"stateDiagram-v2\nstate 묶음 {\nA\n}\nstate 다른 {\nA\n}", 6, "두 복합"},
		{"stateDiagram-v2\nstate 묶음 {\nA\n}\nstate 묶음 {\nB\n}", 5, "두 번"},
		{"stateDiagram-v2\nstate F [[fork]]", 2, "[[fork]]"},
		{"stateDiagram-v2\nA --> F\nstate F <<fork>>", 3, "처음 쓰기 전에"},
		{"stateDiagram-v2\nstate F <<fork>>\nF : 설명", 3, "설명을 붙이지"},
		{"stateDiagram-v2\nstate A", 2, "상태를 만들지 않는다"},
		{"stateDiagram-v2\nstate A B {", 2, "한 낱말"},
		{"stateDiagram-v2\nscale 350 width\nA", 2, "scale"},
		{"stateDiagram-v2\nA; B", 2, ";"},
		{"stateDiagram-v2\nA --> B ; C", 2, ";"},
		{"stateDiagram-v2\nA # 주석", 2, "#"},
		{"stateDiagram-v2\nA %% 주석", 2, "%%"},
		{"stateDiagram-v2\n\"따옴표 이름\" --> B", 2, "따옴표"},
		{"stateDiagram-v2\nclicked --> B", 2, "키워드 click"},
		{"stateDiagram-v2\nA --> Defaulted", 2, "키워드 default"},
		{"stateDiagram-v2\nStateful", 2, "키워드 state"},
		{"stateDiagram-v2\nNotebook : 공책", 2, "키워드 note"},
		{"stateDiagram-v2\nA --> hrefs", 2, "키워드 href"},
		{"stateDiagram-v2\nscaled", 2, "키워드 scale"},
		{"stateDiagram-v2\nhideout --> A", 2, "키워드 hide"},
		{"stateDiagram-v2\nclassDef 강조 fill:#f00\nA", 2, "확신할 수 없음"},
		{"stateDiagram-v2\nstyles --> A", 2, "키워드 style"},
		{"stateDiagram-v2\nclassroom", 2, "키워드 class"},
		{"stateDiagram-v2\nA --> root", 2, "root"},
		{"stateDiagram-v2\nA --> 묶음_start", 2, "_start"},
		{"stateDiagram-v2\nstate as {\nA\n}", 2, "as"},
		{"stateDiagram-v2\nA --> 𝔸", 2, "BMP"},
		{"stateDiagram-v2\n_a", 2, "_"},
		{"stateDiagram-v2\nA : a::b", 2, ":"},
		{"stateDiagram-v2\nA : 끝:", 2, ":"},
		{"stateDiagram-v2\nA : *강조*", 2, "*"},
		{"stateDiagram-v2\nA : - 목록", 2, "목록"},
		{"stateDiagram-v2\nA : 1. 첫째", 2, "목록"},
		{"stateDiagram-v2\nA : a<br>b", 2, "<"},
		{"stateDiagram-v2\nA : ", 2, "비어"},
		{"stateDiagram-v2\nA --> B :", 2, "비어"},
		{"stateDiagram-v2\nstate \"\" as A", 2, "비어"},
		{"stateDiagram-v2\nstate \" 앞 빈칸\" as A {\nB\n}", 2, "빈칸"},
		{"stateDiagram-v2\nstate \"x\" as A:y", 2, "state 문장은"},
		{"stateDiagram-v2\nA --> B --> C", 2, "전이 꼴"},
		{"stateDiagram-v2\nA -> B", 2, "-->"},
		{"stateDiagram-v2\n[*]", 2, "알 수 없는"},
		{"stateDiagram-v2\nA : direction LR", 2, "direction"},
		{"stateDiagram-v2\naccTitle: 방향 direction LR\nA", 2, "direction"},
		{"stateDiagram\ndirection", 2, "direction"},
		{"stateDiagram-V2\nA", 1, "stateDiagram"},
		{"statediagram\nA", 1, "stateDiagram"},
		{"stateDiagram-v2 direction LR\nA", 1, "stateDiagram"},
		{"A\nstateDiagram", 1, "stateDiagram"},
		{"stateDiagram-v2\n%% 메모\n", 0, "그릴 상태가 없음"},
		{"stateDiagram-v2\naccDescr {\n%%\n}\nA", 3, "글 없는"},
		{"stateDiagram-v2\naccTitle:\nA", 2, "글이 없음"},
		{"stateDiagram-v2\naccDescr {\n설명", 2, "닫히지"},
		{"stateDiagram-v2\nA" + string(rune(0xa0)) + "--> B", 2, "U+00A0"},
		{"stateDiagram-v2\nA" + string(rune(0xfeff)) + "", 2, "U+FEFF"},
		{"stateDiagram-v2\nA : x ﬂ°", 2, "내부 표식"},
		{"stateDiagram-v2\nA\rB", 2, "CR"},
		{"stateDiagram-v2\nclass 상태 강조", 2, "확신할 수 없음"},
		{"stateDiagram-v2\nstate 묶음 {\nclick A \"u\" \"t\"\n}", 3, "click"},
		{"stateDiagram-v2\nstate 묶음 {\nhide empty description\n}", 3, "hide"},
		{"%%{init: {}}%% A\nstateDiagram-v2\nA", 1, "설정 지시"},
		{"stateDiagram-v2\nstate 묶음 {\n%%{init: {}}%%\nA\n}", 3, "블록 안"},
	} {
		errAt(t, c.src, c.line, c.msg)
	}
}

// 쓰지 않는 줄은 경고(종류마다 한 번)하고 그림은 그린다.
func TestIgnoredLines(t *testing.T) {
	for _, c := range []struct {
		src  string
		want []string // 경고 줄:메시지 일부
	}{
		{"%%{init: {\"theme\":\"dark\"}}%%\nstateDiagram-v2\nA\n%%{init: {}}%%", []string{"1:설정 지시"}},
		{"---\ntitle: t\nconfig:\n  theme: dark\n---\nstateDiagram-v2\nA", []string{"3:config"}},
		{"stateDiagram-v2\nA\nstyle A fill:#f9f,stroke:#333,stroke-width:4px\nstyle A,B fill:#f00", []string{"3:style"}},
		{"stateDiagram-v2\nclassDef strong fill:#f00\nclassDef bad fill:#f00,color:white,font-weight:bold;\nA", []string{"2:classDef"}},
		// defaults 는 DEFAULT\s+ 에 맞지 않아 원본도 CLASSDEF_ID(\w+\s+ — jison:99)로 읽는다
		{"stateDiagram-v2\nclassDef defaults fill:#f00\nclassDef default1 x\nA", []string{"2:classDef"}},
		{"stateDiagram-v2\nA\nclass A,B bad\nC:::bad", []string{"3:class"}},
		{"stateDiagram-v2\nA --> B:::x", []string{"2:class"}},
		{"stateDiagram-v2\nA\nclick A \"https://example.com\" \"설명\"\nclick A href \"https://example.com\"", []string{"3:click"}},
		{"stateDiagram-v2\nhide empty description\nA", []string{"2:hide"}},
		{"stateDiagram-v2\nstate 묶음 {\nA\nstyle A fill:#f00\nclass A x\nclassDef x fill:#f00\n}", []string{"4:style", "5:class", "6:classDef"}},
	} {
		_, is := drawn(t, c.src)
		var got []string
		for _, i := range is {
			got = append(got, fmt.Sprintf("%d:%s", i.Line, i.Msg))
		}
		if len(got) != len(c.want) {
			t.Errorf("%q 경고 %v, 기대 %v", c.src, got, c.want)
			continue
		}
		for k, w := range c.want {
			if !strings.HasPrefix(got[k], strings.SplitN(w, ":", 2)[0]+":") || !strings.Contains(got[k], strings.SplitN(w, ":", 2)[1]) {
				t.Errorf("%q 경고 %d = %q, 기대 %q", c.src, k, got[k], w)
			}
		}
	}
}

// 지시 경계 고정 사례 — 지시 사이 문장·acc 값·라벨·주석·블록 안.
func TestDirectiveLines(t *testing.T) {
	for _, c := range []struct {
		src  string
		line int
	}{
		{"stateDiagram-v2\n%%{init: {}}%% A %%{init: {}}%%", 2},
		{"stateDiagram-v2\naccTitle: 앞 %%{init: {}}%% 뒤\nA", 2},
		{"stateDiagram-v2\nA : 설명 %%{init: {}}%%", 2},
		{"stateDiagram-v2\n%% 메모 %%{init: {}}%%\nA", 2},
		{"stateDiagram-v2\naccDescr {\n%%{init: {}}%%\n}\nA", 3},
		{"stateDiagram-v2\n%%{init: {", 2},
	} {
		errAt(t, c.src, c.line, "설정 지시")
	}
	if _, is := drawn(t, "stateDiagram-v2\n%%{init: {}}%%\nA"); len(is) != 1 {
		t.Errorf("한 줄 지시 경고 %v", is)
	}
}

// 상한 경계 — 바로 밑은 그림, 넘으면 원문.
func TestLimits(t *testing.T) {
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	for i := 0; i < MaxStates; i++ {
		fmt.Fprintf(&b, "S%d\n", i)
	}
	if _, err := Render(b.String()); err != nil {
		t.Errorf("상태 %d 개: %v", MaxStates, err)
	}
	errAt(t, b.String()+"S_X\n", MaxStates+2, "상태가")

	b.Reset()
	b.WriteString("stateDiagram-v2\n")
	for i := 0; i < MaxTransitions; i++ {
		fmt.Fprintf(&b, "S%d --> S%d\n", i%40, (i*7+1)%40)
	}
	if _, err := Render(b.String()); err != nil {
		t.Errorf("전이 %d 개: %v", MaxTransitions, err)
	}
	errAt(t, b.String()+"A --> B\n", MaxTransitions+2, "전이가")

	b.Reset()
	b.WriteString("stateDiagram-v2\n")
	for i := 0; i < MaxComposites; i++ {
		fmt.Fprintf(&b, "state C%d {\nX%d\n}\n", i, i)
	}
	if _, err := Render(b.String()); err != nil {
		t.Errorf("복합 상태 %d 개: %v", MaxComposites, err)
	}
	errAt(t, b.String()+"state CX {\nY\n}\n", 3*MaxComposites+2, "복합 상태가")

	b.Reset()
	b.WriteString("stateDiagram-v2\n")
	for i := 0; i < MaxDescrs; i++ {
		fmt.Fprintf(&b, "A : 설명%d\n", i)
	}
	if _, err := Render(b.String()); err != nil {
		t.Errorf("설명 %d 줄: %v", MaxDescrs, err)
	}
	errAt(t, b.String()+"A : 하나 더\n", MaxDescrs+2, "설명이")

	lbl := strings.Repeat("가", MaxLabel)
	if _, err := Render("stateDiagram-v2\nA --> B : " + lbl); err != nil {
		t.Errorf("라벨 %d 자: %v", MaxLabel, err)
	}
	errAt(t, "stateDiagram-v2\nA --> B : "+lbl+"가", 2, "자를 넘음")
	errAt(t, "stateDiagram-v2\nA : "+lbl+"가", 2, "자를 넘음")
	errAt(t, "stateDiagram-v2\nstate \""+lbl+"가\" as A", 2, "자를 넘음")
	// 이름도 200 자(리뷰 1회차 참고 관찰) — 상태·전이 끝·복합 상태
	name := "A" + strings.Repeat("b", MaxLabel-1)
	if _, err := Render("stateDiagram-v2\n" + name + "\nstate M" + strings.Repeat("m", MaxLabel-1) + " {\nX\n}"); err != nil {
		t.Errorf("이름 %d 자: %v", MaxLabel, err)
	}
	errAt(t, "stateDiagram-v2\n"+name+"b", 2, "자를 넘음")
	errAt(t, "stateDiagram-v2\nX --> "+name+"b", 2, "자를 넘음")
	errAt(t, "stateDiagram-v2\nstate "+name+"b {\nX\n}", 2, "자를 넘음")

	big := "stateDiagram-v2\nA\n" + strings.Repeat("%% 채움 글\n", MaxSource/10)
	if _, err := Render(big); err == nil || !strings.Contains(err.Error(), "KB") {
		t.Errorf("원문 상한: %v", err)
	}
}

func TestDeterminism(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 60; i++ {
		src := randChart(rng, 1+rng.Intn(16), rng.Intn(26))
		a, err := Render(src)
		if err != nil {
			t.Fatalf("%v\n%s", err, src)
		}
		b, _ := Render(src)
		if a.SVG != b.SVG {
			t.Fatalf("두 번 그린 SVG 가 다름\n%s", src)
		}
	}
}

// randChart 는 받아들이는 꼴만으로 된 무작위 그림이다(상태 n, 전이 m — 자기 전이·같은 쌍 여러 전이·[*]·특수 상태·
// 복합 상태 포함).
func randChart(rng *rand.Rand, n, m int) string {
	names := []string{"접수", "결제", "배송", "A", "B_1", "Payment", "재고", "완료", "취소", "검토", "보류", "환불"}
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	if rng.Intn(3) == 0 {
		b.WriteString("direction " + []string{"TB", "BT", "LR", "RL"}[rng.Intn(4)] + "\n")
	}
	st := func(i int) string { return fmt.Sprintf("%s%d", names[i%len(names)], i) }
	kinds := []string{"", "", "", "", " <<choice>>", " <<fork>>", " <<join>>"}
	nc := 0
	if n > 3 {
		nc = rng.Intn(4)
	}
	group := make([]int, n) // 상태마다 묶음(-1 뿌리)
	for i := range group {
		group[i] = -1
		if nc > 0 && rng.Intn(2) == 0 {
			group[i] = rng.Intn(nc)
		}
	}
	decl := func(i int) string {
		k := kinds[rng.Intn(len(kinds))]
		switch {
		case k != "":
			return "state " + st(i) + k
		case rng.Intn(3) == 0:
			return st(i) + " : " + "설명 " + strings.Repeat("긴 글 ", rng.Intn(10))
		case rng.Intn(4) == 0:
			return "state \"보이는 " + st(i) + "\" as " + st(i)
		}
		return st(i)
	}
	trans := func(a, c string) string {
		if rng.Intn(3) == 0 {
			return a + " --> " + c + " : " + []string{"다음", "여러 낱말 라벨", strings.Repeat("긴 라벨 ", 6)}[rng.Intn(3)]
		}
		return a + " --> " + c
	}
	for i := 0; i < n; i++ {
		if group[i] < 0 {
			b.WriteString(decl(i) + "\n")
		}
	}
	for g := 0; g < nc; g++ {
		fmt.Fprintf(&b, "state 묶음%d {\n", g)
		var in []int
		for i := 0; i < n; i++ {
			if group[i] == g {
				b.WriteString("  " + decl(i) + "\n")
				in = append(in, i)
			}
		}
		if len(in) == 0 {
			b.WriteString("  안" + fmt.Sprint(g) + "\n")
		} else {
			b.WriteString("  " + trans("[*]", st(in[0])) + "\n")
			for k := 1; k < len(in); k++ {
				b.WriteString("  " + trans(st(in[rng.Intn(k)]), st(in[k])) + "\n")
			}
			b.WriteString("  " + trans(st(in[len(in)-1]), "[*]") + "\n")
		}
		b.WriteString("}\n")
	}
	if n > 0 {
		b.WriteString(trans("[*]", st(0)) + "\n")
	}
	for j := 0; j < m; j++ {
		a, c := rng.Intn(n), rng.Intn(n)
		b.WriteString(trans(st(a), st(c)) + "\n")
	}
	if n > 0 && rng.Intn(2) == 0 {
		b.WriteString(trans(st(n-1), "[*]") + "\n")
	}
	return b.String()
}

type rect struct{ x0, y0, x1, y1 float64 }

func (r rect) overlaps(o rect, eps float64) bool {
	return r.x0 < o.x1-eps && o.x0 < r.x1-eps && r.y0 < o.y1-eps && o.y0 < r.y1-eps
}

func (r rect) contains(p layered.Pt, eps float64) bool {
	return p.X >= r.x0-eps && p.X <= r.x1+eps && p.Y >= r.y0-eps && p.Y <= r.y1+eps
}

// shapeDist 는 점과 상태 모양 테두리 사이의 거리다(밖은 +, 안은 −) — 원·마름모·사각형·둥근 사각형을 따로 잰다(구현의
// clipShape 를 쓰지 않는다).
func shapeDist(b box, c layered.Pt, p layered.Pt) float64 {
	dx, dy := p.X-c.X, p.Y-c.Y
	hw, hh := b.w/2, b.h/2
	switch b.kind {
	case Start, End:
		return math.Hypot(dx, dy) - hw
	case Choice:
		return (math.Abs(dx)/hw + math.Abs(dy)/hh - 1) * hw * hh / math.Hypot(hw, hh)
	}
	r := 0.0
	if b.kind == Normal {
		r = math.Min(roundR, math.Min(hw, hh))
	}
	qx, qy := math.Abs(dx)-(hw-r), math.Abs(dy)-(hh-r)
	out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return out + math.Min(math.Max(qx, qy), 0) - r
}

// TestLayoutProperties 는 무작위·극단 그림의 배치 속성을 본다: 모든 요소가 viewBox 안, 상태 상자끼리 겹침 없음, 라벨
// 상자끼리·상태와 겹침 없음, 화살촉 끝이 대상 상태 모양의 테두리에 닿음, 다른 묶음의 상태가 묶음 상자 안에 들지 않음,
// 그리는 차례(묶음 → 선 → 화살촉 → (바탕 → 글자) → 상태).
func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(4093))
	var srcs []string
	for i := 0; i < 250; i++ {
		srcs = append(srcs, randChart(rng, 1+rng.Intn(16), rng.Intn(24)))
	}
	one := "stateDiagram-v2\nA"
	var many strings.Builder
	many.WriteString("stateDiagram-v2\n")
	for i := 0; i < MaxStates; i++ {
		fmt.Fprintf(&many, "S%d --> S%d\n", i, (i*7+3)%MaxStates)
	}
	long := "stateDiagram-v2\nstate \"" + strings.Repeat("아주 긴 상태 이름 ", 8) + "\" as A\nA : " + strings.Repeat("긴 설명 글 ", 15) +
		"\nA : 둘째 설명\nA --> A : 자기\nA --> A : 또\nA --> B : " + strings.Repeat("긴 라벨 ", 10)
	multi := "stateDiagram-v2\n[*] --> A\nA --> B : 하나\nA --> B : 둘\nB --> A : 셋\nA --> A : 넷\nstate F <<fork>>\nstate J <<join>>\nstate C <<choice>>\n" +
		"B --> F\nF --> X\nF --> Y\nX --> J\nY --> J\nJ --> C\nC --> [*] : 끝\nC --> A : 다시"
	comps := "stateDiagram-v2\nstate 첫 {\n[*] --> 가\n가 --> 나\n나 --> [*]\n}\nstate \"둘째 묶음\" as 둘 {\n[*] --> 다\n다 --> 라 : 이음\n}\n나 --> 다\n라 --> 끝\n[*] --> 가\nstate 셋 {\n마\n}"
	srcs = append(srcs, one, many.String(), long, multi, comps)
	for _, d := range []string{"TB", "BT", "LR", "RL"} {
		for _, s := range []string{multi, comps, long} {
			srcs = append(srcs, strings.Replace(s, "stateDiagram-v2\n", "stateDiagram-v2\ndirection "+d+"\n", 1))
		}
	}
	for k, src := range srcs {
		c, _, err := Parse(src)
		if err != nil {
			t.Fatalf("%d: %v\n%s", k, err, src)
		}
		l, err := layout(c)
		if err != nil {
			t.Fatalf("%d: %v", k, err)
		}
		fail := func(f string, a ...any) { t.Errorf("그림 %d: %s\n%s", k, fmt.Sprintf(f, a...), src) }
		all := rect{0, 0, l.W, l.H}
		boxes := make([]rect, len(c.States))
		for i, b := range l.boxes {
			boxes[i] = rect{l.at[i].X - b.w/2, l.at[i].Y - b.h/2, l.at[i].X + b.w/2, l.at[i].Y + b.h/2}
			if !all.contains(layered.Pt{X: boxes[i].x0, Y: boxes[i].y0}, 0.01) || !all.contains(layered.Pt{X: boxes[i].x1, Y: boxes[i].y1}, 0.01) {
				fail("상태 %d 가 그림 밖", i)
			}
			for j := 0; j < i; j++ {
				if boxes[i].overlaps(boxes[j], 0.5) {
					fail("상태 %d·%d 겹침", i, j)
				}
			}
			if (b.kind == Fork || b.kind == Join) && (b.w > b.h) != l.vert {
				fail("갈래·합류 막대 %d 의 방향이 흐름과 맞지 않음(%v×%v)", i, b.w, b.h)
			}
		}
		for ci, cb := range l.clusters {
			if !cb.Show {
				fail("묶음 %d 가 보이지 않음", ci)
				continue
			}
			cr := rect{cb.X, cb.Y, cb.X + cb.W, cb.Y + cb.H}
			if !all.contains(layered.Pt{X: cr.x0, Y: cr.y0}, 0.01) || !all.contains(layered.Pt{X: cr.x1, Y: cr.y1}, 0.01) {
				fail("묶음 %d 가 그림 밖", ci)
			}
			for i, st := range c.States {
				inside := cr.contains(layered.Pt{X: boxes[i].x0, Y: boxes[i].y0}, 0.01) && cr.contains(layered.Pt{X: boxes[i].x1, Y: boxes[i].y1}, 0.01)
				if st.Cluster == ci && !inside {
					fail("상태 %d 가 제 묶음 %d 밖", i, ci)
				}
				if st.Cluster != ci && boxes[i].overlaps(cr, 0.5) {
					fail("상태 %d 가 다른 묶음 %d 와 겹침", i, ci)
				}
			}
			title := rect{cb.X, cb.Y, cb.X + cb.W, cb.Y + titleHt(cb.Title)}
			for i := range c.States {
				if boxes[i].overlaps(title, 0.5) {
					fail("상태 %d 가 묶음 %d 의 제목 칸과 겹침", i, ci)
				}
			}
		}
		var labels []rect
		for i, e := range l.edges {
			for _, p := range e.Pts {
				if !all.contains(p, 0.01) {
					fail("전이 %d 의 점 %v 가 그림 밖", i, p)
				}
			}
			if len(e.Label) > 0 {
				lr := rect{e.LC.X - e.LW/2, e.LC.Y - e.LH/2, e.LC.X + e.LW/2, e.LC.Y + e.LH/2}
				if !all.contains(layered.Pt{X: lr.x0, Y: lr.y0}, 0.01) || !all.contains(layered.Pt{X: lr.x1, Y: lr.y1}, 0.01) {
					fail("라벨 %d 가 그림 밖", i)
				}
				for j, b := range boxes {
					if lr.overlaps(b, 0.5) {
						fail("라벨 %d 와 상태 %d 겹침", i, j)
					}
				}
				for j, o := range labels {
					if lr.overlaps(o, 0.5) {
						fail("라벨 %d 와 앞의 라벨 %d 겹침", i, j)
					}
				}
				labels = append(labels, lr)
			}
			tr := c.Trans[i]
			tip := e.Pts[len(e.Pts)-1]
			if d := shapeDist(l.boxes[tr.To], l.at[tr.To], tip); math.Abs(d) > 0.15 {
				fail("전이 %d 의 화살촉 끝이 대상 %d 테두리에서 %.2f 떨어짐", i, tr.To, d)
			}
			if d := shapeDist(l.boxes[tr.From], l.at[tr.From], e.Pts[0]); math.Abs(d) > 0.15 {
				fail("전이 %d 의 시작이 상태 %d 테두리에서 %.2f 떨어짐", i, tr.From, d)
			}
		}
		// 그리는 차례: 묶음 → 모든 전이 선 → 화살촉 → (바탕 rect → 글자) 짝 → 상태
		svg := drawSVG(c, l)
		last := func(s string) int { return strings.LastIndex(svg, s) }
		first := func(s string) int { return strings.Index(svg, s) }
		if len(c.Comps) > 0 && last(`class="st-cl-t"`) > first(`class="st-tr"`) && len(c.Trans) > 0 {
			fail("묶음이 전이보다 뒤에 그려짐")
		}
		if len(c.Trans) > 0 {
			if last(`class="st-tr"`) > first(`class="st-head"`) || last(`class="st-head"`) > first(`class="st-nodes"`) {
				fail("전이 선·화살촉·상태의 그리는 차례가 다름")
			}
			if n := strings.Count(svg, `class="st-head"`); n != len(c.Trans) {
				fail("화살촉 %d 개, 전이 %d 개", n, len(c.Trans))
			}
			// 그려진 화살촉(st-head path)의 꼭짓점 셋을 SVG 에서 직접 읽는다(리뷰 1회차 [3] — 배치 점만 보면 그리기의 회귀를
			// 놓친다): 끝점은 대상 모양 테두리에, 세 점 모두 viewBox 안.
			heads := reHead.FindAllStringSubmatch(svg, -1)
			for i, h := range heads {
				if i >= len(c.Trans) {
					break
				}
				var v [6]float64
				for k := range v {
					v[k], _ = strconv.ParseFloat(h[k+1], 64)
				}
				tr := c.Trans[i]
				if d := shapeDist(l.boxes[tr.To], l.at[tr.To], layered.Pt{X: v[0], Y: v[1]}); math.Abs(d) > 0.15 {
					fail("그려진 화살촉 %d 의 끝이 대상 %d 테두리에서 %.2f 떨어짐", i, tr.To, d)
				}
				for k := 0; k < 6; k += 2 {
					if !all.contains(layered.Pt{X: v[k], Y: v[k+1]}, 0.05) {
						fail("그려진 화살촉 %d 의 꼭짓점 (%v, %v) 이 그림 밖", i, v[k], v[k+1])
					}
				}
			}
			if len(heads) != len(c.Trans) {
				fail("읽은 화살촉 path %d 개, 전이 %d 개", len(heads), len(c.Trans))
			}
			if len(labels) > 0 {
				if last(`class="st-head"`) > first(`class="st-lbg"`) || last(`class="st-lbl"`) > first(`class="st-nodes"`) {
					fail("라벨의 그리는 차례가 다름")
				}
				for _, m := range regexp.MustCompile(`<rect class="st-lbg"[^>]*></rect>(<text class="st-lbl")?`).FindAllStringSubmatch(svg, -1) {
					if m[1] == "" {
						fail("라벨 바탕 rect 바로 뒤에 글자가 없음")
					}
				}
			}
		}
		if t.Failed() {
			return
		}
	}
}

// reHead 는 화살촉 path 의 꼭짓점 셋이다(arrowHead 의 "M x y L x y L x y Z" 꼴).
var reHead = regexp.MustCompile(`<path class="st-head" d="M(-?[\d.]+) (-?[\d.]+) L(-?[\d.]+) (-?[\d.]+) L(-?[\d.]+) (-?[\d.]+) Z"`)

// 클래스는 모두 st- 접두사(앱 CSS 충돌), dist css 의 .st- 선택자는 .diagram-svg 문맥 안(er 와 같은 규칙), 다른 그림 종류의
// 접두사(gt-·tl-·sq-·er-)·흐름도 옛 클래스와 겹치지 않는다.
func TestClassCollisions(t *testing.T) {
	svg, _ := drawn(t, "stateDiagram-v2\naccDescr: 설명\nstate F <<fork>>\nstate J <<join>>\nstate C <<choice>>\n[*] --> A\nA : 머리\nA : 아래\n"+
		"A --> F : r\nF --> J\nJ --> C\nC --> [*]\nstate 묶음 {\n[*] --> B\n}\nB --> B : 자기")
	for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		for _, w := range strings.Fields(m[1]) {
			if w != "diagram-svg" && !strings.HasPrefix(w, "st-") {
				t.Errorf("상태도 클래스 %q 에 st- 접두사가 없음", w)
			}
		}
	}
	files, err := filepath.Glob("../../web/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("dist css 를 못 찾음: %v", err)
	}
	used := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if regexp.MustCompile(`\.st-[a-z]`).MatchString(line) && !strings.Contains(line, ".diagram-svg") {
				t.Errorf("%s:%d — .st- 선택자가 .diagram-svg 문맥 밖", f, i+1)
			}
			for _, m := range regexp.MustCompile(`\.(st-[a-z-]+)`).FindAllStringSubmatch(line, -1) {
				used[m[1]] = true
			}
		}
	}
	groups := map[string]bool{"st-cls": true, "st-trs": true, "st-heads": true, "st-lbls": true, "st-nodes": true}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		for _, w := range strings.Fields(m[1]) {
			seen[w] = true
			if strings.HasPrefix(w, "st-") && !used[w] && !groups[w] {
				t.Errorf("클래스 %q 의 CSS 규칙이 없음(색은 테마 토큰)", w)
			}
		}
	}
	for _, w := range []string{"st-cl", "st-cl-h", "st-cl-t", "st-tr", "st-head", "st-lbg", "st-lbl", "st-node", "st-div", "st-name", "st-desc",
		"st-start", "st-end", "st-end-in", "st-choice", "st-bar"} {
		if !seen[w] {
			t.Errorf("견본 그림에 클래스 %q 가 없음", w)
		}
	}
}

// 사용자 글자는 이스케이프되고 SVG 는 허용 목록의 요소만 쓴다(mdoc/security_test.go 가 문서 전체로 다시 본다).
func TestSVGEscapeAndElements(t *testing.T) {
	svg, _ := drawn(t, "stateDiagram-v2\naccTitle: a\" onload=\"x\naccDescr: #lt;/title#gt;#lt;script#gt;\nA : #lt;script#gt;\nA --> B : #lt;img#gt; #amp;\nstate \"#lt;b#gt;\" as 묶음 {\nC\n}")
	if strings.Contains(svg, "<script") || strings.Contains(svg, "<img") || strings.Contains(svg, "<b>") || strings.Contains(svg, `onload="`) {
		t.Errorf("이스케이프되지 않은 글자:\n%s", svg)
	}
	for _, m := range regexp.MustCompile(`<([a-z]+)`).FindAllStringSubmatch(svg, -1) {
		switch m[1] {
		case "svg", "g", "rect", "path", "line", "circle", "text", "tspan", "title":
		default:
			t.Errorf("허용 목록 밖 요소 %q", m[1])
		}
	}
	for _, m := range regexp.MustCompile(` d="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		if !regexp.MustCompile(`^[MLCAZ0-9 .-]+$`).MatchString(m[1]) {
			t.Errorf("경로 d 의 꼴이 허용 목록 밖 %q", m[1])
		}
	}
}

// gateSample 은 출력 글자 필드를 모두 채우는 원문이다(관문 시험이 쓴다).
const gateSample = "---\ntitle: 캡션\n---\nstateDiagram-v2\naccTitle: 제목\naccDescr: 설명\nA : 설명\nA --> B : 라벨\nstate \"묶음\" as M {\nC\n}"

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다 — chart 에서 닿는 구조체의
// 내보낸 string·[]string 필드(reflect)와 eachText 가 넘긴 필드 이름을 견준다. 모델에 글 필드를 더하고 관문에 넣지 않으면 여기서 멈춘다.
// 이 시험은 필드 목록의 덮음만 본다 — 그 값이 최종 SVG 에 보이는 글자 그대로 나가는지(출력 보존)는 XML 파서로 읽는 시험
// (TestEntityPolicyTable·TestAccAndWrapSVG·TestAriaLabelTab·TestWrapOutput·TestCaptionOutput)이 본다.
func TestGateCoversAllText(t *testing.T) {
	want := map[string]bool{}
	seen := map[reflect.Type]bool{}
	var walk func(rt reflect.Type)
	walk = func(rt reflect.Type) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			switch {
			case f.Type.Kind() == reflect.String, f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String:
				want[rt.Name()+"."+f.Name] = true
			default:
				walk(f.Type)
			}
		}
	}
	walk(reflect.TypeOf(chart{}))
	c, _, err := Parse(gateSample)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	_ = c.eachText(func(field, _, _ string, _ int) error { got[field] = true; return nil })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v, 모델의 글 필드 %v", got, want)
	}
}

// TestGateMatchesFieldChecks 는 관문이 개별 검사(label·accSet 의 xmlOK)와 같은 줄 번호·문구를 내는지 본다 — 받은 모델의 값에
// U+FFFE 를 넣어 gate 를 부른 결과와, 같은 값을 원문에 넣어 Parse 가 낸 오류를 견준다.
func TestGateMatchesFieldChecks(t *testing.T) {
	const bad = "x\ufffey"
	for _, c := range []struct {
		name string
		src  string
		set  func(c *chart)
	}{
		{"상태 설명", "stateDiagram-v2\nB\nA : %s", func(c *chart) { c.States[1].Descr[0] = bad }},
		{"따옴표 설명", "stateDiagram-v2\nB\nstate \"%s\" as A", func(c *chart) { c.States[1].Descr[0] = bad }},
		{"전이 라벨", "stateDiagram-v2\nB\nA --> B : %s", func(c *chart) { c.Trans[0].Label = bad }},
		{"복합 상태 설명", "stateDiagram-v2\nB\nstate \"%s\" as M {\nA\n}", func(c *chart) { c.Comps[0].Title = bad }},
		{"accTitle", "stateDiagram-v2\nB\naccTitle: %s", func(c *chart) { c.AccTitle = bad }},
		{"accDescr", "stateDiagram-v2\nB\naccDescr: %s", func(c *chart) { c.AccDescr = bad }},
		{"accDescr 블록", "stateDiagram-v2\nB\naccDescr {\n%s\n}", func(c *chart) { c.AccDescr = bad }},
	} {
		_, _, perr := Parse(fmt.Sprintf(c.src, bad))
		var pe *Error
		if !errors.As(perr, &pe) {
			t.Fatalf("%s: Parse 가 거부하지 않음 %v", c.name, perr)
		}
		m, _, err := Parse(fmt.Sprintf(c.src, "x"))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		c.set(m)
		var ge *Error
		if !errors.As(m.gate(), &ge) || ge.Line != pe.Line || ge.Msg != pe.Msg {
			t.Errorf("%s: 관문 %v, 개별 검사 %d:%s", c.name, ge, pe.Line, pe.Msg)
		}
	}
}
