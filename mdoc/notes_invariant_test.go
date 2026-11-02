package mdoc

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// notesLeak 은 DropNotes 로 렌더한 HTML 을 파서로 읽어 노트 자리(class 토큰에 notes 가 든 요소)가 비어 있지 않은 것 — 자식 요소·
// 공백 아닌 글자, class="notes"·data-line 밖의 속성(사용자 class·id·data-*) — 과 글자·속성값에 든 marker 를 찾는다(#344 — 공유
// 「노트 함께 보이기」 꺼짐의 불변식: 노트는 빈 자리로만 남고 노트 글이 없다). marker 가 비면 글자는 보지 않는다.
func notesLeak(t *testing.T, doc, marker string) []string {
	t.Helper()
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("HTML 을 읽지 못함: %v", err)
	}
	var leaks []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			if isNotesEl(n) {
				for _, a := range n.Attr {
					if !(a.Key == "class" && a.Val == "notes") && a.Key != "data-line" {
						leaks = append(leaks, "노트 자리 속성 "+a.Key+"="+a.Val)
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type != html.TextNode || strings.TrimSpace(c.Data) != "" {
						leaks = append(leaks, "노트 자리 안 "+c.Data)
					}
				}
			}
			for _, a := range n.Attr {
				if marker != "" && strings.Contains(a.Val, marker) {
					leaks = append(leaks, "속성 "+a.Key+"="+a.Val)
				}
			}
		case html.TextNode:
			if marker != "" && strings.Contains(n.Data, marker) {
				leaks = append(leaks, "글자 "+n.Data)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return leaks
}

// noteAttrs 는 HTML 의 노트 요소(class 토큰 notes, 중첩 포함)에 붙은 class="notes"·data-line 밖의 속성이다 — 렌더러는 DropNotes 와
// 상관없이 노트를 늘 class="notes" 하나로 낸다(transform.go notes, #344 2단계 리뷰 5회차). 노트 글은 보지 않는다(호스트 렌더에도 쓴다).
func noteAttrs(t *testing.T, doc string) []string {
	t.Helper()
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("HTML 을 읽지 못함: %v", err)
	}
	var bad []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && isNotesEl(n) {
			for _, a := range n.Attr {
				if !(a.Key == "class" && a.Val == "notes") && a.Key != "data-line" {
					bad = append(bad, a.Key+"="+a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return bad
}

func isNotesEl(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && slices.Contains(strings.Fields(a.Val), "notes") {
			return true
		}
	}
	return false
}

// topBlocks 는 HTML 의 맨 위 요소를 「태그.class…」 로 적은 것이다 — 노트 자리도 따로 정규화하지 않는다(렌더러가 노트를 늘
// class="notes" 하나로 내므로 — transform.go notes, #344 2단계 리뷰 5회차: {.notes .page-break}). 호스트와 DropNotes 결과의 구조
// 비교에 쓴다(글자·id 는 보지 않는다 — 노트 안 제목이 빠지면 뒤 제목의 id 가 바뀔 수 있다).
func topBlocks(t *testing.T, doc string) []string {
	t.Helper()
	nodes, err := html.ParseFragment(strings.NewReader(doc), &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body})
	if err != nil {
		t.Fatalf("HTML 을 읽지 못함: %v", err)
	}
	var out []string
	for _, n := range nodes {
		if n.Type != html.ElementNode {
			continue
		}
		k := n.Data
		for _, a := range n.Attr {
			if a.Key == "class" {
				k += "." + strings.Join(strings.Fields(a.Val), ".")
			}
		}
		out = append(out, k)
	}
	return out
}

// sameStructure 는 같은 원문의 호스트 렌더(DropNotes 거짓)와 공유 렌더(참)의 자동 슬라이드 단계·맨 위 블록 구조가 같은지다(#344 2단계
// 리뷰 3·4회차 — 노트를 빈 자리로 남기므로 구성상 같다). 블록 중첩 상한을 넘은 문서는 서로 다른 대체 렌더라 견주지 않는다.
func sameStructure(t *testing.T, src string) (host, viewer Result, ok bool) {
	t.Helper()
	host, err := Render([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	viewer, err = Render([]byte(src), Options{DropNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(host.HTML, `<pre class="raw-source"`) {
		return host, viewer, true
	}
	ok = true
	if host.SlideLevel != viewer.SlideLevel {
		t.Errorf("노트를 빼자 슬라이드 단계가 바뀜: %d → %d\n입력 %q\n%s\n%s", host.SlideLevel, viewer.SlideLevel, src, host.HTML, viewer.HTML)
		ok = false
	}
	if bad := noteAttrs(t, host.HTML); len(bad) > 0 {
		t.Errorf("호스트 노트에 다른 속성 %q\n입력 %q\n%s", bad, src, host.HTML)
		ok = false
	}
	if h, v := topBlocks(t, host.HTML), topBlocks(t, viewer.HTML); !slices.Equal(h, v) {
		t.Errorf("노트를 빼자 맨 위 블록이 바뀜\n호스트 %q\n공유 %q\n입력 %q", h, v, src)
		ok = false
	}
	return host, viewer, ok
}

// TestDropNotesInvariant 는 노트를 숨기는 꼴이 컨테이너 하나뿐이라는 보장(#344)을 여러 꼴로 고정한다. 노트 글에는 「비밀」 을 적고,
// 제목의 {.notes} 는 노트가 아니므로 그 제목 글자는 남아야 한다(keep). 노트는 빈 자리로만 남고, 호스트와 구조가 같다(sameStructure).
func TestDropNotesInvariant(t *testing.T) {
	cases := []struct {
		name, src string
		keep      []string // 남아야 하는 글자
	}{
		{"컨테이너", "공개\n\n:::notes\n비밀\n:::\n", []string{"공개"}},
		{"속성꼴 컨테이너", "공개\n\n::: {.notes}\n비밀\n:::\n", []string{"공개"}},
		{"여러 class 컨테이너", "공개\n\n::: {.x .notes #n1 k=v}\n비밀\n:::\n", []string{"공개"}},
		{"속성값에 비밀", "공개\n\n::: {.notes #비밀 k=비밀값}\n글\n:::\n", []string{"공개"}},
		{"차트 이름 노트 class", "공개\n\n::: {.chart .notes}\n| 비밀 | 1 |\n|---|---|\n| 가 | 2 |\n:::\n", []string{"공개"}},
		{"속성값 raw HTML 컨테이너", "공개\n\n::: {.notes x=\"<i>비밀</i>\"}\n비밀\n:::\n", []string{"공개"}},
		{"중첩", ":::notes\n바깥 비밀\n\n:::notes\n안 비밀\n:::\n:::\n\n공개\n", []string{"공개"}},
		{"상자 안", ":::box\n상자 공개\n\n:::notes\n상자 비밀\n\n## 노트 제목 비밀\n:::\n:::\n", []string{"상자 공개"}},
		{"노트 안 그림·링크·각주", "공개[^p]\n\n:::notes\n![비밀](비밀.png) [비밀](http://e.com/비밀) 글[^n]\n:::\n\n[^p]: 공개 각주\n[^n]: 비밀 각주\n",
			[]string{"공개", "공개 각주"}},
		{"제목 .notes", "# 공개\n\n## 제목가 {.notes}\n", []string{"제목가"}},
		{"제목 .x .notes", "## 제목나 {.x .notes}\n", []string{"제목나"}},
		{"제목 속성값 <b>", "# 공개\n\n## 제목다 {.notes x=\"<b>\"}\n", []string{"제목다"}},
		{"제목 속성값 <i>", "## 제목라 {.notes x=\"<i>숨김</i>\"}\n", []string{"제목라"}},
		{"제목 .notes #id", "## 제목마 {.notes #m}\n", []string{"제목마"}},
		{"raw HTML class", "공개 <span class=\"notes\">x</span>\n\n<div class=\"notes\">\n\nx\n\n</div>\n", []string{"공개"}},
		{"슬라이드", "---\nslides: true\n---\n\n# 쪽\n\n글\n\n:::notes\n비밀\n:::\n\n# 둘째 {.notes}\n\n. . .\n\n::: {.incremental .notes}\n비밀\n:::\n", []string{"쪽", "둘째"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := Render([]byte(c.src), Options{DropNotes: true})
			if err != nil {
				t.Fatal(err)
			}
			if leaks := notesLeak(t, r.HTML, "비밀"); len(leaks) > 0 {
				t.Errorf("노트 누출 %q\n%s", leaks, r.HTML)
			}
			sameStructure(t, c.src)
			for _, k := range c.keep {
				if !strings.Contains(r.HTML, k) {
					t.Errorf("%q 가 빠짐\n%s", k, r.HTML)
				}
			}
			var walk func([]TOCItem)
			walk = func(items []TOCItem) {
				for _, it := range items {
					if strings.Contains(it.Text, "비밀") || strings.Contains(it.ID, "비밀") {
						t.Errorf("목차에 노트: %+v", it)
					}
					walk(it.Children)
				}
			}
			walk(r.TOC)
			// DropNotes 가 거짓이어도 제목에는 notes class 가 없다 — .notes 요소는 컨테이너뿐이다
			keep, err := Render([]byte(c.src), Options{})
			if err != nil {
				t.Fatal(err)
			}
			root, _ := html.Parse(strings.NewReader(keep.HTML))
			var heads func(*html.Node)
			heads = func(n *html.Node) {
				if n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
					for _, a := range n.Attr {
						if a.Key == "class" && slices.Contains(strings.Fields(a.Val), "notes") {
							t.Errorf("DropNotes 없이 제목에 notes class: %s", keep.HTML)
						}
					}
				}
				for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
					heads(ch)
				}
			}
			heads(root)
		})
	}
}

// FuzzDropNotesInvariant 는 정상 노트(식별 글자 notesMarker)를 앞에 두고 뒤 문서를 아무렇게나 바꿔도 DropNotes 결과의 노트 자리가
// 비어 있고(자식·글자·사용자 속성 없음) 노트 글자가 — HTML·목차·경고 어디에도 — 없으며(#344 리뷰 3회차: 뒤 문서 때문에 원문 보기로
// 바뀌는 경로까지), 호스트와 슬라이드 단계·맨 위 블록 구조가 같은지다(리뷰 4회차 — sameStructure). 노트는 앞에서 닫히므로 뒤 문서가
// 그 글을 노트 밖으로 꺼낼 수 없다. go test 기본 실행은 시드만 돈다.
func FuzzDropNotesInvariant(f *testing.F) {
	for _, s := range []string{
		"",
		"공개\n",
		"## 비밀 {.notes x=\"<b>\"}\n",
		"## 비밀 {.x .notes}\n",
		"::: {.notes x=\"<i>비밀</i>\"}\n글\n:::\n",
		":::box\n:::notes\n비밀\n:::\n:::\n",
		"![a](b.png){.notes}\n",
		strings.Repeat(">", 65) + " 깊은 공개 인용\n",
		strings.Repeat("- ", 70) + "깊은 목록\n",
		strings.Repeat(":::box\n", 70) + "깊은 상자\n",
		"# 제목\n\n글[^n]\n\n[^n]: 각주\n",
		"<!-- 주석 -->\n\n## 첫\n\n본문\n",
		":::notes\n메모[^n]\n:::\n\n## 둘\n\n[^n]: 노트 각주\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, rest string) {
		if len(rest) > 4096 || strings.Contains(rest, notesMarker) {
			return
		}
		src := "# 공개\n\n:::notes\n" + notesMarker + "\n:::\n\n" + rest
		_, r, _ := sameStructure(t, src)
		if leaks := notesLeak(t, r.HTML, notesMarker); len(leaks) > 0 {
			t.Errorf("노트 누출 %q\n입력 %q\n%s", leaks, src, r.HTML)
		}
		if strings.Contains(r.HTML, notesMarker) {
			t.Errorf("HTML 에 노트 글자\n입력 %q\n%s", src, r.HTML)
		}
		var walk func([]TOCItem)
		walk = func(items []TOCItem) {
			for _, it := range items {
				if strings.Contains(it.Text, notesMarker) || strings.Contains(it.ID, notesMarker) {
					t.Errorf("목차에 노트: %+v\n입력 %q", it, src)
				}
				walk(it.Children)
			}
		}
		walk(r.TOC)
		for _, w := range r.Warnings {
			if strings.Contains(w.Message, notesMarker) {
				t.Errorf("경고에 노트: %+v\n입력 %q", w, src)
			}
		}
	})
}

const notesMarker = "발표비밀표식"

// TestDropNotesWithheldRaw 는 블록 중첩 상한을 넘어 원문 보기로 바뀌는 문서다(#344 리뷰 3회차 P1). 원문 글자에서는 노트를 뺄 수
// 없으므로 DropNotes 면 원문 대신 안내만 내고(nesting.go withheldRaw), DropNotes 가 아니면(호스트 화면) 원문 보기 그대로다.
func TestDropNotesWithheldRaw(t *testing.T) {
	src := []byte("# 공개\n\n:::notes\n" + notesMarker + "\n:::\n\n" + strings.Repeat(">", MaxNestingDepth+1) + " 깊은 공개 인용\n")
	r, err := Render(src, Options{DropNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.HTML, notesMarker) || strings.Contains(r.HTML, "raw-source") || !strings.Contains(r.HTML, WithheldNotice) {
		t.Errorf("DropNotes 대체 렌더가 안내가 아님: %s", r.HTML)
	}
	if len(r.TOC) != 0 || r.SlideLevel != 0 {
		t.Errorf("안내에 목차·슬라이드 단계: %+v %d", r.TOC, r.SlideLevel)
	}
	tooComplex := false
	for _, w := range r.Warnings {
		if strings.Contains(w.Message, notesMarker) {
			t.Errorf("경고에 노트: %+v", w)
		}
		tooComplex = tooComplex || w.Code == WarnTooComplex
	}
	if !tooComplex {
		t.Errorf("too-complex 경고 없음: %+v", r.Warnings)
	}
	host, err := Render(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(host.HTML, `<pre class="raw-source"`) || !strings.Contains(host.HTML, notesMarker) {
		t.Errorf("호스트 원문 보기가 바뀜: %s", host.HTML)
	}
}

// TestDropNotesStructure 는 노트를 빈 자리로 남기므로 자동 슬라이드 단계·맨 위 블록 구조가 호스트와 같은지다(#344 2단계 리뷰 3·4회차 —
// 노트를 떼어 내면 제목 다음 블록이 바뀌어 단계가 달라졌다). 4회차 반례는 제목과 노트 사이에 변환이 지우는 HTML 블록이 있는 꼴이다.
func TestDropNotesStructure(t *testing.T) {
	cases := []struct {
		name, src string
		level     int
	}{
		{"제목 뒤 노트(3회차)", "---\nslides: true\n---\n\n# 개요\n\n:::notes\n발표 메모\n:::\n\n## 첫 항목\n\n본문 A\n\n## 둘째 항목\n\n본문 B\n", 1},
		{"제목 주석 노트(4회차)", "---\nslides: true\n---\n\n# 개요\n\n<!-- 공개 주석 -->\n\n:::notes\n발표 메모\n:::\n\n## 첫 항목\n\n본문 A\n\n## 둘째 항목\n\n본문 B\n", 1},
		{"노트 .page-break(5회차)", "# 첫 장\n\n앞 본문\n\n::: {.notes .page-break}\n비밀 메모\n:::\n\n뒤 본문\n", 1},
		{"노트 .incremental", "# 개요\n\n::: {.notes .incremental}\n- 메모\n:::\n\n## 첫\n\n본문\n", 1},
		{"노트 #id 속성", "# 개요\n\n::: {.notes #a k=v}\n메모\n:::\n\n## 첫\n\n본문\n", 1},
		{"상자 이름 .notes", "# 개요\n\n::: {.box .notes}\n메모\n:::\n\n## 첫\n\n본문\n", 1},
		{"제목 빈 div 노트(4회차)", "# 개요\n\n<div></div>\n\n:::notes\n발표 메모\n:::\n\n## 첫 항목\n\n본문 A\n\n## 둘째 항목\n\n본문 B\n", 1},
		{"제목 뒤 노트 둘", "# 개요\n\n:::notes\n가\n:::\n\n::: {.notes}\n나\n:::\n\n## 첫\n\n본문\n", 1},
		{"끝 제목 뒤 노트 각주", "# 하나\n\n## 둘\n\n본문\n\n# 끝\n\n:::notes\n메모[^n]\n:::\n\n[^n]: 노트 각주\n", 1},
		{"끝 제목 뒤 각주 목록", "## 둘\n\n## 셋\n\n:::notes\n메모[^n]\n:::\n\n# 끝\n\n[^n]: 노트 각주\n", 1},
		{"상자 안 노트", "# 개요\n\n:::box\n:::notes\n가\n:::\n:::\n\n## 첫\n\n본문\n", 1},
		{"노트 없음", "# 개요\n\n## 첫\n\n본문\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host, viewer, _ := sameStructure(t, c.src)
			if host.SlideLevel != c.level {
				t.Errorf("호스트 슬라이드 단계 %d, 기대 %d", host.SlideLevel, c.level)
			}
			if leaks := notesLeak(t, viewer.HTML, "메모"); len(leaks) > 0 {
				t.Errorf("노트 누출 %q\n%s", leaks, viewer.HTML)
			}
			for _, bad := range []string{"노트 각주", "<hr>", "<ol>"} {
				if strings.Contains(c.src, "[^n]") && strings.Contains(viewer.HTML, bad) {
					t.Errorf("노트 전용 각주 목록에 %q 가 남음\n%s", bad, viewer.HTML)
				}
			}
		})
	}
}
