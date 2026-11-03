// Package diagram 은 mermaid 그림 종류(flowchart·gantt …)가 함께 쓰는 부분이다(itda-hyve#409) — 공개 꼴(결과·경고·오류),
// 공통 상한, 글자 폭·줄바꿈, SVG 쓰기 도구, frontmatter·접근성 문장 읽기, 종류 판별.
// 종류마다의 파서·그리기는 diagram/flowchart·diagram/gantt 이 맡는다(공통 부분을 복제하지 않게 여기에 둔다).
//
// 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리, 오늘 날짜·난수·로캘·환경변일 읽지 않는다).
package diagram

import (
	"fmt"
	"strings"
)

// 공통 상한. 종류마다의 요소 상한(노드 수·작업 수 …)은 각 패키지에 있다.
const (
	MaxSource = 16 << 10 // 그림 하나의 원문(바이트)
	MaxLabel  = 200      // 라벨 하나의 글자 수(룬)
)

// Error 는 그리지 못한 까닭이다. Line 은 원문 안의 줄(1부터, 모르면 0)이다.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%d번째 줄: %s", e.Line, e.Msg)
	}
	return e.Msg
}

// Issue 는 그림은 그렸지만 쓰지 않은 줄 하나다(경고).
type Issue struct {
	Line int
	Msg  string
}

// Result 는 그린 그림이다.
type Result struct {
	SVG     string
	Caption string // frontmatter 의 title — 그림 캡션
	Issues  []Issue
}

// Kind 는 그림 원문의 종류(머리 낱말)를 돌려준다 — frontmatter(--- … ---)·주석(%%)·빈 줄을 건너뛴
// 첫 낱말. 머리가 없으면 ""(부르는 쪽이 그리지 못한다고 알린다). 종류끼리 같은 규칙으로 갈라 놓는다(mdoc).
func Kind(src string) string {
	lines := strings.Split(src, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end < 0 {
			return "" // 닫히지 않은 frontmatter — 종류를 알 수 없다
		}
		start = end + 1
	}
	for i := start; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		if j := strings.IndexAny(t, " \t"); j >= 0 {
			return t[:j]
		}
		return t
	}
	return ""
}

// Clip 은 경고에 넣을 원문 조각이다(40자까지).
func Clip(s string) string {
	r := []rune(s)
	if len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}

// Quote 는 경고에 넣을 이름 묶음이다.
func Quote(s string) string { return "“" + s + "”" }
