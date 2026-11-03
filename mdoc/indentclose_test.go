package mdoc

import (
	"reflect"
	"strings"
	"testing"
)

// itda-hyve#494 의 꼴 — 목록 바로 뒤에 빈 줄 없이 쓴 닫는 줄을 포매터가 4칸 들여써 :::notes 가 문서 끝까지 이어졌다.
const indentedCloseDoc = "## 하나\n\n본문\n\n:::notes\n\n### 노트 제목\n\n- 항목\n  - 아래 항목\n    :::\n\n## 둘\n\n본문\n\n## 셋\n\n### 셋의 아래\n"

func TestIndentedClose(t *testing.T) {
	r := render(t, indentedCloseDoc)
	want := []Warning{{Line: 11, Code: WarnIndentedClose,
		Message: "11줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — 5줄의 :::notes 가 문서 끝까지 이어짐(뒤 제목 3개가 그 안에 들어감). 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다",
		Fix:     &Fix{Line: 11, Old: "    :::", New: "\n:::"}}}
	if !reflect.DeepEqual(r.Warnings, want) {
		t.Fatalf("경고 %+v\n기대 %+v", r.Warnings, want)
	}
	// 고치기를 원문에 넣고 다시 그리면 경고가 없고 컨테이너가 닫혀 뒤 제목이 밖에 나온다
	fixed := applyFix(t, indentedCloseDoc, *r.Warnings[0].Fix)
	r2 := render(t, fixed)
	if len(r2.Warnings) != 0 {
		t.Fatalf("고친 뒤 경고 %+v", r2.Warnings)
	}
	if !strings.Contains(r2.HTML, "</div>\n<h2 id=\"둘\" data-line=\"14\">") {
		t.Errorf("고친 뒤 :::notes 가 둘 앞에서 닫히지 않음:\n%s", r2.HTML)
	}
}

func TestIndentedCloseCases(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []Warning
	}{
		{"앞 줄이 빈 줄이면 빈 줄을 더하지 않음", ":::box\n- 항목\n\n      :::\n\n# 뒤\n", []Warning{{Line: 4, Code: WarnIndentedClose,
			Message: "4줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — 1줄의 :::box 가 문서 끝까지 이어짐(뒤 제목 1개가 그 안에 들어감). 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다",
			Fix:     &Fix{Line: 4, Old: "      :::", New: ":::"}}}},
		{"제목이 없으면 제목 수를 적지 않음", ":::box\n문단\n    ::::  \n", []Warning{{Line: 3, Code: WarnIndentedClose,
			Message: "3줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — 1줄의 :::box 가 문서 끝까지 이어짐. 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다",
			Fix:     &Fix{Line: 3, Old: "    ::::  ", New: "\n::::  "}}}},
		{"탭 들여쓰기도", ":::box\n- 항목\n\t:::\n", []Warning{{Line: 3, Code: WarnIndentedClose,
			Message: "3줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — 1줄의 :::box 가 문서 끝까지 이어짐. 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다",
			Fix:     &Fix{Line: 3, Old: "\t:::", New: "\n:::"}}}},
		// 인용 줄(> )로 시작하는 줄은 들여쓴 닫는 줄 꼴이 아니다 — 보통 경고
		{"인용 안은 보통 경고", "> :::box\n> - 항목\n>       :::\n\n뒤\n", []Warning{{Line: 1, Code: WarnUnclosedContainer, Message: ":::box 가 닫히지 않아 감싼 블록 끝에서 닫음"}}},
		{"펜스 코드 안의 들여쓴 ::: 는 아님", ":::box\n```\n    :::\n```\n", []Warning{{Line: 1, Code: WarnUnclosedContainer, Message: ":::box 가 닫히지 않아 감싼 블록 끝에서 닫음"}}},
		{"이미 닫힌 컨테이너는 아님", ":::box\n- 항목\n    :::\n\n:::\n", nil},
		{"0~3칸 들여쓴 닫는 줄은 닫음", ":::box\n- 항목\n\n   :::\n# 뒤\n", nil},
		{"들여쓴 줄이 없으면 보통 경고", ":::box\n안\n", []Warning{{Line: 1, Code: WarnUnclosedContainer, Message: ":::box 가 닫히지 않아 감싼 블록 끝에서 닫음"}}},
		// 안쪽 컨테이너(닫힘) 안의 들여쓴 줄은 바깥 것이 아니다 — 바꾸면 안쪽이 먼저 닫힌다
		{"닫힌 안쪽 컨테이너 안의 줄은 아님", "::::a\n:::b\n- 항목\n    :::\n:::\n", []Warning{{Line: 1, Code: WarnUnclosedContainer, Message: ":::a 가 닫히지 않아 감싼 블록 끝에서 닫음"}}},
		// 둘 다 닫히지 않으면 각자 제 줄 — 안쪽은 제 안의 줄, 바깥은 바로 품은 줄
		{"겹친 둘", ":::notes\n- 가\n    :::\n\n## 둘\n\n:::notes\n- 나\n    :::\n\n## 셋\n", []Warning{
			{Line: 3, Code: WarnIndentedClose,
				Message: "3줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — 1줄의 :::notes 가 문서 끝까지 이어짐(뒤 제목 2개가 그 안에 들어감). 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다",
				Fix:     &Fix{Line: 3, Old: "    :::", New: "\n:::"}},
			{Line: 9, Code: WarnIndentedClose,
				Message: "9줄의 `:::` 가 4칸 이상 들여써져 닫는 줄로 읽히지 않음 — 7줄의 :::notes 가 감싼 블록 끝까지 이어짐(뒤 제목 1개가 그 안에 들어감). 이 줄로 닫으려던 것이라면 「고치기」로 들여쓰기를 줄이고 앞에 빈 줄을 둔다",
				Fix:     &Fix{Line: 9, Old: "    :::", New: "\n:::"}}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			got := r.Warnings
			if len(got) == 0 {
				got = nil
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("경고 %+v\n기대 %+v", got, c.want)
			}
		})
	}
}

// 고치기를 차례로(앞 줄부터 다시 그려 가며) 넣으면 경고가 모두 사라진다
func TestIndentedCloseFixAll(t *testing.T) {
	src := ":::notes\n- 가\n    :::\n\n## 둘\n\n:::notes\n- 나\n    :::\n\n## 셋\n"
	for i := 0; i < 5; i++ {
		r := render(t, src)
		var fix *Fix
		for _, w := range r.Warnings {
			if w.Fix != nil {
				fix = w.Fix
				break
			}
		}
		if fix == nil {
			if len(r.Warnings) != 0 {
				t.Fatalf("남은 경고 %+v", r.Warnings)
			}
			if strings.Count(r.HTML, "<div class=\"notes\"") != 2 || strings.Contains(r.HTML, "<div class=\"notes\" data-line=\"1\">\n<ul>\n<li>가</li>\n</ul>\n<h2") {
				t.Fatalf("노트 둘이 따로 닫히지 않음:\n%s", r.HTML)
			}
			return
		}
		src = applyFix(t, src, *fix)
	}
	t.Fatal("고치기가 끝나지 않음")
}

// applyFix 는 화면(호스트 앱)이 하는 것과 같게 Line 줄이 Old 일 때 New 로 바꾼다.
func applyFix(t *testing.T, src string, f Fix) string {
	t.Helper()
	ls := strings.Split(src, "\n")
	if f.Line < 1 || f.Line > len(ls) || ls[f.Line-1] != f.Old {
		t.Fatalf("Fix 의 줄 %d 이 %q 가 아님", f.Line, f.Old)
	}
	ls[f.Line-1] = f.New
	return strings.Join(ls, "\n")
}

// 부모(목록 항목·각주) 안의 컨테이너 — Fix 는 여는 줄의 앞 공백을 남기고 넘친 들여쓰기만 지운다. 고친 뒤 경고가 없고
// 닫는 줄 뒤의 글(after)이 부모 안에 남는다(itda-hyve#494 리뷰 1회차 [1]).
func TestIndentedCloseInParent(t *testing.T) {
	for _, c := range []struct {
		name, src string
		fix       Fix
		want      string // 고친 뒤 HTML 에 있어야 하는 조각 — after 가 부모 안에 남음
	}{
		{"목록 항목 안", "- item\n\n  :::box\n  text\n      :::\n\n  after\n\n# outside\n",
			Fix{Line: 5, Old: "      :::", New: "\n  :::"},
			"<div class=\"box\">\n<p>text</p>\n</div>\n<p>after</p>\n</li>\n</ul>\n<h1"},
		{"목록 항목 안 + 컨테이너 안 목록", "- item\n\n  :::box\n  - a\n        :::\n\n  after\n\n# outside\n",
			Fix{Line: 5, Old: "        :::", New: "\n  :::"},
			"<li>a</li>\n</ul>\n</div>\n<p>after</p>\n</li>\n</ul>\n<h1"},
		{"각주 안", "본문[^1]\n\n[^1]: 각주\n\n    :::box\n    text\n        :::\n\n    after\n\n# outside\n",
			Fix{Line: 7, Old: "        :::", New: "\n    :::"},
			"<div class=\"box\">\n<p>text</p>\n</div>\n<p>after&#160;<a href=\"#fnref:1\""},
		{"여는 줄이 더 들여써진 목록 안", "- item\n\n    :::box\n    text\n        :::\n\n  after\n",
			Fix{Line: 5, Old: "        :::", New: "\n    :::"},
			"<div class=\"box\">\n<p>text</p>\n</div>\n<p>after</p>\n</li>"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnIndentedClose || r.Warnings[0].Fix == nil || *r.Warnings[0].Fix != c.fix {
				t.Fatalf("경고 %+v\n기대 Fix %+v", r.Warnings, c.fix)
			}
			if !strings.Contains(r.Warnings[0].Message, "「고치기」") {
				t.Errorf("문구 %q", r.Warnings[0].Message)
			}
			r2 := render(t, applyFix(t, c.src, c.fix))
			if len(r2.Warnings) != 0 {
				t.Fatalf("고친 뒤 경고 %+v", r2.Warnings)
			}
			if !strings.Contains(r2.HTML, c.want) {
				t.Errorf("고친 뒤 after 가 부모 밖으로 나감:\n%s", r2.HTML)
			}
		})
	}
}

// 안전한 접두를 정할 수 없으면 경고만 내고 Fix 를 주지 않는다 — 문구에 「고치기」가 없다.
func TestIndentedCloseNoFix(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		// 여는 줄의 앞이 목록 기호(글자) — 닫는 줄을 어디까지 들여써야 하는지 줄 머리 공백만으로 정할 수 없다
		{"목록 기호 줄에서 연 컨테이너", "- :::box\n  text\n      :::\n\n  after\n"},
		// 후보 줄이 여는 줄의 앞 공백 그대로로 시작하지 않음(여는 줄은 공백 4, 후보는 탭)
		{"접두가 다른 공백", "- item\n\n    :::box\n    text\n\t\t:::\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnIndentedClose || r.Warnings[0].Fix != nil {
				t.Fatalf("경고 %+v", r.Warnings)
			}
			if m := r.Warnings[0].Message; strings.Contains(m, "「고치기」") || !strings.Contains(m, "이 줄로 닫으려던 것이라면 들여쓰기를 여는 ::: 와 같은 칸으로 줄이고") {
				t.Errorf("문구 %q", m)
			}
		})
	}
}
