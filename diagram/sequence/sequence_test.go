package sequence

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram"
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
		t.Errorf("Render 오류 %d:%q, 기대 %d 에 %q", de.Line, de.Msg, line, msg)
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

func TestParseStatements(t *testing.T) {
	// 머리 줄 — sequenceDiagram(대소문자 무시 — 원본 lexer case-insensitive)
	for _, h := range []string{"sequenceDiagram", "SequenceDiagram", "SEQUENCEDIAGRAM"} {
		svg, issues := drawn(t, h+"\nA->>B: 안녕")
		if len(issues) != 0 {
			t.Errorf("머리 %q: 경고 %+v", h, issues)
		}
		if !strings.Contains(svg, ">안녕<") {
			t.Errorf("머리 %q: 메시지가 없음\n%s", h, svg)
		}
	}
	errAt(t, "sequenceDiagram LR\nA->>B: x", 1, "sequenceDiagram 이 아님") // 방향 낱말은 없다

	// title — 그림 안 제목(대소문자 무시). title: 옛 표기도 같다. <br>·줄바꿈은 한 줄로 뭉친다.
	for _, h := range []string{"title 주문 흐름", "TITLE 주문 흐름", "title: 주문 흐름", "Title: 주문 흐름"} {
		svg, issues := drawn(t, "sequenceDiagram\n"+h+"\nA->>B: 안녕")
		if len(issues) != 0 || !strings.Contains(svg, `class="sq-title"`) || !strings.Contains(svg, ">주문 흐름<") {
			t.Errorf("%q: 경고 %+v·제목 없음\n%s", h, issues, svg)
		}
	}
	if r, err := Render("---\ntitle: 캡션\n---\nsequenceDiagram\nA->>B: 안녕"); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(r.SVG, `aria-label="캡션"`) {
		t.Errorf("frontmatter title 이 캡션이 아님\n%s", r.SVG)
	}

	// 참여자 — participant·actor·as·선언 없이 메시지에 처음 나온 이름(순서대로)
	svg, _ := drawn(t, "sequenceDiagram\nparticipant A as 앨리스\nactor B as 밥\nA->>B: 안녕")
	for _, s := range []string{">앨리스<", ">밥<", `class="sq-fig-h"`} {
		if !strings.Contains(svg, s) {
			t.Errorf("참여자: %s 없음\n%s", s, svg)
		}
	}
	if strings.Contains(svg, ">A<") || strings.Contains(svg, ">B<") {
		t.Errorf("별명이 안 쓰임\n%s", svg)
	}
	// 사람 모양(actor) 은 위·아래 상자 모두에 하나씩
	if n := strings.Count(svg, `class="sq-fig-h"`); n != 2 {
		t.Errorf("사람 모양 %d 개, 기대 2(위·아래)\n%s", n, svg)
	}
	// 암묵 참여자 — 메시지에 처음 나온 순서(보낸이→받은이)
	c, _, err := Parse("sequenceDiagram\nX->>Y: 가\nY->>Z: 나")
	if err != nil {
		t.Fatal(err)
	}
	if got := idsOf(c); got != "X,Y,Z" {
		t.Errorf("암묵 참여자 순서 %q, 기대 X,Y,Z", got)
	}
	// 다시 적기 — 별명 있는 것은 이기고, 별명 없는 것은 아무 것도 바꾸지 않는다(원본 addActor)
	c, _, err = Parse("sequenceDiagram\nparticipant A as 앨리스\nparticipant A\nA->>A: x")
	if err != nil {
		t.Fatal(err)
	}
	if c.Actors[0].Name != "앨리스" {
		t.Errorf("별명 없는 다시 적기가 별명을 덮음: %q", c.Actors[0].Name)
	}
	c, _, err = Parse("sequenceDiagram\nparticipant A\nparticipant A as 밥\nA->>A: x")
	if err != nil || c.Actors[0].Name != "밥" {
		t.Errorf("별명 있는 다시 적기: %q %v", c.Actors[0].Name, err)
	}
	// 이름에 공백 — 원본은 「Alice Smith」로 읽지만 허용 꼴 밖이다(itda-hyve#208 허용 목록 — 표시 이름은 as 별명으로)
	errAt(t, "sequenceDiagram\nparticipant Alice Smith\nA->>B: x", 2, "참여자 선언은")
	c, _, err = Parse("sequenceDiagram\nparticipant AlS as Alice Smith\nAlS->>B: x")
	if err != nil || c.Actors[0].Name != "Alice Smith" {
		t.Errorf("as 별명 이름: %+v %v", c, err)
	}

	// 화살표 10종 — 실선/점선 × 끝 모양
	for _, tc := range []struct {
		tok string
		dot bool
		hd  string
	}{
		{"->>", false, `class="sq-head"`}, {"-->>", true, `class="sq-head"`},
		{"->", false, ""}, {"-->", true, ""},
		{"-x", false, `class="sq-cross"`}, {"--x", true, `class="sq-cross"`},
		{"-)", false, `class="sq-head"`}, {"--)", true, `class="sq-head"`},
		{"<<->>", false, `class="sq-head"`}, {"<<-->>", true, `class="sq-head"`},
	} {
		src := "sequenceDiagram\nA" + tc.tok + "B: 글"
		svg, issues := drawn(t, src)
		if len(issues) != 0 {
			t.Errorf("화살표 %s: 경고 %+v", tc.tok, issues)
			continue
		}
		if got := strings.Contains(svg, "sq-dt"); got != tc.dot {
			t.Errorf("화살표 %s: 점선 %v, 기대 %v\n%s", tc.tok, got, tc.dot, svg)
		}
		if tc.hd != "" && !strings.Contains(svg, tc.hd) {
			t.Errorf("화살표 %s: 화살촉 없음\n%s", tc.tok, svg)
		}
		if tc.hd == "" && strings.Contains(svg, "sq-head") {
			t.Errorf("화살표 %s: 화살촉이 있음\n%s", tc.tok, svg)
		}
	}
	// 양방향은 두 화살촉
	svg, _ = drawn(t, "sequenceDiagram\nA<<->>B: 글")
	if n := strings.Count(svg, `class="sq-head"`); n != 2 {
		t.Errorf("양방향 화살촉 %d 개, 기대 2\n%s", n, svg)
	}
	// 자기 메시지 — 되돌아오는 꺾인 선
	svg, _ = drawn(t, "sequenceDiagram\nA->>A: 스스로")
	if !strings.Contains(svg, "<path class=\"sq-msg\"") || !strings.Contains(svg, ">스스로<") {
		t.Errorf("자기 메시지가 안 그려짐\n%s", svg)
	}
	// 화살표 앞뒤 빈칸은 읽고, 이름 안의 -·빈칸은 허용 꼴 밖이다(원본은 「A-B」「C D」로 읽는다 — 의도된 축소)
	svg, _ = drawn(t, "sequenceDiagram\nA ->> + B : 글")
	if !strings.Contains(svg, ">A<") || !strings.Contains(svg, ">B<") || !strings.Contains(svg, ">글<") {
		t.Errorf("화살표 앞뒤 빈칸 처리가 다름\n%s", svg)
	}
	errAt(t, "sequenceDiagram\nA-B ->> C: 글", 2, "참여자 이름")
	errAt(t, "sequenceDiagram\nA ->> C D: 글", 2, "참여자 이름")
	// 빈 글 메시지 — 화살표만
	svg, issues := drawn(t, "sequenceDiagram\nA->>B:")
	if len(issues) != 0 || !strings.Contains(svg, "sq-msg") {
		t.Errorf("빈 글 메시지: 경고 %+v\n%s", issues, svg)
	}

	// 활성 — activate·deactivate·메시지 끝 +·-
	svg, _ = drawn(t, "sequenceDiagram\nA->>+B: 요청\nB-->>-A: 응답")
	if n := strings.Count(svg, `class="sq-act"`); n != 1 {
		t.Errorf("활성 막대 %d 개, 기대 1\n%s", n, svg)
	}
	// 겹친 활성 — 옆으로 비켜 쌓인다(두 막대의 x 가 다르다)
	svg, _ = drawn(t, "sequenceDiagram\nA->>+B: 1\nB->>+B: 2\nB-->>-B: 3\nB-->>-A: 4")
	acts := allRects(svg, "sq-act")
	if len(acts) != 2 {
		t.Fatalf("겹친 활성 막대 %d 개, 기대 2\n%s", len(acts), svg)
	}
	if acts[0].x == acts[1].x {
		t.Errorf("겹친 활성이 같은 자리\n%s", svg)
	}
	// 닫히지 않은 activate 는 막대를 그리지 않는다(원본도 ACTIVE_END 에만 그린다)
	svg, _ = drawn(t, "sequenceDiagram\nA->>B: x\nactivate B\nA->>B: y")
	if n := strings.Count(svg, `class="sq-act"`); n != 0 {
		t.Errorf("닫히지 않은 활성이 그려짐\n%s", svg)
	}
	errAt(t, "sequenceDiagram\nA->>B: x\ndeactivate B", 3, "활성되지 않은")
	errAt(t, "sequenceDiagram\nA->>+B: x\nB-->>-A: y\nB-->>-A: z", 4, "활성되지 않은")
	errAt(t, "sequenceDiagram\nactivate Q", 2, "참여자")
	errAt(t, "sequenceDiagram\nA->>B: x\ndeactivate Q", 3, "참여자")

	// 메모 — left of·right of·over·over A,B(대소문자 무시)
	svg, _ = drawn(t, "sequenceDiagram\nA->>B: x\nNote left of A: 왼쪽\nNOTE RIGHT OF B: 오른쪽\nnote over A: 위")
	for _, s := range []string{">왼쪽<", ">오른쪽<", ">위<"} {
		if !strings.Contains(svg, s) {
			t.Errorf("메모 %s 없음\n%s", s, svg)
		}
	}
	if n := strings.Count(svg, `class="sq-note"`); n != 3 {
		t.Errorf("메모 상자 %d 개, 기대 3\n%s", n, svg)
	}
	svg, _ = drawn(t, "sequenceDiagram\nA->>B: x\nNote over A,B: 둘에 걸침")
	if !strings.Contains(svg, ">둘에 걸침<") {
		t.Errorf("over A,B 메모가 없음\n%s", svg)
	}
	errAt(t, "sequenceDiagram\nNote left of A,B: x", 2, "참여자 하나만")
	errAt(t, "sequenceDiagram\nNote over A", 2, "메모는")
	errAt(t, "sequenceDiagram\nNote above A: x", 2, "메모는")

	// 묶음 — loop·opt·alt/else·par/and·critical/option·break·rect(중첩 포함)
	svg, _ = drawn(t, "sequenceDiagram\nA->>B: x\n"+
		"loop 매번\nopt 선택\nA->>B: y\nend\nend\n"+
		"alt 있음\nA->>B: 1\nelse 없음\nA->>B: 2\nend\n"+
		"par 이것\nA->>B: 3\nand 저것\nA->>B: 4\nend\n"+
		"critical 중요\nA->>B: 5\noption 대안\nA->>B: 6\nend\n"+
		"break 끊김\nA->>B: 7\nend\n"+
		"rect rgb(1,2,3)\nA->>B: 8\nend")
	for _, s := range []string{">loop<", ">[매번]<", ">opt<", ">[선택]<", ">alt<", ">[있음]<", ">[없음]<", ">par<",
		">critical<", ">[대안]<", ">break<", ">[끊김]<", ">1<", ">8<"} { // 조건 글은 원본처럼 대괄호를 감싼다(수정 확인 5)
		if !strings.Contains(svg, s) {
			t.Errorf("묶음 %s 없음\n%s", s, svg)
		}
	}
	if n := strings.Count(svg, `class="sq-div"`); n != 3 {
		t.Errorf("구획 점선 %d 개, 기대 3(else·and·option)\n%s", n, svg)
	}
	if n := strings.Count(svg, `class="sq-frame"`); n != 6 {
		t.Errorf("묶음 테두리 %d 개, 기대 6\n%s", n, svg)
	}
	if !strings.Contains(svg, `class="sq-rect"`) {
		t.Errorf("rect 띠가 없음\n%s", svg)
	}
	// 빈 머리 글 묶음도 그린다(원본과 같다)
	drawn(t, "sequenceDiagram\nA->>B: x\nloop\nA->>B: y\nend")
	// 오류 — 짝·자리
	errAt(t, "sequenceDiagram\nloop x\nA->>B: y", 2, "닫히지 않음")
	errAt(t, "sequenceDiagram\nA->>B: x\nend", 3, "end 짝이 없음")
	errAt(t, "sequenceDiagram\nloop x\nA->>B: y\nend\nelse 나중", 5, "묶음 밖에")
	errAt(t, "sequenceDiagram\nloop x\nelse 잘못\nend", 3, "alt 안에")
	errAt(t, "sequenceDiagram\nalt x\nA->>B: y\nand 잘못\nend", 4, "par 안에")
	errAt(t, "sequenceDiagram\ncritical x\noption o\nend", 0, "참여자가 없음") // 문법은 되고, 그릴 참여자가 없다
	errAt(t, "sequenceDiagram\nend x", 2, "end 뒤에")
	errAt(t, "sequenceDiagram\nloop x\nA->>B: y\nend x", 4, "end 뒤에")

	// 번호 — autonumber·autonumber 10·autonumber 10 5·autonumber off(다시 켜면 세던 수에 이어)
	for _, tc := range []struct {
		src  string
		want []string
	}{
		{"sequenceDiagram\nautonumber\nA->>B: 1\nA->>B: 2", []string{">1<", ">2<"}},
		{"sequenceDiagram\nautonumber 10\nA->>B: a\nA->>B: b", []string{">10<", ">11<"}},
		{"sequenceDiagram\nautonumber 10 5\nA->>B: a\nA->>B: b", []string{">10<", ">15<"}},
		{"sequenceDiagram\nautonumber\nA->>B: a\nautonumber off\nA->>B: b\nautonumber\nA->>B: c", []string{">1<", ">3<"}}, // off 구간도 센다(원본 — 리뷰 1회차 [2], 기대값을 바로잡음)
	} {
		svg, issues := drawn(t, tc.src)
		if len(issues) != 0 {
			t.Errorf("번호 %q: 경고 %+v", tc.src, issues)
		}
		for _, w := range tc.want {
			if !strings.Contains(svg, w) {
				t.Errorf("번호 %q: %s 없음\n%s", tc.src, w, svg)
			}
		}
	}
	// 번호 없는 기본 — 동그라미가 없다
	svg, _ = drawn(t, "sequenceDiagram\nA->>B: x")
	if strings.Contains(svg, "sq-num") {
		t.Errorf("번호가 기본으로 켜짐\n%s", svg)
	}
	// 메모·활성은 번호를 세지 않는다(원본과 같다)
	svg, _ = drawn(t, "sequenceDiagram\nautonumber\nA->>B: a\nNote over A: n\nA->>B: b")
	if !strings.Contains(svg, ">1<") || !strings.Contains(svg, ">2<") || strings.Contains(svg, ">3<") {
		t.Errorf("메모가 번호를 셈\n%s", svg)
	}

	// 무리(box) — 참여자 띠·이름, 색 값은 쓰지 않고 경고
	svg, issues = drawn(t, "sequenceDiagram\nbox 서버\nparticipant S1\nparticipant S2\nend\nS1->>S2: 요청")
	if len(issues) != 0 {
		t.Errorf("box: 경고 %+v", issues)
	}
	if !strings.Contains(svg, `class="sq-box"`) || !strings.Contains(svg, ">서버<") {
		t.Errorf("box 띠·이름 없음\n%s", svg)
	}
	_, issues = drawn(t, "sequenceDiagram\nbox rgb(0,0,0) 서버\nparticipant S1\nend\nS1->>S1: x")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "box 의 색") {
		t.Errorf("box 색 경고가 다름: %+v", issues)
	}
	_, issues = drawn(t, "sequenceDiagram\nbox grey 서버\nparticipant S1\nend\nS1->>S1: x")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "box 의 색") {
		t.Errorf("box 색 이름 경고가 다름: %+v", issues)
	}
	// box 안에는 참여자 선언만(원본도 같다)
	errAt(t, "sequenceDiagram\nbox b\nA->>B: x\nend", 3, "participant·actor 선언만")
	errAt(t, "sequenceDiagram\nbox b\nparticipant A\nend\nbox c\nparticipant A as 다시\nend", 6, "다른 box")
	// 빈 box 는 그리지 않고 경고
	svg, issues = drawn(t, "sequenceDiagram\nbox 빈\nend\nA->>B: x")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "빈 box") {
		t.Errorf("빈 box 경고가 다름: %+v", issues)
	}
	if strings.Contains(svg, "빈") {
		t.Errorf("빈 box 를 그림\n%s", svg)
	}

	// `;` 문장 구분은 받지 않는다(한 줄에 문장 하나 — 허용 목록)
	errAt(t, "sequenceDiagram\nA->>B: 첫째; B->>A: 둘째", 2, "`;`")
	errAt(t, "sequenceDiagram\nparticipant A;", 2, "`;`")

	// 줄바꿈 <br> — 세 꼴
	for _, br := range []string{"<br>", "<br/>", "<br />"} {
		svg, _ = drawn(t, "sequenceDiagram\nA->>B: 첫째 줄"+br+"둘째 줄")
		if !strings.Contains(svg, ">첫째 줄<") || !strings.Contains(svg, ">둘째 줄<") {
			t.Errorf("%s 줄바꿈이 안 풀림\n%s", br, svg)
		}
	}

	// Mermaid 엔티티 — 엔티티는 그림이 되고(#…;), 풀린 글자가 문법 글자여도 다시 읽히지 않는다
	svg, _ = drawn(t, "sequenceDiagram\nA->>B: 비율 #35;")
	if !strings.Contains(svg, ">비율 #<") {
		t.Errorf("엔티티가 안 풀림\n%s", svg)
	}
	errAt(t, "sequenceDiagram\nA#58;B->>C: x", 2, "참여자 이름") // 이름의 엔티티는 허용 꼴 밖(표시 이름은 as 별명으로)
	svg, _ = drawn(t, "sequenceDiagram\nparticipant A as 가#58;나\nA->>A: x")
	if !strings.Contains(svg, ">가:나<") {
		t.Errorf("별명의 엔티티가 안 풀림\n%s", svg)
	}
	// 남은 # 은 원문 + 경고(원본 lexer 가 주석으로 버리는 자리 — timeline ② 와 같은 결정)
	errAt(t, "sequenceDiagram\nA->>B: 메모 #1", 2, "# 부터는")
	errAt(t, "sequenceDiagram\n# 메모\nA->>B: x", 2, "# 부터는")
	// style/classDef 줄의 끝 ; 자르기(원본 전처리 — timeline ② TestReviewR3 와 같은 경계): ; 이 잘리며 남은 itda-hyve#35 는
	// 엔티티꼴이 아니므로 이 판은 그 줄을 원문 + 경고로 돌린다(원본은 TXT 가 # 에서 잘라 앞부분만 그린다).
	errAt(t, "sequenceDiagram\nA->>B: style:x#35;\nB->>A: 답", 2, "# 부터는")
	errAt(t, "sequenceDiagram\nA->>B: classDef:y#ff0000;", 2, "# 부터는")
	_, issues = drawn(t, "sequenceDiagram\n%% # 이것은 주석\nA->>B: x")
	if len(issues) != 0 {
		t.Errorf("%% 주석 안의 # 이 경고됨: %+v", issues)
	}
	// 내부 표식 문자 — 원문에 있으면 그리지 않는다(② 와 같은 결정)
	errAt(t, "sequenceDiagram\nA->>B: x#58; y ¶ß z", 2, "내부 표식 문자")

	// 쓰지 않는 줄(경고 한 번) — %%{init}%%·frontmatter 설정·링크 메뉴
	svg, issues = drawn(t, "%%{init: {\"theme\":\"dark\"}}%%\nsequenceDiagram\nA->>B: x")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "설정 지시") {
		t.Errorf("%%{init}%% 경고가 다름: %+v", issues)
	}
	_, issues = drawn(t, "---\nconfig:\n  theme: dark\n---\nsequenceDiagram\nA->>B: x")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "config") {
		t.Errorf("frontmatter 설정 경고가 다름: %+v", issues)
	}
	svg, issues = drawn(t, "sequenceDiagram\nlink A: 문서 @ https://x.example\nA->>B: x")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "링크 메뉴") {
		t.Errorf("링크 메뉴 경고가 다름: %+v", issues)
	}
	if !strings.Contains(svg, ">x<") || strings.Contains(svg, "x.example") {
		t.Errorf("링크 메뉴 문장이 그림에 남거나 그림이 없음\n%s", svg)
	}
	for _, kw := range []string{"links", "properties", "details"} {
		_, issues = drawn(t, "sequenceDiagram\n"+kw+" A: {\"k\": \"v\"}\nA->>B: x")
		if len(issues) != 1 || !strings.Contains(issues[0].Msg, kw) {
			t.Errorf("%s 경고가 다름: %+v", kw, issues)
		}
	}

	// 접근성 — accTitle(aria-label)·accDescr(<title>). 키워드 대소문자 무시, 선언이 여럿이면 마지막, 블록형 200자 상한
	svg, _ = drawn(t, "sequenceDiagram\naccTitle: 순서\naccDescr: 흐름 설명\nA->>B: x")
	if !strings.Contains(svg, `aria-label="순서"`) || !strings.Contains(svg, "<title>흐름 설명</title>") {
		t.Errorf("접근성 글이 없음\n%s", svg)
	}
	svg, _ = drawn(t, "sequenceDiagram\nACCTITLE: 대문자\naccDescr {\n두 줄\n설명\n}\nA->>B: x")
	if !strings.Contains(svg, `aria-label="대문자"`) || !strings.Contains(svg, "<title>두 줄\n설명</title>") { // 블록 줄바꿈은 그대로(정책 B)
		t.Errorf("접근성 대소문자·블록이 안 읽힘\n%s", svg)
	}
	long := strings.Repeat("가", MaxLabel)
	svg, _ = drawn(t, "sequenceDiagram\naccDescr { "+long+" }\nA->>B: x")
	if !strings.Contains(svg, "<title>"+long+"</title>") {
		t.Errorf("accDescr 블록 200자가 안 읽힘\n%s", svg)
	}
	errAt(t, "sequenceDiagram\naccDescr { "+long+"가 }\nA->>B: x", 2, "accDescr 값이 200 자")
	errAt(t, "sequenceDiagram\naccDescr {\n"+long+"가\n}\nA->>B: x", 4, "accDescr 값이 200 자")

	// 미지원 — 반쪽 화살표·가운데 연결·create·destroy·par_over·참여자 설정
	errAt(t, "sequenceDiagram\nA-|\\B: x", 2, "반쪽 화살표")
	errAt(t, "sequenceDiagram\nA/|-B: x", 2, "반쪽 화살표")
	errAt(t, "sequenceDiagram\nA--|/B: x", 2, "반쪽 화살표")
	errAt(t, "sequenceDiagram\nA->>()B: x", 2, "가운데 연결")
	errAt(t, "sequenceDiagram\nA()->>B: x", 2, "가운데 연결")
	errAt(t, "sequenceDiagram\ncreate participant B\nA->>B: x", 2, "create")
	errAt(t, "sequenceDiagram\ndestroy participant B\nA->>B: x", 2, "destroy")
	errAt(t, "sequenceDiagram\npar_over x\nA->>B: y\nend", 2, "par_over")
	errAt(t, "sequenceDiagram\nparticipant A@{shape: database}\nA->>B: x", 2, "참여자 설정")

	// 오류 — 모양·자리
	errAt(t, "sequenceDiagram\nA->>B", 2, "콜론")
	errAt(t, "sequenceDiagram\nA->>B: x\n이것은 문장", 3, "알 수 없는 문장")
	errAt(t, "sequenceDiagram\nA->>B(x): y", 2, "참여자 이름")
	errAt(t, "sequenceDiagram\nA-->->B: x", 2, "참여자 이름")
	errAt(t, "sequenceDiagram\ntitle\nA->>B: x", 2, "제목이 없거나")
	errAt(t, "sequenceDiagram\ntitle:제목\nA->>B: x", 2, "제목이 없거나") // 옛 표기는 콜론 뒤 빈칸이 있어야 한다(원본 "title:"\s)
	errAt(t, "sequenceDiagram\nparticipant\nA->>B: x", 2, "참여자 선언은")
	errAt(t, "sequenceDiagram\nA->>B: x\nautonumber 10x", 3, "수")
	errAt(t, "sequenceDiagram\nA->>B: x\nlink", 3, "링크 메뉴는")
	errAt(t, "sequenceDiagram\nA->>B: x\nlink A {\"문서\": \"x\"}", 3, "링크 메뉴는") // 콜론 없는 꼴 — 원본은 이름을 「A {"문서"」로 읽는다
	errAt(t, "graph TD\nA --> B", 1, "sequenceDiagram 이 아님")
	errAt(t, "sequenceDiagram", 0, "참여자가 없음")
	errAt(t, "sequenceDiagram\naccDescr {\n풀리지 않음\nA->>B: x", 2, "닫히지 않음")
	errAt(t, "---\ntitle: [가\n---\nsequenceDiagram\nA->>B: x", 1, "YAML")

	// 주석·빈 줄
	drawn(t, "sequenceDiagram\n%% 메모\n\nA->>B: x")
}

func idsOf(c *chart) string {
	var ids []string
	for _, a := range c.Actors {
		ids = append(ids, a.ID)
	}
	return strings.Join(ids, ",")
}

func TestLimits(t *testing.T) {
	// 참여자 상한 — 50 은 그리고 51 은 원문
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	for i := 0; i < MaxActors; i++ {
		fmt.Fprintf(&b, "P%d->>P%d: x\n", i, i)
	}
	drawn(t, b.String())
	errAt(t, b.String()+"P99->>P98: y\n", MaxActors+2, "참여자가 50 개")

	// 메시지·메모 합계 상한 — 500 은 그리고 501 은 원문
	b.Reset()
	b.WriteString("sequenceDiagram\nA->>B: x\n")
	for i := 0; i < MaxEvents-1; i++ {
		fmt.Fprintf(&b, "A->>A: %d\n", i)
	}
	drawn(t, b.String())
	errAt(t, b.String()+"A->>A: 더\n", MaxEvents+2, "메시지·메모 합계가 500")

	// 묶음 상한 — 100 개는 그리고 101 은 원문
	b.Reset()
	b.WriteString("sequenceDiagram\nA->>B: x\n")
	for i := 0; i < MaxBlocks; i++ {
		b.WriteString("loop\nend\n")
	}
	drawn(t, b.String())
	errAt(t, b.String()+"loop\nend\n", 3+MaxBlocks*2, "묶음이 100 개")

	// 묶음 깊이 — 10 은 그리고 11 은 원문
	b.Reset()
	b.WriteString("sequenceDiagram\nA->>B: x\n")
	for i := 0; i < MaxDepth; i++ {
		b.WriteString("loop\n")
	}
	for i := 0; i < MaxDepth; i++ {
		b.WriteString("end\n")
	}
	drawn(t, b.String())
	b.Reset()
	b.WriteString("sequenceDiagram\nA->>B: x\n")
	for i := 0; i <= MaxDepth; i++ {
		b.WriteString("loop\n")
	}
	errAt(t, b.String(), 3+MaxDepth, "깊이가 10")

	// box 상한 — 20 은 그리고 21 은 원문
	b.Reset()
	b.WriteString("sequenceDiagram\n")
	for i := 0; i < MaxBoxes; i++ {
		fmt.Fprintf(&b, "box b%d\nparticipant P%d\nend\n", i, i)
	}
	b.WriteString("P0->>P1: x\n")
	drawn(t, b.String())
	errAt(t, b.String()+"box 넘침\nend\n", MaxBoxes*3+3, "box 가 20 개")

	// 라벨 상한
	long := strings.Repeat("가", MaxLabel)
	drawn(t, "sequenceDiagram\nA->>B: "+long) // 200 자는 그린다
	errAt(t, "sequenceDiagram\nA->>B: "+long+"가", 2, "라벨이 200 자")
	errAt(t, "sequenceDiagram\nparticipant A as "+long+"가", 2, "라벨이 200 자")

	// 원문 상한(공통)
	errAt(t, "sequenceDiagram\nA->>B: "+strings.Repeat("글 ", 5000), 0, "16KB")
}

// TestDeterminism 은 같은 입력을 두 번 그려 바이트가 같은지 본다(map 순회·환경 의존 없음).
func TestDeterminism(t *testing.T) {
	for i, src := range []string{
		"sequenceDiagram\nA->>B: 안녕",
		"sequenceDiagram\nautonumber\nloop 매번\nA->>+B: 요청\nB-->>-A: 응답\nend\nNote over A,B: 걸침",
		"sequenceDiagram\nactor U as " + strings.Repeat("아주 긴 고객 이름 ", 6) + "\nU->>U: " + strings.Repeat("아주 긴 메시지라 줄이 바뀐다 ", 4),
		"sequenceDiagram\nA<<->>B: 둘\nA--xB: 가위\nA-)B: 가는\nA->A: 나",
		"sequenceDiagram\nbox 무리\nparticipant X\nparticipant Y\nend\nalt 처음\nX->>Y: 1\nelse 나중\nY->>X: 2\nend",
	} {
		a, err := Render(src)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		b, _ := Render(src)
		if a.SVG != b.SVG {
			t.Errorf("%d: 두 번 렌더가 다름\n%s\n%s", i, a.SVG, b.SVG)
		}
	}
}

// ---- SVG 구조 도우미(timeline_test.go 와 같은 방식 — 패키지가 다르니 여기에 둔다)

type el struct {
	tag   string
	attrs map[string]string
}

func splitEls(svg string) []el {
	var out []el
	dec := func(s string) el {
		s = strings.TrimPrefix(s, "<")
		i := strings.IndexByte(s, ' ')
		if i < 0 {
			i = strings.IndexAny(s, ">/")
			if i < 0 {
				i = len(s)
			}
		}
		e := el{tag: s[:i], attrs: map[string]string{}}
		for _, m := range regexp.MustCompile(`([a-zA-Z0-9-]+)="([^"]*)"`).FindAllStringSubmatch(s, -1) {
			e.attrs[m[1]] = m[2]
		}
		return e
	}
	for _, part := range strings.Split(svg, "<") {
		if part == "" || strings.HasPrefix(part, "?") || strings.HasPrefix(part, "/") {
			continue
		}
		out = append(out, dec(part))
	}
	return out
}

type rectEl struct {
	cls        string
	x, y, w, h float64
}

func allRects(svg, cls string) []rectEl {
	var out []rectEl
	for _, e := range splitEls(svg) {
		if e.tag != "rect" || !strings.Contains(e.attrs["class"], cls) {
			continue
		}
		f := func(k string) float64 { v, _ := strconv.ParseFloat(e.attrs[k], 64); return v }
		out = append(out, rectEl{cls: e.attrs["class"], x: f("x"), y: f("y"), w: f("width"), h: f("height")})
	}
	return out
}

// checkLayout 은 SVG 하나의 배치 성질을 본다 — 모든 요소가 viewBox 안, 메시지 화살표 y 가 원문 순서대로 단조 증가,
// 메시지 글 상자·메모가 서로 겹치지 않음, 묶음 테두리가 안의 메시지·중첩 묶음을 감쌈, 활성 막대가 생명선 위,
// 생명선이 모든 글 바탕보다 먼저 그려지고(글 바탕이 선을 덮는다) 글 바탕 rect 바로 뒤에 글자가 온다.
func checkLayout(t *testing.T, name, svg string, ls ...*layout) {
	t.Helper()
	m := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("%s: viewBox 없음", name)
	}
	vw, _ := strconv.ParseFloat(m[1], 64)
	vh, _ := strconv.ParseFloat(m[2], 64)
	els := splitEls(svg)
	num := func(e el, k string) float64 {
		v, err := strconv.ParseFloat(e.attrs[k], 64)
		if err != nil {
			t.Fatalf("%s: <%s> 의 %s=%q 이 숫자가 아님", name, e.tag, k, e.attrs[k])
		}
		return v
	}
	var lifes, tbgs, notes, acts, frames []el
	var arrows []float64
	lifeX := map[float64]bool{}
	for _, e := range els {
		in := func(v, lim float64) {
			if v < -0.05 || v > lim+0.05 {
				t.Fatalf("%s: <%s class=%q> 좌표 %v 가 viewBox %v 밖\n%s", name, e.tag, e.attrs["class"], v, lim, svg)
			}
		}
		switch e.tag {
		case "svg", "title", "text", "tspan", "g":
		case "rect":
			x, y, w, h := num(e, "x"), num(e, "y"), num(e, "width"), num(e, "height")
			in(x, vw)
			in(y, vh)
			in(x+w, vw)
			in(y+h, vh)
			switch {
			case strings.Contains(e.attrs["class"], "sq-life"):
			case strings.Contains(e.attrs["class"], "sq-tbg"):
				tbgs = append(tbgs, e)
			case strings.Contains(e.attrs["class"], "sq-note"):
				notes = append(notes, e)
			case strings.Contains(e.attrs["class"], "sq-act"):
				acts = append(acts, e)
			case strings.Contains(e.attrs["class"], "sq-frame"):
				frames = append(frames, e)
			}
		case "line":
			x1, y1, x2, y2 := num(e, "x1"), num(e, "y1"), num(e, "x2"), num(e, "y2")
			in(x1, vw)
			in(x2, vw)
			in(y1, vh)
			in(y2, vh)
			cls := e.attrs["class"]
			switch {
			case strings.Contains(cls, "sq-life"):
				lifes = append(lifes, e)
				lifeX[x1] = true
			case strings.Contains(cls, "sq-msg"):
				arrows = append(arrows, y1)
				if y1 != y2 {
					t.Fatalf("%s: 메시지 선이 가로가 아님\n%s", name, svg)
				}
			}
		case "circle":
			in(num(e, "cx"), vw)
			in(num(e, "cy"), vh)
			in(num(e, "cx")+num(e, "r"), vw)
			in(num(e, "cx")-num(e, "r"), vw)
		case "path":
			coord := regexp.MustCompile(`-?[\d.]+`).FindAllString(strings.ReplaceAll(strings.ReplaceAll(
				strings.ReplaceAll(e.attrs["d"], "M", " "), "L", " "), "Z", " "), -1)
			for j, ns := range coord {
				v, _ := strconv.ParseFloat(ns, 64)
				if j%2 == 0 {
					in(v, vw)
				} else {
					in(v, vh)
				}
			}
		default:
			t.Fatalf("%s: 모르는 요소 <%s>", name, e.tag)
		}
	}
	if len(lifes) == 0 {
		t.Fatalf("%s: 생명선이 없음", name)
	}
	// 메시지 y 단조 증가(원문 순서 = 문서 순서)
	for i := 1; i < len(arrows); i++ {
		if arrows[i] < arrows[i-1]-0.05 {
			t.Fatalf("%s: 메시지 화살표 y 가 거꾸로(%d: %v < %v)\n%s", name, i, arrows[i], arrows[i-1], svg)
		}
	}
	// 글 바탕(sq-tbg)은 생명선·활성 막대·묶음 테두리·구획 점선보다 문서 순서에서 뒤다 — 바탕이 선·막대를 덮어
	// 글자를 지킨다(수정 1 — 조건 글이 활성 막대에 덮이던 것). 바탕 rect 바로 뒤에는 그 글자가 온다.
	lastUnder := -1
	for k, e := range els {
		switch {
		case e.tag == "line" && (strings.Contains(e.attrs["class"], "sq-life") || strings.Contains(e.attrs["class"], "sq-div")):
			lastUnder = k
		case e.tag == "rect" && (strings.Contains(e.attrs["class"], "sq-act") || strings.Contains(e.attrs["class"], "sq-frame")):
			lastUnder = k
		}
	}
	for k, e := range els {
		if e.tag != "rect" || !strings.Contains(e.attrs["class"], "sq-tbg") {
			continue
		}
		if k < lastUnder {
			t.Fatalf("%s: 글 바탕(%d)이 선·막대·테두리(%d)보다 먼저 그려짐\n%s", name, k, lastUnder, svg)
		}
		nx := els[k+1]
		if nx.tag != "text" || !strings.Contains(nx.attrs["class"], "sq-msg-t") && !strings.Contains(nx.attrs["class"], "sq-sec-t") {
			t.Fatalf("%s: 글 바탕 뒤에 글자가 없음(%s)\n%s", name, nx.attrs["class"], svg)
		}
	}
	// 거꾸로 — 메시지 글·조건 글은 모두 바탕 rect 바로 뒤에 온다(선·막대가 지나는 글에는 바탕이 있어야 한다).
	for k, e := range els {
		if e.tag != "text" || !strings.Contains(e.attrs["class"], "sq-msg-t") && !strings.Contains(e.attrs["class"], "sq-sec-t") {
			continue
		}
		if k == 0 {
			t.Fatalf("%s: 글자가 첫 요소임\n%s", name, svg)
		}
		if pv := els[k-1]; pv.tag != "rect" || !strings.Contains(pv.attrs["class"], "sq-tbg") {
			t.Fatalf("%s: 글자(%s) 앞에 바탕 rect 가 없음(%s %s)\n%s", name, e.attrs["class"], pv.tag, pv.attrs["class"], svg)
		}
	}
	// 모든 글자 상자(묶음 머리 칸·조건 글·구획 글·메시지 글·메모 글·참여자 이름·무리 이름·번호·제목)끼리 겹침 없음
	// (수정 1 — 조건 글이 머리 칸·메시지 글과 겹치던 것) 및 메모 상자끼리도.
	type span struct {
		x0, y0, x1, y1 float64
		cls            string
	}
	var spans []span
	for _, e := range tbgs {
		spans = append(spans, span{num(e, "x"), num(e, "y"), num(e, "x") + num(e, "width"), num(e, "y") + num(e, "height"), "tbg"})
	}
	for _, e := range notes {
		spans = append(spans, span{num(e, "x"), num(e, "y"), num(e, "x") + num(e, "width"), num(e, "y") + num(e, "height"), "note"})
	}
	for _, b := range textBoxes(t, svg) {
		spans = append(spans, span{b.x0, b.y0, b.x1, b.y1, b.cls})
	}
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			a, b := spans[i], spans[j]
			// 바탕(tbg)·메모 상자는 제 글자를 담는 그릇 — 그릇과 글자의 짝은 빼고, 글자끼리·그릇끼리만 견준다
			// (그릇이 남의 글자를 덮으면 그릇끼리 겹침으로 잡힌다).
			rectA, rectB := a.cls == "tbg" || a.cls == "note", b.cls == "tbg" || b.cls == "note"
			if rectA != rectB {
				continue
			}
			if a.x0 < b.x1-0.05 && b.x0 < a.x1-0.05 && a.y0 < b.y1-0.05 && b.y0 < a.y1-0.05 {
				t.Fatalf("%s: 글 상자 겹침 %q[%.1f,%.1f]-[%.1f,%.1f] × %q[%.1f,%.1f]-[%.1f,%.1f]\n%s", name,
					a.cls, a.x0, a.y0, a.x1, a.y1, b.cls, b.x0, b.y0, b.x1, b.y1, svg)
			}
		}
	}
	// 조건 글(sq-sec-t)은 제 묶음 테두리 안에 있다(수정 1 — 좁은 묶음에서 테두리·머리 칸을 벗어나던 것)
	type frame struct{ x0, y0, x1, y1 float64 }
	var fs []frame
	for _, e := range frames {
		fs = append(fs, frame{num(e, "x"), num(e, "y"), num(e, "x") + num(e, "width"), num(e, "y") + num(e, "height")})
	}
	for _, b := range textBoxes(t, svg) {
		if !strings.Contains(b.cls, "sq-sec-t") {
			continue
		}
		inside := false
		for _, f := range fs {
			if b.x0 >= f.x0-0.05 && b.x1 <= f.x1+0.05 && b.y0 >= f.y0-0.05 && b.y1 <= f.y1+0.05 {
				inside = true
			}
		}
		if !inside {
			t.Fatalf("%s: 조건 글 %q[%.1f,%.1f]-[%.1f,%.1f] 이 묶음 안에 없음\n%s", name, b.s, b.x0, b.y0, b.x1, b.y1, svg)
		}
	}
	// 메모 상자는 생명선을 가로질러도 된다(원본도 그렇다) — 메시지 글 바탕과 메모의 겹침만 위에서 본다
	// 묶음 테두리는 자기 세로 범위 안의 메시지 선·중첩 테두리를 감싼다
	for _, e := range els {
		if e.tag != "line" || !strings.Contains(e.attrs["class"], "sq-msg") {
			continue
		}
		x1, y, x2 := num(e, "x1"), num(e, "y1"), num(e, "x2")
		lo, hi := mathMin(x1, x2), mathMax(x1, x2)
		for _, f := range fs {
			if f.y0 <= y && y <= f.y1 { // 이 묶음 안의 메시지
				if lo < f.x0-0.05 || hi > f.x1+0.05 {
					t.Fatalf("%s: 메시지(y=%v x=%v..%v)가 묶음 [%v,%v] 밖\n%s", name, y, lo, hi, f.x0, f.x1, svg)
				}
			}
		}
	}
	for i := range fs {
		for j := range fs {
			if i == j {
				continue
			}
			a, b := fs[i], fs[j]
			// 세로로 겹치는 두 묶음은 서로 포함이거나 disjoint 다(같은 층은 허용치 안에서 정렬)
			if a.y0 < b.y1-0.05 && b.y0 < a.y1-0.05 {
				inside := a.x0 <= b.x0+0.05 && a.x1 >= b.x1-0.05 || b.x0 <= a.x0+0.05 && b.x1 >= a.x1-0.05
				apart := a.x1 < b.x0+0.05 || b.x1 < a.x0+0.05
				if !inside && !apart {
					t.Fatalf("%s: 묶음 겹침 [%.1f,%.1f]-[%.1f,%.1f] × [%.1f,%.1f]-[%.1f,%.1f]\n%s", name, a.x0, a.y0, a.x1, a.y1, b.x0, b.y0, b.x1, b.y1, svg)
				}
			}
		}
	}
	// 활성 막대는 생명선 근처(중심에서 활성 폭의 몇 배 안)에 있다
	for _, e := range acts {
		cx := num(e, "x") + num(e, "width")/2
		near := false
		for x := range lifeX {
			if mathAbs(cx-x) <= actW*3 {
				near = true
			}
		}
		if !near {
			t.Fatalf("%s: 활성 막대(x 중심 %v)가 생명선에서 멀다\n%s", name, cx, svg)
		}
	}
	// 글자가 viewBox 안(timeline 의 textBoxes 방식 — tspan 포함)
	for _, b := range textBoxes(t, svg) {
		if b.x0 < -0.05 || b.y0 < -0.05 || b.x1 > vw+0.05 || b.y1 > vh+0.05 {
			t.Fatalf("%s: 글자 %q(%s) 경계 [%.1f,%.1f]-[%.1f,%.1f] 가 viewBox %vx%v 밖\n%s", name, b.s, b.cls, b.x0, b.y0, b.x1, b.y1, vw, vh, svg)
		}
	}

	// 소유 관계(레이아웃 구조체와 SVG 좌표를 견줌 — 리뷰 1회차 「검증과 한계」의 보강): 활성 막대는 **해당 참여자**의
	// 생명선 위(구조의 공식과 SVG 좌표 둘 다), 조건·구획 글은 **제** 묶음 테두리 안(글의 내용·순서까지 견줌).
	if len(ls) == 0 || ls[0] == nil {
		return
	}
	l := ls[0]
	if len(lifes) != len(l.center) {
		t.Fatalf("%s: 생명선 %d 개·참여자 %d 명이 다름\n%s", name, len(lifes), len(l.center), svg)
	}
	dx := num(lifes[0], "x1") - l.center[0] // 생명선은 참여자 차례로 그린다 — 평행이동 거리
	for i, e := range lifes {
		if got := num(e, "x1") - l.center[i]; mathAbs(got-dx) > 0.2 { // 좌표는 소수 한 자리로 적힌다(반올림 허용)
			t.Fatalf("%s: 생명선 %d 이 참여자 %d 과 어긋남(이동 %v ≠ %v)\n%s", name, i, i, got, dx, svg)
		}
	}
	if len(acts) != len(l.acts) {
		t.Fatalf("%s: 활성 막대 %d 개·구조 %d 개가 다름\n%s", name, len(acts), len(l.acts), svg)
	}
	for i, e := range acts {
		a := l.acts[i]
		wantX := l.center[a.actor] + float64(a.level-1)*actW/2
		if mathAbs(a.x-wantX) > 0.05 || mathAbs(num(e, "x")-(a.x+dx)) > 0.2 {
			t.Fatalf("%s: 활성 막대 %d(참여자 %d 겹침 %d)가 해당 생명선 위가 아님\n%s", name, i, a.actor, a.level, svg)
		}
	}
	var wantFrames []*frameDraw
	for _, f := range l.frames {
		if f.kind != bRect {
			wantFrames = append(wantFrames, f)
		}
	}
	if len(frames) != len(wantFrames) {
		t.Fatalf("%s: 묶음 테두리 %d 개·구조 %d 개가 다름\n%s", name, len(frames), len(wantFrames), svg)
	}
	for i, e := range frames {
		f := wantFrames[i]
		if mathAbs(num(e, "x")-(f.x1+dx)) > 0.2 || mathAbs(num(e, "width")-(f.x2-f.x1)) > 0.2 {
			t.Fatalf("%s: 묶음 %d 테두리가 구조와 다름\n%s", name, i, svg)
		}
	}
	type wantText struct {
		lines []string
		f     *frameDraw
	}
	var wantTexts []wantText
	for _, f := range wantFrames {
		if len(f.lines) > 0 {
			wantTexts = append(wantTexts, wantText{f.lines, f})
		}
		for _, sec := range f.sections {
			if len(sec.lines) > 0 {
				wantTexts = append(wantTexts, wantText{sec.lines, f})
			}
		}
	}
	var gotTexts []tbox
	for _, b := range textBoxes(t, svg) {
		if strings.Contains(b.cls, "sq-sec-t") {
			gotTexts = append(gotTexts, b)
		}
	}
	var flat []wantText
	for _, w := range wantTexts {
		for _, ln := range w.lines {
			flat = append(flat, wantText{[]string{ln}, w.f})
		}
	}
	if len(gotTexts) != len(flat) {
		t.Fatalf("%s: 조건·구획 글 줄 %d 개·기대 %d 개가 다름\n%s", name, len(gotTexts), len(flat), svg)
	}
	for i, b := range gotTexts {
		if b.s != flat[i].lines[0] {
			t.Fatalf("%s: 조건·구획 글 줄 %d 가 기대 %q 와 다름(그림 %q)\n%s", name, i, flat[i].lines[0], b.s, svg)
		}
		if b.x0 < flat[i].f.x1+dx-0.05 || b.x1 > flat[i].f.x2+dx+0.05 {
			t.Fatalf("%s: 조건 글 %q[%.1f..%.1f] 이 제 묶음[%.1f..%.1f] 밖\n%s", name, b.s, b.x0, b.x1, flat[i].f.x1+dx, flat[i].f.x2+dx, svg)
		}
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// box 는 글자 한 줄의 추정 경계다(timeline_test.go 와 같은 방식).
type tbox struct {
	cls    string
	x0, y0 float64
	x1, y1 float64
	s      string
}

func textBoxes(t *testing.T, svg string) []tbox {
	t.Helper()
	var out []tbox
	attr := func(s, k string) string {
		m := regexp.MustCompile(`\b` + k + `="([^"]*)"`).FindStringSubmatch(s)
		if m == nil {
			return ""
		}
		return m[1]
	}
	mk := func(cls, anchor, fs, xs, ys, content string) tbox {
		f, _ := strconv.ParseFloat(fs, 64)
		x, _ := strconv.ParseFloat(xs, 64)
		y, _ := strconv.ParseFloat(ys, 64)
		w := diagram.TextW(content, f)
		x0 := x
		switch anchor {
		case "middle":
			x0 = x - w/2
		case "end":
			x0 = x - w
		}
		return tbox{cls: cls, x0: x0, x1: x0 + w, y0: y - f*0.85, y1: y + f*0.25, s: content}
	}
	for _, m := range regexp.MustCompile(`<text ([^>]*)>(.*?)</text>`).FindAllStringSubmatch(svg, -1) {
		cls, anchor, fs := attr(m[1], "class"), attr(m[1], "text-anchor"), attr(m[1], "font-size")
		if fs == "" {
			fs = "14"
		}
		if !strings.Contains(m[2], "<tspan") {
			out = append(out, mk(cls, anchor, fs, attr(m[1], "x"), attr(m[1], "y"), htmlUnescape(m[2])))
			continue
		}
		for _, ts := range regexp.MustCompile(`<tspan ([^>]*)>(.*?)</tspan>`).FindAllStringSubmatch(m[2], -1) {
			out = append(out, mk(cls, anchor, fs, attr(ts[1], "x"), attr(ts[1], "y"), htmlUnescape(ts[2])))
		}
	}
	return out
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&amp;", "&")
	return r.Replace(s)
}

// TestLayoutProperties 는 무작위·극단 그림 수십 판으로 배치 성질을 본다 — 참여자 1/50, 긴 이름·글, 자기 메시지
// 연속, 묶음 깊이 10, <br> 연속, 한글/영문 혼합, 활성 중첩, 메모가 양끝 참여자 밖으로 나가는 것. 시드를 고정해
// 무작위도 결정적으로 만든다.
func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(2029))
	name := func(i int) string {
		switch rng.Intn(10) {
		case 0:
			return strings.Repeat("가", 1+rng.Intn(MaxLabel))
		case 1:
			return "긴 이름 word " + strings.Repeat("name ", rng.Intn(15)) + strconv.Itoa(i)
		case 2:
			return "첫째<br>둘째" + strconv.Itoa(i)
		case 3:
			return "가<br><br>나" + strconv.Itoa(i) // <br> 연속
		case 4:
			return "별명#35;" + strconv.Itoa(i) // 별명의 엔티티(리뷰 1회차 [3] 의 라벨 경로)
		}
		return "참여자" + strconv.Itoa(i)
	}
	msg := func(i int) string {
		switch rng.Intn(12) {
		case 0:
			return strings.Repeat("다", 1+rng.Intn(MaxLabel))
		case 1:
			return strings.Repeat("long message text ", rng.Intn(12)) + strconv.Itoa(i)
		case 2:
			return "첫째 줄<br>둘째 줄 " + strconv.Itoa(i)
		case 3:
			return "" // 빈 글
		}
		return "메시지" + strconv.Itoa(i)
	}
	arrows := []string{"->>", "-->>", "->", "-->", "-x", "--x", "-)", "--)", "<<->>", "<<-->>"}
	// cond 는 묶음 조건 글이다 — 좁은 묶음(자기 메시지 하나)에서도 머리 칸과 겹치지 않는지 보게 긴 글을 섞는다(수정 1).
	cond := func() string {
		switch rng.Intn(9) {
		case 0:
			return "매번 확인"
		case 7:
			return "조건 " + strconv.Itoa(rng.Intn(100))
		case 1:
			return strings.Repeat("긴 조건 ", rng.Intn(20))
		case 2:
			return strings.Repeat("가", 1+rng.Intn(60))
		}
		return []string{"매번", "선택", "갈림", "병렬", "위험", "중단"}[rng.Intn(6)]
	}
	ran, selfSeen, deepSeen, narrowSeen, boxSeen := 0, 0, 0, 0, 0
	for round := 0; round < 80; round++ {
		var b strings.Builder
		b.WriteString("sequenceDiagram\n")
		if rng.Intn(3) == 0 {
			b.WriteString("title 순서도 " + strconv.Itoa(round) + "\n")
		}
		n := 1 + rng.Intn(8)
		if round%20 == 0 {
			n = 1 + rng.Intn(3) // 참여자 아주 적은 판
		}
		for i := 0; i < n; i++ {
			kw := "participant "
			if rng.Intn(3) == 0 {
				kw = "actor "
			}
			if rng.Intn(4) == 0 {
				fmt.Fprintf(&b, "%sP%d as %s\n", kw, i, name(i))
			} else {
				fmt.Fprintf(&b, "%sP%d\n", kw, i)
			}
		}
		switch rng.Intn(8) {
		case 0:
			b.WriteString("autonumber 10 5\n")
		case 1:
			b.WriteString("autonumber\n")
		case 2:
			b.WriteString("autonumber 90000 90\n") // 큰 시작·단계(리뷰 1회차 [8]) — 번호는 5 자리 안에 머문다
		case 3:
			b.WriteString("autonumber 990 3\n") // 여러 자리 번호(4 자리 도달)
		}
		open := 0
		var openKinds []string
		actsOpen := 0
		// 문장 활성(activate) 으로 열어 묶음 머리를 지나는 활성 막대를 만든다(수정 1 — 결함 1 의 재료).
		if rng.Intn(3) == 0 {
			b.WriteString("activate P0\n")
			actsOpen++
		}
		for k := 0; k < 8+rng.Intn(24); k++ {
			switch rng.Intn(15) {
			case 0:
				if open < 4 {
					kw := []string{"loop", "opt", "alt", "par", "critical", "break"}[rng.Intn(6)] + " " + cond()
					if rng.Intn(8) == 0 {
						kw = "rect rgb(1,2,3)"
					}
					b.WriteString(kw + "\n")
					openKinds = append(openKinds, kw)
					open++
				}
			case 1:
				if open > 0 {
					inner := openKinds[len(openKinds)-1]
					if rng.Intn(4) == 0 && (strings.HasPrefix(inner, "alt") || strings.HasPrefix(inner, "par") || strings.HasPrefix(inner, "critical")) {
						b.WriteString(map[string]string{"alt": "else 나머지", "par": "and 함께", "critical": "option 대안"}[inner[:strings.IndexByte(inner, ' ')]])
						b.WriteString("\n")
					} else {
						b.WriteString("end\n")
						openKinds = openKinds[:len(openKinds)-1]
						open--
					}
				}
			case 2:
				fmt.Fprintf(&b, "Note left of P%d: %s\n", rng.Intn(n), msg(k))
			case 3:
				fmt.Fprintf(&b, "Note right of P%d: %s\n", rng.Intn(n), msg(k))
			case 4:
				a, c := rng.Intn(n), rng.Intn(n)
				fmt.Fprintf(&b, "P%d%sP%d: %s\n", a, arrows[rng.Intn(len(arrows))], c, msg(k))
				if a == c {
					selfSeen++
				}
			case 5:
				fmt.Fprintf(&b, "P0->>+P0: %s\n", msg(k))
				actsOpen++
			case 6:
				if actsOpen > 0 {
					fmt.Fprintf(&b, "P0-->>-P0: %s\n", msg(k))
					actsOpen--
				}
			case 7: // 좁은 묶음 — 자기 메시지 하나만 감싼다(수정 1 — 결함 2·3 의 재료: 머리 칸과 조건 글이 비좁다)
				if open < 4 {
					kw := []string{"loop", "opt", "par"}[rng.Intn(3)] + " " + cond()
					b.WriteString(kw + "\n")
					fmt.Fprintf(&b, "P0->>P0: %s\nend\n", msg(k))
					narrowSeen++
				}
			case 8, 9: // box — 긴 이름·이웃 box 로 띠 폭·이름 줄·간격을 본다(리뷰 1회차 [4])
				what := []string{"서버", strings.Repeat("가", 1+rng.Intn(120)), "rgb(1,2,3) " + strings.Repeat("무리 ", rng.Intn(30))}[rng.Intn(3)]
				fmt.Fprintf(&b, "box %s\nparticipant Q%d\nend\n", what, k)
				if rng.Intn(2) == 0 {
					fmt.Fprintf(&b, "box %s\nparticipant R%d\nend\nQ%d->>R%d: %s\n", what, k, k, k, msg(k))
					boxSeen++
				}
				boxSeen++
			default:
				fmt.Fprintf(&b, "P%d%sP%d: %s\n", rng.Intn(n), arrows[rng.Intn(len(arrows))], rng.Intn(n), msg(k))
			}
		}
		for ; actsOpen > 0; actsOpen-- {
			b.WriteString("P0-->>-P0: 닫기\n")
		}
		for ; open > 0; open-- {
			b.WriteString("end\n")
		}
		src := b.String()
		if len(src) > MaxSource {
			continue
		}
		l, svg0, err := renderLErr(src)
		if err != nil {
			t.Fatalf("round %d: %v\n%s", round, err, src)
		}
		r, err := Render(src)
		if err != nil {
			t.Fatalf("round %d 재렌더: %v\n%s", round, err, src)
		}
		if svg0 != r.SVG {
			t.Fatalf("round %d: 두 경로의 SVG 가 다름", round)
		}
		checkLayout(t, "round "+strconv.Itoa(round), r.SVG, l)
		ran++
		if strings.Count(r.SVG, "<tspan") > n*2 {
			deepSeen++
		}
	}
	if ran < 70 || selfSeen < 10 || deepSeen < 15 || narrowSeen < 10 || boxSeen < 8 {
		t.Errorf("실행 %d 회·자기 메시지 %d·여러 줄 라벨 %d·좁은 묶음 %d·box %d — 표본이 모자람", ran, selfSeen, deepSeen, narrowSeen, boxSeen)
	}

	// 극단 — 상한 경계: 참여자 50(메시지로 암묵 생성)·묶음 깊이 10·메모가 양끝 밖(Note left of 첫 참여자)·자기 메시지 연속
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	for i := 0; i < MaxActors; i++ {
		fmt.Fprintf(&b, "P%d->>P%d: x\n", i, i)
	}
	svg, _ := drawn(t, b.String())
	checkLayout(t, "참여자 50", svg)

	b.Reset()
	b.WriteString("sequenceDiagram\nA->>B: x\n")
	for i := 0; i < MaxDepth; i++ {
		b.WriteString("loop 층\n")
	}
	b.WriteString("A->>B: 안\n")
	for i := 0; i < MaxDepth; i++ {
		b.WriteString("end\n")
	}
	svg, _ = drawn(t, b.String())
	checkLayout(t, "깊이 10", svg)

	checkLayout(t, "메모 양끝", mustSVG(t, "sequenceDiagram\nA->>B: x\nNote left of A: 왼쪽 끝 메모\nNote right of B: 오른쪽 끝 메모"))
	checkLayout(t, "자기 연속", mustSVG(t, "sequenceDiagram\nA->>A: 하나\nA->>A: 둘\nA-->>A: 셋"))
	checkLayout(t, "메시지 0", mustSVG(t, "sequenceDiagram\nparticipant A\nparticipant B"))
	long := strings.Repeat("가", MaxLabel)
	checkLayout(t, "라벨 200자", mustSVG(t, "sequenceDiagram\nparticipant A as "+long+"\nA->>B: "+long))
	checkLayout(t, "묶음만", mustSVG(t, "sequenceDiagram\nA->>B: x\nloop\nend"))
}

func mustSVG(t *testing.T, src string) string {
	t.Helper()
	svg, _ := drawn(t, src)
	return svg
}

// 클래스 충돌(간트·타임라인과 같은 검사) — 순서도 SVG 의 클래스는 모두 sq- 접두사를 단다. .sq- 선택자는 base.css 의
// .diagram-svg 아래에만 있어야 한다.
func TestClassCollisions(t *testing.T) {
	svg, _ := drawn(t, "sequenceDiagram\nactor A\nbox 무리\nparticipant B\nend\nautonumber\nA->>+B: x\nalt 매번\nNote over A,B: 메모\nB-->>-A: y\nelse 가끔\nend")
	classWords := func(svg string) map[string]bool {
		out := map[string]bool{}
		for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
			for _, w := range strings.Fields(m[1]) {
				out[w] = true
			}
		}
		return out
	}
	for w := range classWords(svg) {
		if w != "diagram-svg" && !strings.HasPrefix(w, "sq-") {
			t.Errorf("순서도 클래스 %q 에 sq- 접두사가 없음", w)
		}
	}
	files, err := filepath.Glob("../../web/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("dist css 를 못 찾음: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s 읽기 실패: %v", f, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, ".sq-") && !strings.Contains(line, ".diagram-svg") {
				t.Errorf("%s:%d — .sq- 선택자가 .diagram-svg 문맥 밖", f, i+1)
			}
		}
	}
}

// TestHashBoundary 는 남은 `#` 경계를 구조로 닫는 속성 시험이다(timeline ② TestHashBoundaryProperties 꼴 — ①② 리뷰가
// 두 회차 연속 이 경계를 찾았다). 무작위 문장 문자열(#·;·ASCII 낱말 글자·한글·공백·&·<·>)마다, 시험 안에 독립으로 쓴
// 판정기(원본 규칙: style `;` 자르기 → classDef `;` 자르기 → 엔티티 #\w+; 제거 → # 가 남으면 원본이 주석으로 버린다)와
// Render 의 성패가 같은지 본다. 그려지는 판은 날 <·& 가 SVG 에 없는지(이스케이프 경계)도 본다.
func TestHashBoundary(t *testing.T) {
	entTok := regexp.MustCompile(`#[A-Za-z0-9_]+;`)
	reject := func(line string) bool { // 원본 전처리+lexer 규칙의 독립 구현
		s := regexp.MustCompile(`style.*:\S*#.*;`).ReplaceAllStringFunc(line, func(s string) string { return s[:len(s)-1] })
		s = regexp.MustCompile(`classDef.*:\S*#.*;`).ReplaceAllStringFunc(s, func(s string) string { return s[:len(s)-1] })
		return strings.Contains(entTok.ReplaceAllString(s, ""), "#")
	}
	knownTag := map[string]bool{"svg": true, "rect": true, "line": true, "path": true, "circle": true, "text": true, "tspan": true, "title": true}
	rng := rand.New(rand.NewSource(2030))
	letters := []rune("#;abzAZ09_가다 사건&<>") // 콜론(문장 구분)은 # 판정과 무관하게 다른 오류를 낼 수 있어 넣지 않는다
	ran, rejected, drawnCount := 0, 0, 0
	for round := 0; round < 400; round++ {
		mk := func(n int) string {
			var b strings.Builder
			for i := 0; i < n; i++ {
				if rng.Intn(6) == 0 { // 엔티티 꼴을 확실히 섞는다
					b.WriteString([]string{"#35;", "#amp;", "#lt;", "#58;"}[rng.Intn(4)])
					continue
				}
				b.WriteRune(letters[rng.Intn(len(letters))])
			}
			return b.String()
		}
		text := mk(1 + rng.Intn(12))
		src := "sequenceDiagram\nA->>B: " + text
		want := reject(text)
		r, err := Render(src)
		ran++
		got := err != nil && strings.Contains(err.Error(), "# 부터는")
		if got != want {
			t.Fatalf("round %d: 메시지 %q 의 # 판정이 판정기와 다름 — Render 오류 %v, 판정기 거부 %v\n%s", round, text, err, want, src)
		}
		if err != nil {
			rejected++
			continue
		}
		drawnCount++
		for _, m := range regexp.MustCompile(`<([a-zA-Z/]+)`).FindAllStringSubmatch(r.SVG, -1) {
			tag := strings.TrimPrefix(m[1], "/")
			if !knownTag[tag] {
				t.Fatalf("round %d: 날 < 가 SVG 에 있음(%q)\n%s", round, m[1], r.SVG)
			}
		}
		for i := 0; i < len(r.SVG); i++ {
			if r.SVG[i] == '&' && !strings.HasPrefix(r.SVG[i:], "&amp;") && !strings.HasPrefix(r.SVG[i:], "&lt;") &&
				!strings.HasPrefix(r.SVG[i:], "&gt;") && !strings.HasPrefix(r.SVG[i:], "&quot;") && !strings.HasPrefix(r.SVG[i:], "&#39;") {
				t.Fatalf("round %d: 날 & 가 SVG 에 있음(%.20s)\n%s", round, r.SVG[i:], r.SVG)
			}
		}
	}
	if ran < 400 || rejected < 50 || drawnCount < 100 {
		t.Errorf("표본 부족 — 실행 %d·거부 %d·그림 %d", ran, rejected, drawnCount)
	}
}

// renderL 은 배치 구조와 SVG 를 함께 돌려준다(소유 관계 검사 — 활성의 참여자·조건 글의 묶음).
func renderL(t *testing.T, src string) (*layout, string) {
	t.Helper()
	c, _, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	l, err := newLayout(c)
	if err != nil {
		t.Fatalf("layout(%q): %v", src, err)
	}
	return l, l.emit()
}

// TestReviewR1 은 리뷰 1회차 발견 8건의 재현 시험이다(scratchpad result-sequence-r1.md — 리뷰어의 overlay 시험을
// 저장소 시험로 옮겼다).
func TestReviewR1(t *testing.T) {
	t.Run("box 중복·소속(별명 없는 재선언)", func(t *testing.T) { // [1] 원본 addActor 는 충돌 검사·소속 대입이 조기 반환보다 앞
		errAt(t, "sequenceDiagram\nbox one\nparticipant A\nend\nbox two\nparticipant A\nparticipant B\nend", 6, "다른 box")
		c, _, err := Parse("sequenceDiagram\nparticipant A\nbox one\nparticipant A\nparticipant B\nend\nA->>B: x")
		if err != nil {
			t.Fatal(err)
		}
		if c.Actors[0].Box != 0 || c.Actors[1].Box != 0 {
			t.Fatalf("재선언 참여자의 box 소속: A=%d B=%d, 기대 둘 다 0", c.Actors[0].Box, c.Actors[1].Box)
		}
	})
	t.Run("번호 카운터 분리·0", func(t *testing.T) { // [2] off 구간·autonumber 앞의 메시지도 세고, 0 은 세던 값
		svg, _ := drawn(t, "sequenceDiagram\nautonumber\nA->>B: one\nautonumber off\nA->>B: hidden\nautonumber\nA->>B: three")
		if !strings.Contains(svg, ">1<") || !strings.Contains(svg, ">3<") || strings.Contains(svg, ">2<") {
			t.Errorf("off 구간의 번호: 1·3 이어야 함\n%s", svg)
		}
		svg, _ = drawn(t, "sequenceDiagram\nA->>B: first\nautonumber\nA->>B: second") // autonumber 앞도 센다
		if !strings.Contains(svg, ">2<") {
			t.Errorf("autonumber 앞의 메시지를 안 셈\n%s", svg)
		}
		svg, _ = drawn(t, "sequenceDiagram\nautonumber 0 0\nA->>B: one\nA->>B: two")
		if !strings.Contains(svg, ">1<") || !strings.Contains(svg, ">2<") {
			t.Errorf("autonumber 0 0: 0 은 세던 값(1, 2)\n%s", svg)
		}
	})
	t.Run("참여자 표시 이름 엔티티·상한", func(t *testing.T) { // [3] → 이름의 엔티티는 허용 꼴 밖(itda-hyve#208 허용 목록), 별명은 라벨 경로
		errAt(t, "sequenceDiagram\nparticipant A#35;\nA->>B: hi", 2, "참여자 선언은")
		errAt(t, "sequenceDiagram\nA#35;->>B: hi", 2, "참여자 이름")
		svg, _ := drawn(t, "sequenceDiagram\nparticipant A as A#35;\nA->>B: hi")
		if strings.Contains(svg, "ﬂ°") || strings.Contains(svg, "¶ß") || !strings.Contains(svg, ">A#<") {
			t.Errorf("별명의 엔티티가 표식으로 새거나 안 풀림\n%s", svg)
		}
		errAt(t, "sequenceDiagram\nparticipant "+strings.Repeat("가", MaxLabel+1), 2, "라벨이 200 자")
	})
	t.Run("긴 box 이름", func(t *testing.T) { // [4] 띠 폭에서 감고 높이에 반영
		l, svg := renderL(t, "sequenceDiagram\nbox "+strings.Repeat("가", 100)+"\nparticipant A\nend\nA->>A: x")
		checkLayout(t, "긴 box 이름", svg, l)
		if n := strings.Count(svg, "sq-box-t"); n == 0 || !strings.Contains(svg, "</tspan>") {
			t.Errorf("box 이름이 여러 줄로 안 감김\n%s", svg)
		}
	})
	t.Run("wrap 앞말 자리", func(t *testing.T) { // [5] → itda-hyve#208 결정으로 원문+경고로 좁힘(자리 목록은 그대로 검사)
		for _, src := range []string{
			"sequenceDiagram\nparticipant A as wrap: Alice",
			"sequenceDiagram\nloop wrap: Retry\nA->>A: x\nend",
			"sequenceDiagram\nalt nowrap: 첫째\nA->>A: x\nelse wrap: 둘째\nend",
			"sequenceDiagram\nA->>A: :wrap: hello",
			"sequenceDiagram\nNote over A: wrap: 메모",
			"sequenceDiagram\nbox nowrap: 무리\nparticipant B\nend",
		} {
			r, err := Render(src)
			if err == nil || strings.Contains(r.SVG, "<svg") {
				t.Errorf("wrap 앞말 %q 가 원문+경고가 아님: %v", src, err)
			}
		}
	})
	t.Run("box 색 transparent·애매 함수", func(t *testing.T) { // [6] → itda-hyve#208 결정으로 경계 축소(자세한 규칙은 TestReviewR2)
		c, _, err := Parse("sequenceDiagram\nbox transparent Team\nparticipant A\nend\nA->>A: x")
		if err != nil || c.Boxes[0].Name != "Team" {
			t.Errorf("transparent 색: 이름 %q %v", c.Boxes[0].Name, err)
		}
		c, _, err = Parse("sequenceDiagram\nbox NotAColor Team\nparticipant A\nend\nA->>A: x")
		if err != nil || c.Boxes[0].Name != "NotAColor Team" { // 일반 낱말 — 원본처럼 이름 전체
			t.Errorf("모르는 색 낱말: 이름 %q %v", c.Boxes[0].Name, err)
		}
		errAt(t, "sequenceDiagram\nbox red-team\nparticipant A\nend", 2, "색 경계를 확신할 수 없음")
		errAt(t, "sequenceDiagram\nbox rgb(1) Team\nparticipant A\nend", 2, "색 경계를 확신할 수 없음")
		errAt(t, "sequenceDiagram\nbox rgb(bad) Team\nparticipant A\nend", 2, "색 경계를 확신할 수 없음")
		errAt(t, "sequenceDiagram\nbox hsl(120deg, 1%, 1%) Team\nparticipant A\nend", 2, "색 경계를 확신할 수 없음")
	})
	t.Run("선언 ID 의 금지 글자", func(t *testing.T) { // [7] 별명 안의 같은 글자는 허용
		errAt(t, "sequenceDiagram\nparticipant A:B\nA->>B: x", 2, "참여자 선언은")
		errAt(t, "sequenceDiagram\nparticipant A,B\nA->>B: x", 2, "참여자 선언은")
		drawn(t, "sequenceDiagram\nparticipant A as 별명: 쉼표, 콜론:\nA->>A: x") // 별명은 restOfLine — 허용
	})
	t.Run("큰 autonumber", func(t *testing.T) { // [8] 상한·비유한 값·번호 폭
		errAt(t, "sequenceDiagram\nautonumber 9999999\nA->>A: x", 2, "6 자리 이하")
		errAt(t, "sequenceDiagram\nautonumber 10 9999999\nA->>A: x", 2, "6 자리 이하")
		errAt(t, "sequenceDiagram\nautonumber "+strings.Repeat("9", 400)+"\nA->>A: x", 2, "6 자리")
		errAt(t, "sequenceDiagram\nautonumber 999999 999999\nA->>A: x\nA->>A: y", 4, "번호가 6 자리")
		l, svg := renderL(t, "sequenceDiagram\nautonumber 9999 33\nA->>A: x\nA->>A: y\nA->>A: z") // 4~5 자리 — 글자 크기 사다리
		checkLayout(t, "큰 번호", svg, l)
		if !strings.Contains(svg, `font-size="9"`) {
			t.Errorf("4 자리 번호의 글자 크기 사다리(9)\n%s", svg)
		}
	})
}

func sequenceDiagramSrc(s string) string { return "sequenceDiagram\n" + s }

// renderLErr 는 renderL 의 오류를 돌려주는 꼴(생성기에서 쓴다).
func renderLErr(src string) (*layout, string, error) {
	c, _, err := Parse(src)
	if err != nil {
		return nil, "", err
	}
	l, err := newLayout(c)
	if err != nil {
		return nil, "", err
	}
	return l, l.emit(), nil
}

// TestReviewR2 는 리뷰 2회차 발견 4건의 재현 시험이다(scratchpad result-sequence-r2.md — itda-hyve#208 접근 방식
// 결정으로 좁힌 뒤의 기대값).
func TestReviewR2(t *testing.T) {
	t.Run("wrap 앞말 이중 제거", func(t *testing.T) { // [1] → 앞말은 이제 원문+경고(B1)
		for _, src := range []string{
			"sequenceDiagram\nparticipant A as wrap:wrap:Alice",
			"sequenceDiagram\nloop wrap:wrap:Retry\nA->>A: wrap:wrap:hello\nend",
			"sequenceDiagram\nA->>A: wrap: hello",
		} {
			if r, err := Render(src); err == nil || strings.Contains(r.SVG, "<svg") {
				t.Errorf("%q 가 원문+경고가 아님: %v", src, err)
			}
		}
	})
	t.Run("box 색 경계", func(t *testing.T) { // [2] red-team·rgb(1) — 원본 해석(red/-team)을 확신할 수 없어 원문
		errAt(t, "sequenceDiagram\nbox red-team\nparticipant A\nend", 2, "색 경계를 확신할 수 없음")
		errAt(t, "sequenceDiagram\nbox rgb(1) Team\nparticipant A\nend", 2, "색 경계를 확신할 수 없음")
		// 정상 꼴은 그린다(색 무시 경고 1회)
		_, issues := drawn(t, "sequenceDiagram\nbox rgb(1,2,3) Team\nparticipant A\nend\nA->>A: x")
		if len(issues) != 1 || !strings.Contains(issues[0].Msg, "box 의 색") {
			t.Errorf("정상 함수 꼴: 경고 %+v", issues)
		}
		_, issues = drawn(t, "sequenceDiagram\nbox red Team\nparticipant A\nend\nA->>A: x")
		if len(issues) != 1 || !strings.Contains(issues[0].Msg, "box 의 색") {
			t.Errorf("색 이름+이름: 경고 %+v", issues)
		}
		c, _, err := Parse("sequenceDiagram\nbox red-team Title\nparticipant A\nend") // red 뒤 '-' — 거부
		if err == nil {
			t.Errorf("red-team Title 가 통과: %+v", c.Boxes)
		}
	})
	t.Run("접근성 값의 세미콜론", func(t *testing.T) { // [3] 값은 줄 끝까지 — 문장이 생기지 않는다
		// 접근성 값의 맨 > 는 원문이다(정책 B) — 화살표는 > 없는 -x 로 적는다
		c, issues, err := Parse("sequenceDiagram\nparticipant A\naccTitle: description; A-xB: phantom")
		if err != nil {
			t.Fatal(err)
		}
		if c.AccTitle != "description; A-xB: phantom" {
			t.Errorf("AccTitle=%q", c.AccTitle)
		}
		if len(c.Actors) != 1 || len(c.Events) != 0 || len(issues) != 0 {
			t.Errorf("유령 참여자·메시지가 생김: actors=%d events=%d issues=%v", len(c.Actors), len(c.Events), issues)
		}
		c, _, err = Parse("sequenceDiagram\nparticipant A\naccDescr: 설명; activate B; # 메모")
		if err != nil {
			t.Fatal(err)
		}
		if c.AccDescr != "설명; activate B; # 메모" {
			t.Errorf("AccDescr=%q", c.AccDescr)
		}
	})
	t.Run("참조 자리 예약어", func(t *testing.T) { // [4]
		errAt(t, "sequenceDiagram\nA->>end: hello", 2, "예약어")
		errAt(t, "sequenceDiagram\nA->>loop: x", 2, "예약어")
		errAt(t, "sequenceDiagram\nnote over end: x", 2, "예약어") // 대소문자 무시
		errAt(t, "sequenceDiagram\nNote right of BOX: x", 2, "예약어")
		// 선언(ID 상태)은 원본이 읽지만 허용 목록은 선언·참조 모두 막는다(단순화 — itda-hyve#208)
		errAt(t, "sequenceDiagram\nparticipant end\nparticipant A\nA->>A: x", 2, "예약어")
		errAt(t, "sequenceDiagram\nparticipant A\nactivate end", 3, "예약어")
	})
}

// TestReviewR3 은 리뷰 3회차 재현 4건이다(scratchpad result-sequence-r3.md) — itda-hyve#208 두 번째 접근 방식 결정(허용 목록
// 문법)으로 넷 모두 원문 + 줄 번호 경고가 된다.
func TestReviewR3(t *testing.T) {
	for _, tc := range []struct {
		src  string
		line int
		msg  string
	}{
		{"sequenceDiagram\nparticipant A\naccDescr { 설명 } A->>B: visible", 3, "닫는 } 뒤에"}, // [1] } 뒤 문장을 버리지 않는다
		{"sequenceDiagram\nparticipant A\naccDescr {\n설명\n} A->>B: visible", 5, "닫는 } 뒤에"},
		{"sequenceDiagram\nparticipant P\nA%%->>B: phantom", 3, "%%"}, // [2] 메시지 앞부분의 %%
		{"sequenceDiagram\n5 ->>B: hi", 2, "참여자 이름"},                  // [3] 숫자 시작 이름(원본 NUM 토큰)
		{"sequenceDiagram\n5->>B: hi", 2, "참여자 이름"},                   // 빈칸 없는 꼴도 같은 규칙(숫자 시작 이름 없음)
		{"sequenceDiagram\nparticipant A as;", 2, "`;`"},              // [4] as 뒤 ; — 한 줄에 문장 하나
		{"sequenceDiagram\nparticipant A as ;", 2, "`;`"},
		{"sequenceDiagram\nparticipant A as", 2, "참여자 선언은"}, // as 뒤 라벨이 비면 원문(원본은 줄바꿈이면 별명 표시, 끝이면 이름 「A as」)
		{"sequenceDiagram\nparticipant A as \t", 2, "참여자 선언은"},
		{"sequenceDiagram\nparticipant xB\nA->>+xB: 열기\nxB->>-xB: 닫기", 4, "가위표"}, // 검토 중 찾음 — -x 토큰이 - 보다 앞
	} {
		errAt(t, tc.src, tc.line, tc.msg)
	}
	// 닫는 } 가 줄 끝이면 그린다 — 같은 줄·다음 줄 모두
	for _, src := range []string{"sequenceDiagram\naccDescr { 설명 }\nA->>B: x", "sequenceDiagram\naccDescr {\n설명\n}  \nA->>B: x"} {
		c, _, err := Parse(src)
		if err != nil || c.AccDescr != "설명" || len(c.Events) != 1 {
			t.Errorf("%q: AccDescr %q events %d %v", src, c.AccDescr, len(c.Events), err)
		}
	}
}

// TestReviewR4 는 리뷰 4회차 재현 — 지시자 대소문자는 ASCII 만 접고(원본 JS i 깃발, u 없음), box 색 후보는 CSS 색
// 키워드 닫힌 목록(시스템 색 포함)으로 가른다. 글자는 코드 포인트로 적는다(U+212A KELVIN SIGN·U+017F LONG S).
func TestReviewR4(t *testing.T) {
	for _, tc := range []struct {
		src  string
		line int
		msg  string
	}{
		{"sequenceDiagram\nparticipant A\nbrea\u212a check\nA->>A: ok\nend", 3, "알 수 없는 문장"},  // [1] K(U+212A) 는 k 가 아니다
		{"sequenceDiagram\nparticipant A\nalt a\nA->>A: x\nel\u017fe b\nend", 5, "알 수 없는 문장"}, // ſ(U+017F) 는 s 가 아니다
		{"sequenceDiagram\nlin\u212a A: x", 2, "알 수 없는 문장"},
		{"sequenceDiagram\naccDe\u017fcr: x", 2, "알 수 없는 문장"},
		{"\u017fequenceDiagram\nA->>A: x", 1, "sequenceDiagram 이 아님"},
		{"sequenceDiagram\nbox revert-layer Team\nparticipant A\nend", 2, "색 경계"}, // 원본 \w* 는 revert 까지만 — 확신 못 함
		{"sequenceDiagram\nbox blac\u212a Team\nparticipant A\nend", 2, "색 경계"},
	} {
		errAt(t, tc.src, tc.line, tc.msg)
	}
	// ASCII 대문자 K 는 접힌다 — break 묶음
	c, _, err := Parse("sequenceDiagram\nparticipant A\nbreaK check\nA->>A: ok\nend")
	if err != nil || len(c.Events) == 0 || c.Events[0].kind != evBlockStart || c.Events[0].block != bBreak || c.Events[0].text != "check" {
		t.Errorf("breaK(ASCII K): %v %+v", err, c)
	}
	// [2] 시스템 색은 색이다 — 색을 떼고(경고) 나머지가 제목. 목록에 없는 낱말은 제목의 일부.
	for _, tc := range []struct{ src, name string }{
		{"box Canvas Team", "Team"},
		{"box ButtonText Team", "Team"},
		{"box canvastext Team", "Team"},
		{"box ThreeDFace Team", "Team"},
		{"box Team A", "Team A"},
	} {
		c, issues, err := Parse("sequenceDiagram\n" + tc.src + "\nparticipant A\nend")
		if err != nil || len(c.Boxes) != 1 || c.Boxes[0].Name != tc.name {
			t.Errorf("%q: %v %+v", tc.src, err, c)
			continue
		}
		warned := len(issues) == 1 && strings.Contains(issues[0].Msg, "box 의 색")
		if want := tc.name == "Team"; warned != want {
			t.Errorf("%q: 색 경고 %v, 기대 %v (%v)", tc.src, warned, want, issues)
		}
	}
	for _, color := range []string{"Canvas", "AccentColor", "WindowText", "revert-layer"} {
		if _, _, err := Parse("sequenceDiagram\nrect " + color + "\nA->>A: x\nend"); err != nil {
			t.Errorf("rect %s: %v", color, err)
		}
	}
}

// TestReviewR5 는 리뷰 5회차 재현 — 빈 accDescr 블록(한 줄·여러 줄, 공백뿐)은 앞 설명을 바꾸지 않는다. 원본은 여는
// 토큰 accDescr\s*"{"\s* 가 공백·줄바꿈을 먹고 곧장 } 규칙이 상태만 닫아 값 토큰이 없다(jison:76–78·190).
func TestReviewR5(t *testing.T) {
	for _, tc := range []struct{ block, want string }{
		{"accDescr {}", "keep"},
		{"accDescr {   }", "keep"},
		{"accDescr{\t}", "keep"},
		{"accDescr {\n   \n\n}", "keep"},
		{"accDescr {\n}", "keep"},
		{"accDescr { 새 설명 }", "새 설명"},
		{"accDescr {\n새\n설명\n}", "새\n설명"}, // 블록 줄바꿈은 그대로(정책 B)
	} {
		svg, issues := drawn(t, "sequenceDiagram\nparticipant A\naccDescr: keep\n"+tc.block+"\nA->>A: x")
		if !strings.Contains(svg, "<title>"+tc.want+"</title>") || len(issues) != 0 {
			t.Errorf("%q: <title>%s</title> 없음 또는 경고 %v\n%s", tc.block, tc.want, issues, svg)
		}
	}
	// 블록 뒤 빈 블록 — 앞 블록 설명이 남는다. 앞 설명이 없으면 그대로 비어 있다(<title> 없음).
	c, _, err := Parse("sequenceDiagram\naccDescr {\n앞\n}\naccDescr {  }\nA->>A: x")
	if err != nil || c.AccDescr != "앞" {
		t.Errorf("블록 뒤 빈 블록: %q %v", c.AccDescr, err)
	}
	if svg, _ := drawn(t, "sequenceDiagram\naccDescr {}\nA->>A: x"); strings.Contains(svg, "<title>") {
		t.Errorf("앞 설명 없는 빈 블록이 <title> 을 만듦\n%s", svg)
	}
}

// TestReviewR6 는 리뷰 6회차 재현 — ASCII 밖 공백·서식 문자가 든 줄은 원문 + 그 줄 번호(2026-10-30 결정). 원본 JS \s 는
// U+FEFF 를 공백으로 먹어 accDescr {U+FEFF} 가 빈 블록인데 Go TrimSpace 는 남겨 앞 설명을 덮어썼다.
func TestReviewR6(t *testing.T) {
	const keep = "sequenceDiagram\nparticipant A\naccDescr: keep\n"
	for _, tc := range []struct {
		src  string
		line int
	}{
		{keep + "accDescr {\ufeff}\nA->>A: x", 4},     // 리뷰어 재현 — 단독
		{keep + "accDescr { \ufeff }\nA->>A: x", 4},   // 양옆 공백
		{keep + "accDescr {\n\ufeff\n}\nA->>A: x", 5}, // 여러 줄
		{keep + "accDescr {\n\u0085\n}\nA->>A: x", 5}, // U+0085 — Go 만 공백
		{"sequenceDiagram\nA->>B:\u0085글", 2},
		{"sequenceDiagram\nA\u00a0->>B: 글", 2},
		{"sequenceDiagram\nA->>B: 붙여넣은\u00a0글", 2},
		{"sequenceDiagram\nA->>B: 글\u00a0", 2}, // 줄 끝
		{"sequenceDiagram\nparticipant A as 이름\u00a0표시", 2},
		{"sequenceDiagram\nparticipant A as 이름\u0085표시", 2},
		{"sequenceDiagram\nA->>B: 폭\u200b없음", 2},
		{"sequenceDiagram\nA->>B: 단어\u2060이음", 2},
		{"sequenceDiagram\nA->>B: 줄\u2028나눔", 2},
		{"sequenceDiagram\n%% 주석\u3000메모", 2}, // 주석 줄도 같은 규칙(단순화)
		{"sequenceDiagram\nloop 매번\u202f확인\nA->>B: x\nend", 2},
	} {
		errAt(t, tc.src, tc.line, "ASCII 밖 공백·서식 문자")
	}
	// 대조군 — ASCII 공백만이면 그대로 그린다(빈 블록은 앞 설명 유지)
	if svg, issues := drawn(t, keep+"accDescr { \t }\nA->>A: x"); !strings.Contains(svg, "<title>keep</title>") || len(issues) != 0 {
		t.Errorf("ASCII 공백 빈 블록: %v\n%s", issues, svg)
	}
}

// ---- 독립 판정기(itda-hyve#208 두 번째 결정 — 허용 목록 문법). 시험 안에 허용 꼴을 따로 옮긴 정규식으로 수락 여부와 값을 구하고
// Render(구조)와 전부 대조한다. 구현 코드·상수(정규식·색 목록·예약어)를 가져다 쓰지 않는다.

const jID = `[\p{L}_][\p{L}\p{N}_]*`

var (
	jReDecl  = jRe(`^(?i:(participant|actor))\s+(` + jID + `)(?:\s+(?i:as)\s+(.+))?$`)
	jReMsg   = regexp.MustCompile(`^(` + jID + `)\s*(->>|<<->>|-->>|<<-->>|->|-->|-[xX]|--[xX]|-\)|--\))\s*([+-]?)\s*(` + jID + `)\s*:(.*)$`)
	jReNote  = jRe(`^(?i:note)\s+((?i:left of|right of|over))\s+(` + jID + `)(?:\s*,\s*(` + jID + `))?\s*:(.*)$`)
	jReAct   = jRe(`^((?i:activate|deactivate))\s+(` + jID + `)$`)
	jReBlock = jRe(`^((?i:loop|opt|alt|par|critical|break))(?:\s+(.*))?$`)
	jReSect  = jRe(`^((?i:else|and|option))(?:\s+(.*))?$`)
	jReRect  = jRe(`^(?i:rect)\s+(.+)$`)
	jReBox   = jRe(`^(?i:box)(?:\s+(.*))?$`)
	jReAuto  = jRe(`^(?i:autonumber)(?:\s+((?i:off))|\s+([0-9]{1,6})(?: +([0-9]{1,6}))?)?$`)
	jReTitle = jRe(`^(?i:title)(?::\s+|\s+)(.+)$`)
	jReLink  = jRe(`^(?i:links|link|properties|details)[ \t]+(` + jID + `)\s*:`)
	jReAccT  = jRe(`^(?i:acctitle)\s*:\s*(.*)$`)
	jReAccD  = jRe(`^(?i:accdescr)\s*:\s*(.*)$`)
	jReAccB  = jRe(`^(?i:accdescr)\s*\{(.*)$`)
	jReEnt   = regexp.MustCompile(`#\w+;`)
	jReWrap  = jRe(`^(?i::?(?:no)?wrap:)`)
	// box 머리 — 원본 sequenceDb.ts:400 의 정규식을 다시 옮긴 것.
	jReBoxHead = regexp.MustCompile(`^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$`)
	jReFnRGB   = regexp.MustCompile(`^rgba?\(\s*(?:-?\d+\s*,\s*-?\d+\s*,\s*-?\d+|-?\d+%\s*,\s*-?\d+%\s*,\s*-?\d+%)(?:\s*,\s*-?(?:\d+(?:\.\d+)?|\.\d+)%?)?\s*\)$`)
	jReFnHSL   = regexp.MustCompile(`^hsla?\(\s*-?\d+\s*,\s*-?\d+%\s*,\s*-?\d+%(?:\s*,\s*-?(?:\d+(?:\.\d+)?|\.\d+)%?)?\s*\)$`)
)

// jRe 는 시험 정규식을 만든다 — (?i:…) 묶음 안의 ASCII 글자만 [aA] 꼴로 펼친다. 원본 lexer 의 대소문자 무시는 JS i 깃발(u
// 없음)이라 ASCII 만 접으므로 Go 의 (?i)(유니코드 접기: ſ→s, K U+212A→k)를 쓰지 않는다(리뷰 4회차 [1]).
func jRe(pat string) *regexp.Regexp {
	var b strings.Builder
	depth := 0 // (?i: 묶음 안에서의 괄호 깊이(0 이면 밖)
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch {
		case c == '\\' && i+1 < len(pat):
			b.WriteString(pat[i : i+2])
			i++
		case depth == 0 && strings.HasPrefix(pat[i:], "(?i:"):
			b.WriteString("(?:")
			i += 3
			depth = 1
		case depth > 0 && c == '(':
			depth++
			b.WriteByte(c)
		case depth > 0 && c == ')':
			depth--
			b.WriteByte(c)
		case depth > 0 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'):
			b.WriteString("[" + string(c|0x20) + string(c&^0x20) + "]")
		default:
			b.WriteByte(c)
		}
	}
	return regexp.MustCompile(b.String())
}

// jLower 는 ASCII 대문자만 소문자로 바꾼다(시험 쪽 따로 구현).
func jLower(s string) string {
	r := []rune(s)
	for i, c := range r {
		if 'A' <= c && c <= 'Z' {
			r[i] = c + 32
		}
	}
	return string(r)
}

// jColorNames 는 시험 쪽 CSS 색 키워드 목록이다 — 구현 목록과 따로 옮겼다. CSS Color 4 §6.1 명색 148개 + transparent,
// §6.4 currentcolor, §6.2 시스템 색 19개 + 부록 A 폐기 예정 시스템 색 23개(소문자로 적음), CSS Cascade 5 전체 키워드.
var jColorNames = strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue blueviolet brown
burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray
darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen darkslateblue
darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite
forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory
khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow lightgray lightgreen
lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime
limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue
mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive
olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum
powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue
slateblue slategray slategrey snow springgreen steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke
yellow yellowgreen transparent currentcolor inherit initial unset revert revert-layer
accentcolor accentcolortext activetext buttonborder buttonface buttontext canvas canvastext field fieldtext graytext highlight
highlighttext linktext mark marktext selecteditem selecteditemtext visitedtext
activeborder activecaption appworkspace background buttonhighlight buttonshadow captiontext inactiveborder inactivecaption
inactivecaptiontext infobackground infotext menu menutext scrollbar threeddarkshadow threedface threedhighlight
threedlightshadow threedshadow window windowframe windowtext`)

// jKeywordsB 는 원본 lexer 에서 \b 가 붙어 ACTOR 보다 먼저 읽히는 지시자, jReserved 는 그 밖에 이름으로 받지 않는 낱말.
var (
	jKeywordsB = strings.Fields(`box participant actor create destroy loop rect opt alt else par par_over and critical option break
end over note activate deactivate sequencediagram autonumber off`)
	jReserved = strings.Fields(`title acctitle accdescr links link properties details as`)
)

func jIsColor(s string) bool {
	return containsStr(jColorNames, jLower(s)) || jReFnRGB.MatchString(s) || jReFnHSL.MatchString(s)
}

// jIDOK — 예약어 그 자체, 또는 \b 지시자 + ASCII 낱말 글자가 아닌 글자로 시작하는 이름은 받지 않는다.
func jIDOK(id string) bool {
	low := ""
	for _, r := range id {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		low += string(r)
	}
	if containsStr(jReserved, low) || containsStr(jKeywordsB, low) {
		return false
	}
	for _, k := range jKeywordsB {
		if strings.HasPrefix(low, k) {
			next := low[len(k)]
			if !(next == '_' || next >= '0' && next <= '9' || next >= 'a' && next <= 'z') {
				return false
			}
		}
	}
	return true
}

// jEnt 는 글의 엔티티 토큰(#\w+;)을 정책 B 로 한 번 푼다 — 이름 7개(대소문자 구분)와 십진 7자리 이하의 수 가운데 공백·제어가
// 아닌 글자. 생성기 낱말(#35;·#58;·#59;·#169;·#160;·#32;·#copy;·#nbsp;·#Quot;·#quot;)에 맞춘 손 규칙이다(구현의 표를 쓰지 않는다).
// 받지 않는 엔티티가 있으면 false.
func jEnt(s string) (string, bool) {
	names := map[string]string{"quot": "\"", "amp": "&", "lt": "<", "gt": ">", "apos": "'", "num": "#", "semi": ";"}
	ok := true
	out := jReEnt.ReplaceAllStringFunc(s, func(e string) string {
		name := e[1 : len(e)-1]
		if v, found := names[name]; found {
			return v
		}
		n, err := strconv.Atoi(name)
		if err != nil || len(name) > 7 || n < 0x21 || n >= 0x7f && n <= 0xa0 || n == 0x2003 {
			ok = false
			return ""
		}
		return string(rune(n))
	})
	return out, ok
}

// jDecode 는 보이는 라벨(앞뒤 빈칸·탭을 지운 것)의 값이다(정책 B) — 안쪽에 탭·연속 빈칸이 있으면 false. <br> 은 줄바꿈이고 그
// 앞뒤 빈칸 한 칸은 줄바꿈이 대신한다. 엔티티는 줄마다 jEnt 로 한 번.
func jDecode(s string) (string, bool) {
	if strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return "", false
	}
	lines := strings.Split(s, "<br>")
	for i, l := range lines {
		v, ok := jEnt(strings.Trim(l, " "))
		if !ok {
			return "", false
		}
		lines[i] = v
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n"), true
}

// jAcc 는 접근성 값(원본처럼 앞뒤 빈칸·탭을 지운 것)이다(정책 B) — 안쪽 빈칸·탭은 그대로, 맨 &·<·> 는 false, 엔티티는 jEnt.
func jAcc(v string) (string, bool) {
	if strings.ContainsAny(v, "&<>") {
		return "", false
	}
	return jEnt(v)
}

// jWant 는 판정기가 받아들인 줄의 해석이다(kind 별로 쓰는 칸만 채운다).
type jWant struct {
	kind              string // decl·msg·note·act·block·sect·end·rect·box·auto·title·accT·accD·comment·init·link
	id, name          string
	actor             bool
	keep              bool // accD — 빈 블록이라 앞 설명을 그대로 둔다(원본은 값 토큰이 없다 — jison:76–78)
	from, to          string
	dotted, both      bool
	head              byte
	mark              string
	text              string
	placement, ids    string
	block             string
	off, haveStart    bool
	on                bool
	numStart, numStep float64
	boxColor          bool
}

// jJudge 는 줄 하나의 판정이다(ok=false 면 원문 + 경고여야 한다).
// jOddRunes 는 시험 쪽 「ASCII 밖 공백·서식 문자」 목록이다 — 구현(unicode.IsSpace 사용)과 따로 코드 포인트를 적었다.
// ECMAScript WhiteSpace·LineTerminator 의 ASCII 밖(U+00A0·1680·2000–200A·2028·2029·202F·205F·3000·FEFF),
// Go unicode.IsSpace 에만 있는 U+0085, 서식 문자 U+200B–200D·U+2060.
var jOddRunes = []rune{0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009,
	0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff, 0x200b, 0x200c, 0x200d, 0x2060}

// jSpice 는 생성한 줄의 아무 글자 경계에 jOddRunes 한 글자를 가끔 끼운다(리뷰 6회차 [1]).
func jSpice(rng *rand.Rand, line string) string {
	if rng.Intn(8) != 0 {
		return line
	}
	rs := []rune(line)
	at := rng.Intn(len(rs) + 1)
	odd := jOddRunes[rng.Intn(len(jOddRunes))]
	return string(rs[:at]) + string(odd) + string(rs[at:])
}

func jJudge(line string) (jWant, bool) {
	for _, r := range line {
		if containsRune(jOddRunes, r) {
			return jWant{}, false // ASCII 밖 공백·서식 문자가 든 줄은 원문(2026-10-30 결정 — 리뷰 6회차 [1])
		}
	}
	t := strings.Trim(line, " \t")
	label := func(s string) (string, bool) {
		v := strings.Trim(s, " \t")
		if jReWrap.MatchString(v) {
			return "", false
		}
		return jDecode(v)
	}
	if strings.HasPrefix(t, "%%") {
		if strings.HasPrefix(t, "%%{") {
			return jWant{kind: "init"}, strings.HasSuffix(t, "}%%")
		}
		return jWant{kind: "comment"}, true
	}
	if m := jReAccT.FindStringSubmatch(t); m != nil {
		v, ok := jAcc(strings.Trim(m[1], " \t"))
		return jWant{kind: "accT", text: v}, ok && m[1] != ""
	}
	if m := jReAccD.FindStringSubmatch(t); m != nil {
		v, ok := jAcc(strings.Trim(m[1], " \t"))
		return jWant{kind: "accD", text: v}, ok && m[1] != ""
	}
	if m := jReAccB.FindStringSubmatch(t); m != nil {
		in, after, closed := strings.Cut(m[1], "}")
		in = strings.Trim(in, " \t")
		v, ok := jAcc(in)
		return jWant{kind: "accD", text: v, keep: in == ""}, closed && strings.TrimSpace(after) == "" && (ok || in == "")
	}
	bare := jReEnt.ReplaceAllString(t, "")
	if strings.ContainsAny(bare, "#;") || strings.Contains(t, "%%") {
		return jWant{}, false
	}
	if m := jReDecl.FindStringSubmatch(t); m != nil {
		w := jWant{kind: "decl", id: m[2], name: m[2], actor: jLower(m[1]) == "actor"}
		ok := jIDOK(m[2])
		if m[3] != "" {
			var lok bool
			w.name, lok = label(m[3])
			ok = ok && lok
		}
		return w, ok
	}
	if m := jReNote.FindStringSubmatch(t); m != nil {
		w := jWant{kind: "note", placement: jLower(m[1]), ids: m[2]}
		ok := jIDOK(m[2]) && (m[3] == "" || w.placement == "over" && jIDOK(m[3]))
		if m[3] != "" {
			w.ids += "," + m[3]
		}
		var lok bool
		w.text, lok = label(m[4])
		return w, ok && lok
	}
	if m := jReAct.FindStringSubmatch(t); m != nil {
		return jWant{kind: "act", id: m[2], on: jLower(m[1]) == "activate"}, jIDOK(m[2])
	}
	if m := jReBlock.FindStringSubmatch(t); m != nil {
		v, ok := label(m[2])
		return jWant{kind: "block", block: jLower(m[1]), text: v}, ok
	}
	if m := jReSect.FindStringSubmatch(t); m != nil {
		v, ok := label(m[2])
		return jWant{kind: "sect", block: jLower(m[1]), text: v}, ok
	}
	if jLower(t) == "end" {
		return jWant{kind: "end"}, true
	}
	if m := jReRect.FindStringSubmatch(t); m != nil {
		return jWant{kind: "rect"}, jIsColor(m[1])
	}
	if m := jReBox.FindStringSubmatch(t); m != nil {
		h := jReBoxHead.FindStringSubmatch(m[1])
		cand, rest := h[1], h[2]
		sp := rest == "" || rest[0] == ' ' || rest[0] == '\t'
		w := jWant{kind: "box"}
		var raw string
		switch {
		case cand == "":
			raw = m[1]
		case sp && jIsColor(cand):
			raw, w.boxColor = rest, true
		case sp && regexp.MustCompile(`^\w+$`).MatchString(cand) && !containsStr(jColorNames, jLower(cand)):
			raw = m[1]
		default:
			return w, false
		}
		var ok bool
		w.name, ok = label(raw)
		return w, ok
	}
	if m := jReAuto.FindStringSubmatch(t); m != nil {
		w := jWant{kind: "auto", off: m[1] != ""}
		if m[2] != "" {
			// 원본 NUM 의 (?=[ \n]+) — 마지막 수 뒤 줄 끝 공백이 빈칸으로 시작해야 한다(탭이면 수가 아니다)
			if tail := strings.TrimLeft(line, " \t")[len(t):]; tail != "" && tail[0] != ' ' {
				return w, false
			}
			w.haveStart, w.numStep = true, 1
			w.numStart, _ = strconv.ParseFloat(m[2], 64)
			if m[3] != "" {
				w.numStep, _ = strconv.ParseFloat(m[3], 64)
			}
		}
		return w, true
	}
	if m := jReTitle.FindStringSubmatch(t); m != nil {
		v, ok := label(m[1])
		return jWant{kind: "title", text: strings.ReplaceAll(v, "\n", " ")}, ok
	}
	if m := jReLink.FindStringSubmatch(t); m != nil {
		return jWant{kind: "link", id: m[1]}, jIDOK(m[1])
	}
	if m := jReMsg.FindStringSubmatch(t); m != nil {
		a := jLower(m[2])
		w := jWant{kind: "msg", from: m[1], to: m[4], mark: m[3], dotted: strings.HasPrefix(strings.TrimPrefix(a, "<<"), "--"),
			both: strings.HasPrefix(a, "<<")}
		switch a[len(a)-1] {
		case '>':
			w.head = 'f'
			if !strings.HasSuffix(a, ">>") {
				w.head = 'n'
			}
		case 'x':
			w.head = 'x'
		case ')':
			w.head = 'o'
		}
		v, ok := label(m[5])
		w.text = v
		// 끝 표시 - 뒤 x 로 시작하는 이름 — 원본은 -x(가위표 토큰)를 먼저 읽는다
		xAfterMinus := m[3] == "-" && strings.HasPrefix(jLower(m[4]), "x")
		return w, ok && jIDOK(m[1]) && jIDOK(m[4]) && !xAfterMinus
	}
	return jWant{}, false
}

// jGen 은 경계 낱말을 명시적으로 조합한 줄을 만든다 — 빈칸(없음·빈칸·탭) × ; # %% } : , 숫자 시작 - . 예약어
// as/as;/as  wrap: 색 후보 한글.
func jGen(rng *rand.Rand) string {
	pick := func(xs ...string) string { return xs[rng.Intn(len(xs))] }
	sp := func() string { return pick("", " ", "\t", "  ") }
	sp1 := func() string { return pick(" ", "\t", " \t") }
	good := func() string {
		return pick("A", "B", "가나", "P1", "_x", "Q_2", "end_1", "endpoint", "parx", "Ka", "이름2", "xB", "X1")
	}
	id := func() string {
		if rng.Intn(2) == 0 {
			return good()
		}
		return pick("A", "B", "가나", "P1", "_x", "Q_2", "end", "End", "end가", "end_1", "endpoint", "as", "5", "5x", "P-2", "A B",
			"x.y", "title", "note", "loopé", "off", "Office", "A#35;", "par_over", "parx", "alt가", "link", "Ka", "ſx", "accTitle", "가%%",
			"\u212ax", "brea\u212a", "breaK", "el\u017fe")
	}
	lbl := func() string {
		if rng.Intn(2) == 0 {
			return pick("안녕", "hello world", "10:30 회의", "", "비율 #35;", "첫째<br>둘째", "첫째 <br> 둘째")
		}
		return pick("안녕", "hello world", "10:30 회의", "", "비율 #35;", "첫째<br>둘째", "wrap: 앞말", "Wrap:x", ":nowrap: 글", "a;b",
			"a # b", "a %% b", "x}", "콜론: 뒤", "A->>B: 안", "as", "  여백  ", "쉼표, 글", "#59; 엔티티", "가ſ",
			// 정책 B 경계 — 안쪽 연속 빈칸·탭, 받지 않는 엔티티(이름·공백 수·대소문자), 한 번만 푸는 엔티티
			"x  y", "x\ty", "#copy; 2026", "x#160;y", "x#32;y", "#nbsp;", "#Quot;", "#quot;인용#quot;", "#169;", "#35;35;")
	}
	color := func() string {
		return pick("red", "Blue", "transparent", "grey", "teal", "notacolor", "rgb(1,2,3)", "rgb(1, 2%, 3)", "rgba(0,0,255,.1)",
			"hsl(120,50%,50%)", "hsl(120,50,50)", "rgb(1)", "red-team", "RGB(1,2,3)", "rgb (1,2,3)", "inherit", "팀",
			// 시스템 색·폐기 예정 시스템 색·revert-layer·ASCII 밖 접기(리뷰 4회차 [1][2])
			"Canvas", "ButtonText", "canvastext", "AccentColor", "WindowText", "ThreeDFace", "revert-layer", "Team", "blac\u212a",
			"blacK", "\u017filver")
	}
	tail := func() string { return pick("", "", "", "", "", "", ";", " ;", "#c", " %% c", "}", " x", ";A->>B: y") }
	arrow := func() string {
		return pick("->>", "-->>", "->", "-->", "-x", "--x", "-X", "-)", "--)", "<<->>", "<<-->>", "-|\\", "--|/", "()->>", "->>()")
	}
	switch rng.Intn(16) {
	case 0, 1:
		s := pick("participant", "actor", "Participant", "ACTOR") + sp1() + id()
		switch rng.Intn(5) {
		case 0:
			s += sp1() + pick("as", "AS") + sp1() + lbl()
		case 1:
			s += pick(" as", " as;", " as ", " as \t", " as;B")
		}
		return s + tail()
	case 2, 3, 4:
		return id() + sp() + arrow() + sp() + pick("", "", "+", "-") + sp() + id() + sp() + pick(":", ":", ":", "") + sp() + lbl() + tail()
	case 5:
		s := pick("Note", "note", "NOTE") + sp1() + pick("left of", "right of", "over", "Over", "left  of", "above") + sp1() + id()
		if rng.Intn(3) == 0 {
			s += sp() + "," + sp() + id()
		}
		return s + sp() + pick(":", ":", "") + sp() + lbl()
	case 6:
		return pick("activate", "deactivate", "Activate") + sp1() + id() + tail()
	case 7:
		s := pick("loop", "opt", "alt", "par", "critical", "break", "Loop", "par_over", "loopx", "breaK", "BREAK", "brea\u212a")
		if rng.Intn(4) > 0 {
			s += sp1() + lbl()
		}
		return s + tail()
	case 8:
		s := pick("else", "and", "option", "Else", "ELSE", "el\u017fe", "EL\u017fE")
		if rng.Intn(3) > 0 {
			s += sp1() + lbl()
		}
		return s + tail()
	case 9:
		return pick("end", "End", "end x", "end;", "end #c", "endx")
	case 10:
		return pick("rect", "Rect", "RECT") + sp1() + color() + tail()
	case 11:
		s := pick("box", "Box", "BOX")
		if rng.Intn(3) > 0 {
			s += sp1() + color()
		}
		if rng.Intn(2) == 0 {
			s += sp1() + lbl()
		}
		return s + tail()
	case 12:
		return pick("autonumber", "Autonumber") + pick("", sp1()+pick("10", "0", "999999", "1000000", "10x", "1.5", "off", "OFF", "10 5",
			"10\t5", "10  5", "0 0", "10 5 2"), "") + pick("", " ", "\t", ";", " #c")
	case 13:
		return pick("title", "Title", "title:", "TITLE:") + pick(" ", "", "\t") + lbl()
	case 14:
		switch rng.Intn(5) {
		case 0:
			return pick("accTitle", "ACCTITLE", "accTitle ") + ":" + sp() + lbl() + tail()
		case 1:
			return pick("accDescr", "accdescr", "accDe\u017fcr") + sp() + ":" + sp() + lbl() + tail()
		case 2:
			body := lbl()
			if rng.Intn(2) == 0 {
				body = pick("", " ", "\t", "  ", "\ufeff", " \ufeff ", "\u0085", "\u00a0") // 빈 블록 — 앞 설명을 그대로 둔다(리뷰 5회차 [1]); ASCII 밖 공백은 원문(6회차 [1])
			}
			return "accDescr" + sp() + "{" + sp() + body + sp() + pick("}", "} ", "}"+id()+"->>B: x", "} end", "}x")
		case 3:
			return pick("%% 주석", "%%{init: {\"theme\":\"dark\"}}%%", "%%{init: {", "%% # ; 주석")
		default:
			return pick("link", "links", "properties", "details", "Link", "linK", "LINKS", "lin\u212a", "detail\u017f") + sp1() + id() + sp() + pick(":", ":", "") + sp() + pick("문서 @ https://x.example", "{\"k\": \"v\"}", "x;y")
		}
	default: // 낱말 국 — 경계 낱말을 아무렇게나
		var b strings.Builder
		for i := 0; i < 1+rng.Intn(5); i++ {
			b.WriteString(pick("end", "loop", "A", "가", "->>", ":", ";", "#", "%%", "}", ",", "5", "-", ".", "as", "wrap:", "red", "rgb(1,2,3)", "note", "over"))
			b.WriteString(sp())
		}
		return b.String()
	}
}

// TestJudgeBoundaryProperties — 생성한 줄마다 판정기(jJudge)와 Render 를 대조한다. 판정기가 거부하면 Render 는 그 줄
// 번호의 *Error(원문 + 경고)여야 하고, 받아들이면 Render 가 성공하고 줄의 값(참여자 ID·별명·모양, 메시지 from/to/화살표
// 종류/활성 표시/글, 메모 자리·대상·글, 묶음 종류·조건, autonumber 값, title·acc 값, box 이름·색 경고, 링크 메뉴 참여자)
// 이 구조(chart)와 같아야 한다.
func TestJudgeBoundaryProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(2032))
	count := map[string]int{}
	accepted, rejected := 0, 0
	for round := 0; round < 15000; round++ { // 정책 B 경계 낱말(거부)을 더해 수락 표본이 줄어 판 수를 늘렸다
		base := jGen(rng)
		line := jSpice(rng, base)
		if _, baseOK := jJudge(base); baseOK && line != base {
			count["oddFlip"]++ // 받아들일 줄이 ASCII 밖 공백·서식 문자 한 글자로 원문이 되는 판
		}
		if strings.ContainsAny(line, "\n") {
			t.Fatalf("생성기가 줄바꿈을 만듦: %q", line)
		}
		w, ok := jJudge(line)
		// 골격 — 줄의 문맥(묶음 안·box 닫기·활성 대상)을 머리 낱말로 정하고 판정과 무관하게 붙인다.
		pre, post := "sequenceDiagram\nparticipant P0\n", "\nP0->>P0: 닫기"
		head := jLower(firstWord(strings.Trim(line, " \t")))
		switch head {
		case "else":
			pre += "alt 둘레\n"
			post = "\nend" + post
		case "and":
			pre += "par 둘레\n"
			post = "\nend" + post
		case "option":
			pre += "critical 둘레\n"
			post = "\nend" + post
		case "end":
			pre += "loop 둘레\n"
		case "loop", "opt", "alt", "par", "critical", "break", "rect":
			post = "\nend" + post
		case "box":
			post = "\nparticipant BX\nend" + post
		}
		if ok && w.kind == "act" {
			pre += "participant " + w.id + "\n"
			if !w.on {
				pre += "activate " + w.id + "\n"
			}
		}
		if ok && w.kind == "msg" && w.mark == "-" {
			pre += "participant " + w.from + "\nactivate " + w.from + "\n"
		}
		// 접근성 설명의 상태 전이 — 앞 설명(없음·콜론형·블록형)을 판정과 무관하게 붙이고, 빈 블록이면 앞 설명이 남는다고
		// 따로 계산한다(리뷰 5회차 [1]).
		if strings.HasPrefix(jLower(strings.Trim(line, " \t")), "accdescr") {
			prev := ""
			switch rng.Intn(3) {
			case 1:
				prev = "앞 설명"
				pre += "accDescr: " + prev + "\n"
			case 2:
				prev = "블록 앞 설명"
				pre += "accDescr {\n  " + prev + "\n}\n"
			}
			if ok && w.keep {
				w.text = prev
				if prev != "" {
					count["accKeep"]++ // 빈 블록이 앞 설명을 남기는 판
				}
			}
		}
		lineNo := strings.Count(pre, "\n") + 1
		src := pre + line + post
		c, issues, err := Parse(src)
		if !ok {
			rejected++
			var de *diagram.Error
			if !errors.As(err, &de) || de.Line != lineNo {
				t.Fatalf("round %d: 판정기가 거부한 줄 %q — Render 가 그 줄에서 원문 + 경고가 아님: %v\n%s", round, line, err, src)
			}
			continue
		}
		if err != nil {
			t.Fatalf("round %d: 판정기가 받아들인 줄 %q 를 Render 가 거부: %v\n%s", round, line, err, src)
		}
		accepted++
		count[w.kind]++
		jCompare(t, round, line, w, c, issues)
	}
	t.Logf("판 %d — 수락 %d·거부 %d, 수락 종류 %v", accepted+rejected, accepted, rejected, count)
	for _, k := range []string{"decl", "msg", "note", "act", "block", "sect", "end", "rect", "box", "auto", "title", "accT", "accD", "comment", "init", "link"} {
		if count[k] < 20 {
			t.Errorf("수락 표본 %s 가 %d 판 — 모자람", k, count[k])
		}
	}
	if count["oddFlip"] < 100 {
		t.Errorf("ASCII 밖 공백·서식 문자로 뒤집힌 표본이 %d 판 — 모자람", count["oddFlip"])
	}
	if count["accKeep"] < 5 {
		t.Errorf("빈 accDescr 블록 + 앞 설명 표본이 %d 판 — 모자람", count["accKeep"])
	}
	if accepted < 1500 || rejected < 1500 {
		t.Errorf("표본 부족 — 수락 %d·거부 %d", accepted, rejected)
	}
}

// jCompare 는 받아들인 줄의 값이 Render 구조와 같은지 본다.
func jCompare(t *testing.T, round int, line string, w jWant, c *chart, issues []diagram.Issue) {
	t.Helper()
	fail := func(format string, a ...any) {
		t.Fatalf("round %d: 줄 %q — %s", round, line, fmt.Sprintf(format, a...))
	}
	actor := func(id string) *participant {
		for _, a := range c.Actors {
			if a.ID == id {
				return a
			}
		}
		fail("참여자 %q 가 없음", id)
		return nil
	}
	evs := c.Events[:len(c.Events)-1] // 맨 끝 골격 「P0->>P0: 닫기」를 뺀다(문맥 줄의 사건은 남는다 — 뒤에서부터 찾는다)
	find := func(kind evKind) *event {
		for i := len(evs) - 1; i >= 0; i-- {
			if evs[i].kind == kind {
				return &evs[i]
			}
		}
		fail("사건 %d 가 없음", kind)
		return nil
	}
	hasIssue := func(s string) bool {
		for _, is := range issues {
			if strings.Contains(is.Msg, s) {
				return true
			}
		}
		return false
	}
	blockOf := map[string]int{"loop": bLoop, "opt": bOpt, "alt": bAlt, "par": bPar, "critical": bCritical, "break": bBreak,
		"else": bAlt, "and": bPar, "option": bCritical}
	switch w.kind {
	case "decl":
		a := actor(w.id)
		if a.Name != w.name || a.Actor != w.actor {
			fail("참여자 %q 이름·모양 %q/%v, 기대 %q/%v", w.id, a.Name, a.Actor, w.name, w.actor)
		}
	case "msg":
		ev := find(evMessage)
		mark := map[int]string{0: "", 1: "+", -1: "-"}[ev.activate]
		if ev.from != w.from || ev.to != w.to || ev.text != w.text || mark != w.mark ||
			ev.ar.Dotted != w.dotted || ev.ar.Both != w.both || ev.ar.Head != w.head {
			fail("메시지 %+v, 기대 %+v", *ev, w)
		}
		actor(w.from)
		actor(w.to)
	case "note":
		ev := find(evNote)
		ids := ev.from
		if ev.over2 != "" {
			ids += "," + ev.over2
		}
		place := map[byte]string{leftOf: "left of", rightOf: "right of", over: "over"}[ev.placement]
		if place != w.placement || ids != w.ids || ev.text != w.text {
			fail("메모 %s %s %q, 기대 %s %s %q", place, ids, ev.text, w.placement, w.ids, w.text)
		}
	case "act":
		kind := evActiveEnd
		if w.on {
			kind = evActiveStart
		}
		if ev := evs[len(evs)-1]; ev.kind != kind || ev.from != w.id {
			fail("활성 %+v, 기대 %s %v", ev, w.id, w.on)
		}
	case "block", "sect":
		kind := evBlockStart
		if w.kind == "sect" {
			kind = evBlockElse
		}
		ev := find(kind)
		if ev.block != blockOf[w.block] || ev.text != w.text {
			fail("묶음 %s %q, 기대 %s %q", blockName[ev.block], ev.text, w.block, w.text)
		}
	case "end":
		find(evBlockEnd)
	case "rect":
		if ev := find(evBlockStart); ev.block != bRect || !hasIssue("rect 의 색") {
			fail("rect 띠·색 경고가 다름 %+v %v", *ev, issues)
		}
	case "box":
		if len(c.Boxes) != 1 || c.Boxes[0].Name != w.name || hasIssue("box 의 색") != w.boxColor {
			fail("box %+v·경고 %v, 기대 이름 %q 색 %v", c.Boxes, issues, w.name, w.boxColor)
		}
	case "auto":
		ev := find(evAutonumber)
		if ev.numOff != w.off || ev.haveStart != w.haveStart || ev.numStart != w.numStart || ev.numStep != w.numStep && w.haveStart {
			fail("autonumber %+v, 기대 %+v", *ev, w)
		}
	case "title":
		if c.Title != w.text {
			fail("title %q, 기대 %q", c.Title, w.text)
		}
	case "accT":
		if c.AccTitle != w.text {
			fail("accTitle %q, 기대 %q", c.AccTitle, w.text)
		}
	case "accD":
		if c.AccDescr != w.text {
			fail("accDescr %q, 기대 %q", c.AccDescr, w.text)
		}
	case "comment", "init":
		if len(evs) != 0 || len(c.Actors) != 1 || (w.kind == "init") != hasIssue("설정 지시") {
			fail("주석이 사건·참여자·경고를 만듦: %+v %v", evs, issues)
		}
	case "link":
		actor(w.id)
		if !hasIssue("링크 메뉴") {
			fail("링크 메뉴 경고 없음 %v", issues)
		}
	}
	// 판정기가 받아들인 모든 줄에서 활성 문맥 밖의 남는 사건이 없는지 — 메시지 줄은 메시지(+활성) 하나다
	if w.kind == "msg" {
		n := 0
		for _, ev := range evs {
			if ev.kind == evMessage {
				n++
			}
		}
		if n != 1 {
			fail("메시지가 %d 개 생김(유령 문장)", n)
		}
	}
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func containsRune(xs []rune, r rune) bool {
	for _, x := range xs {
		if x == r {
			return true
		}
	}
	return false
}

// TestDirectiveLines 는 er ④ 리뷰 1회차에서 찾은 설정 지시 구멍을 순서도에서도 막는다(공통 diagram.DirectiveProblem) —
// 줄 전체가 지시 하나(accDescr 블록 밖)일 때만 무시하고 그린다. 지시 사이의 문장이 사라지거나 acc 값에 지시가 남던 것.
func TestDirectiveLines(t *testing.T) {
	const d = "%%{init: {}}%%"
	const one = "한 줄에 하나만"
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"sequenceDiagram\n" + d + " A->>B: x " + d + "\nC->>D: y", 2, one},
		{"sequenceDiagram\n" + d + " ??? " + d + "\nC->>D: y", 2, one},
		{"sequenceDiagram\naccTitle: 앞 " + d + " 뒤\nC->>D: y", 2, one},
		{"sequenceDiagram\naccDescr: 앞 " + d + " 뒤\nC->>D: y", 2, one},
		{"sequenceDiagram\nC->>D: 글 " + d, 2, one},
		{"sequenceDiagram\nNote over C: 글 " + d + "\nC->>D: y", 2, one},
		{"sequenceDiagram\n%% 메모 " + d + "\nC->>D: y", 2, one},
		{"sequenceDiagram\naccDescr {\n" + d + "\n}\nC->>D: y", 3, "블록 안"},
		{"sequenceDiagram\n%%{init: {\nC->>D: y", 2, "한 줄로 닫아야"},
	} {
		errAt(t, c.src, c.line, c.msg)
	}
	for _, src := range []string{d + "\nsequenceDiagram\nC->>D: y", "sequenceDiagram\n  " + d + "\nC->>D: y"} {
		svg, iss := drawn(t, src)
		if len(iss) != 1 || !strings.Contains(iss[0].Msg, "설정 지시") || strings.Contains(svg, "%%{") {
			t.Errorf("%q: 경고 %+v", src, iss)
		}
	}
}

// 줄 머리 %% 주석 — 원본 cleanupComments 처럼 accDescr { … } 블록 안에서도 지운다(설명에 넣지 않는다). 블록 밖 주석은 그대로
// 건너뛰고, 뒤에 글이 없는 맨 %% 줄은 원문 + 그 줄 번호(er 와 같은 차례·규칙 — itda-hyve#409 ④-1 후속 리뷰 1회차 [2]).
func TestCommentLines(t *testing.T) {
	const head, tail = "sequenceDiagram\n", "\nC->>D: y"
	for _, c := range []struct{ src, desc string }{
		{head + "accDescr {\n설명\n%% 메모\n}" + tail, "설명"},
		{head + "accDescr {\n  %% 메모 }\n설명\n}" + tail, "설명"}, // 주석 줄은 줄째 지운다 — 안의 } 로 닫지 않는다
		{head + "accDescr { 앞\n%%  \n뒤 }" + tail, "앞\n뒤"},    // %% 뒤 공백만 있어도 원본 정규식은 지운다(블록 줄바꿈은 그대로 — 정책 B)
	} {
		r, err := Render(c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if !strings.Contains(r.SVG, "<title>"+c.desc+"</title>") || strings.Contains(r.SVG, "메모") || strings.Contains(r.SVG, "%%") {
			t.Errorf("%q: 설명이 %q 가 아니거나 주석이 남음\n%s", c.src, c.desc, r.SVG)
		}
	}
	// 블록 밖 맨 %% 는 원본 lexer 의 주석 규칙(jison:28)이 건너뛴다 — 경고 없이 그린다(itda-hyve#409 ④-1 후속 수정 2).
	for _, src := range []string{head + "%% 밖 주석" + tail, head + "%%" + tail, head + "  %%\r" + tail} {
		if r, err := Render(src); err != nil || len(r.Issues) != 0 {
			t.Errorf("%q 블록 밖 주석: %v %+v", src, err, r.Issues)
		}
	}
	// 블록 안 맨 %% 는 원본이 설명 글로 넣는다 — 이 판은 그리지 않는다(보수적).
	for _, c := range []struct {
		src  string
		line int
	}{{head + "accDescr {\n설명\n%%\n}" + tail, 4}, {head + "accDescr { 앞\n  %%\r\n뒤 }" + tail, 3}} {
		_, err := Render(c.src)
		var de *Error
		if !errors.As(err, &de) || de.Line != c.line || !strings.Contains(de.Msg, "블록 안의 글 없는") {
			t.Errorf("%q: 오류 %v, 기대 %d줄 블록 안의 글 없는 %%%% 줄", c.src, err, c.line)
		}
	}
}
