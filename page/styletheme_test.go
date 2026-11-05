package page

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 중립 스타일 테마(slate itda-doc#33 · swiss itda-doc#36 · contrast itda-doc#37 · verdant itda-doc#35 · editorial itda-doc#34)의 글자·면 조합 대비. 작은 글자에 쓰는 색은
// 그 바탕 4.5:1(contrast 는 7:1 — 무드의 목표. 보장은 아래 쌍에 적은 조합까지), 차트 계열은 흰 바탕 3:1 이다. 조합은 테마마다 실제로 쓰는 것만 적는다(theme.css 와 base.css 의 어휘 규칙).
// contrast 의 차트 6~8번(Okabe-Ito 주황·하늘·노랑)은 3:1 에 못 미쳐 이 기준에서 빼고, 대신 그 면을 두르는 테두리 색(--ink)이 3:1 인지 본다 —
// 테두리가 실제로 그려지는지는 배치와 같이 PNG 로 확인한다.
// 쌍의 바탕에는 토큰 말고 base.css 의 고정 색도 쓴다(@base-mark — `.page-body mark` 의 노랑 바탕, base.css 에서 읽는다).
// 할 일 표 머리(강조색 면) 안 인라인 요소는 base 가 머리 글자색·투명 바탕으로 맞춘다(web/base.css 의 .actions th). 그래서 code·del·kbd 의 쌍은
// --accent-ink / --accent 하나이고, mark 안은 먹색 / 노랑이다. 그 규칙이 실제로 그 요소에 걸리는지는 이 시험이 묶지 못하고 브라우저 계산값·PNG 로 확인한다.
// 바닥글·글자가 어느 면 위에 놓이는지(예: slate 구역 장의 남색 면이 바닥글 위에서 끝나는지, swiss 맨 구역 장의 빨강 면 위 바닥글)는
// 배치 문제라 이 시험이 묶지 못한다 — 저장소 시험에는 레이아웃을 재는 브라우저가 없고, CSS 문자열 검사로는 값이 틀려도 통과한다.
// 그 배치는 비율·패널별 렌더 검수(PNG)로 확인한다. 여기서는 놓일 자리의 색 조합만 본다.
func TestStyleThemeContrast(t *testing.T) {
	type pair struct{ fg, bg string }
	type spec struct {
		min        float64      // 글자 쌍의 최소 대비
		pairs      []pair       // 글자 / 바탕
		chartEdged map[int]bool // 흰 바탕 3:1 미달이라 먹색 테두리로 모양을 세우는 차트 계열
	}
	cases := map[string]spec{
		"slate": {min: 4.5, pairs: []pair{
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--highlight", "--paper"},
			{"--link", "--paper"}, {"--alert-note", "--paper"},
			{"--ink", "--lead-bg"}, {"--ink", "--accent-soft"}, {"--accent", "--accent-soft"},
			{"--accent-ink", "--accent"}, {"--accent-ink", "--highlight"},
		}},
		"swiss": {min: 4.5, pairs: []pair{
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--accent-strong", "--paper"},
			{"--link", "--paper"},
			{"--ink", "--panel"}, {"--accent-strong", "--panel"}, {"--accent-strong", "--accent-soft"}, {"--ink", "--accent-soft"},
			{"--accent-ink", "--accent"}, // 빨강 면(표지·제목 장·맨 구역 장·actions 머리) 위 흰 글자
		}},
		"contrast": {min: 7, pairs: []pair{
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--link", "--paper"},
			{"--header-color", "--paper"}, {"--caption-color", "--paper"}, {"--quote-ink", "--paper"},
			{"--alert-note", "--paper"}, {"--alert-tip", "--paper"}, {"--alert-important", "--paper"},
			{"--alert-warning", "--paper"}, {"--alert-caution", "--paper"}, // 알림 바탕은 투명(--alert-bg-alpha 0%)
			{"--ink", "--table-head-bg"}, {"--ink", "--accent-soft"}, {"--accent", "--accent-soft"}, // 표 머리, decisions 면과 ✔
			{"--accent-ink", "--accent"}, // 남색 면(actions 머리·steps 번호 원) 위 흰 글자·링크, 머리 안 code·del·kbd(base)
			{"--ink", "@base-mark"},      // 머리 안 mark: 노랑 바탕 위 먹 글자·링크(base 의 --mark-ink)
			{"--ink", "--code-bg"}, {"--code-kw", "--code-bg"}, {"--code-str", "--code-bg"}, {"--code-com", "--code-bg"},
			{"--code-num", "--code-bg"}, {"--code-fn", "--code-bg"}, {"--code-type", "--code-bg"}, {"--code-op", "--code-bg"},
			{"--code-title-fg", "--paper"},
		}, chartEdged: map[int]bool{6: true, 7: true, 8: true}},
		// verdant(spine 바탕): 숲색 --accent 면은 표 머리(결재란 머리는 투명이라 빼고)와 맨 구역 장이고, 잎색 --highlight 는 꼬리표 면(「제 n 장」·슬라이드 제목 번호·맨 구역 장 번호)과
		// 절 번호·목차 번호 글자에 함께 쓴다. 차트 칸 안 값은 base 의 흰 글자(--chart-on)라 계열 색과의 직접 대비를 본다(테두리 보정 전).
		// 숲색 면(일반 표 머리·맨 구역 장 제목) 안 code 의 상자(흰색 16% + 숲색 = 5.65:1)와 mark·kbd 안 code 상자는 합성 색이라 여기 넣지 않았다 — 계산과 PNG 로 확인한다.
		// 그 면 안 규칙(base·spine·verdant 의 CSS)이 실제로 그 요소에 걸리는지는 이 시험이 묶지 못한다 — 브라우저 계산값·PNG 로 확인한다.
		"verdant": {min: 4.5, pairs: []pair{
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--highlight", "--paper"}, // 절·목차 번호 글자, --accent 는 숲색 면 안 kbd(흰 바탕)의 링크도
			{"--link", "--paper"}, {"--link", "--link-hover-bg"},
			{"--alert-note", "--paper"}, {"--alert-tip", "--paper"}, {"--alert-important", "--paper"},
			{"--alert-warning", "--paper"}, {"--alert-caution", "--paper"}, // 알림 면(9% 섞음) 위 값은 보고서에 따로
			{"--ink", "--accent-soft"}, {"--accent", "--accent-soft"}, // decisions 면과 ✔, 표지 정보 표 머리
			{"--ink", "--highlight-soft"}, {"--accent", "--highlight-soft"}, // 요약문·result 면과 「결과」
			{"--ink", "--table-stripe"}, {"--ink", "--code-bg"}, {"--ink", "--box-bg"},
			{"--accent-ink", "--accent"},    // 숲색 면(표 머리·steps 번호 원·맨 구역 장 제목과 그 바닥글) 위 흰 글자·링크·각주, 면 안 del
			{"--accent-ink", "--highlight"}, // 잎색 꼬리표 위 흰 글자(theme.css)
			{"--ink", "@base-mark"},         // 숲색 면 안 mark: 노랑 바탕 위 먹 글자·링크(base·spine·verdant)
			{"--chart-on", "--chart-1"}, {"--chart-on", "--chart-2"}, {"--chart-on", "--chart-3"}, {"--chart-on", "--chart-4"},
			{"--chart-on", "--chart-5"}, {"--chart-on", "--chart-6"}, {"--chart-on", "--chart-7"}, {"--chart-on", "--chart-8"},
		}},
		// editorial(classic 바탕): 바탕은 크림 --paper(#faf7f2 — 토큰이라 아래 vals 가 흰색 기본값을 덮는다). 빨강 --accent 면은 할 일 표 머리
		// 하나이고(다른 표 머리·결재란 머리는 투명), 구역 장 왼쪽 면은 옅은 --accent-soft 라 그 위 바닥글(--muted·--ink)을 본다.
		"editorial": {min: 4.5, pairs: []pair{
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--link", "--paper"}, {"--link", "--link-hover-bg"},
			{"--header-color", "--paper"}, {"--caption-color", "--paper"}, {"--quote-ink", "--paper"},
			{"--alert-note", "--paper"}, {"--alert-tip", "--paper"}, {"--alert-important", "--paper"},
			{"--alert-warning", "--paper"}, {"--alert-caution", "--paper"}, // 알림 면(6% 섞음) 위 값은 보고서에 따로
			{"--ink", "--accent-soft"}, {"--accent", "--accent-soft"}, {"--muted", "--accent-soft"}, // decisions 면과 ✔, 구역 장 면 위 바닥글
			{"--ink", "--table-stripe"}, {"--ink", "--code-bg"},
			{"--accent-ink", "--accent"}, // 빨강 면(할 일 표 머리·steps 번호 원) 위 흰 글자·링크·각주, 면 안 code·del·kbd(base)
			{"--ink", "@base-mark"},      // 빨강 면 안 mark: 노랑 바탕 위 먹 글자·링크(base)
			{"--chart-on", "--chart-1"}, {"--chart-on", "--chart-2"}, {"--chart-on", "--chart-3"}, {"--chart-on", "--chart-4"},
			{"--chart-on", "--chart-5"}, {"--chart-on", "--chart-6"}, {"--chart-on", "--chart-7"}, {"--chart-on", "--chart-8"},
		}},
	}
	ts := builtin(t)
	for name, c := range cases {
		th, ok := ts.Get(name)
		if !ok {
			t.Fatalf("%s 가 없음", name)
		}
		vals := map[string]string{"--paper": "#ffffff", "--accent-ink": "#ffffff", "--chart-on": "#ffffff", "@base-mark": baseMarkBg(t)} // base.css 기본값(테마가 주지 않으면)
		for _, kv := range th.Tokens {
			vals[kv[0]] = kv[1]
		}
		color := func(n string) string {
			v := vals[n]
			for i := 0; i < 4 && strings.HasPrefix(v, "var("); i++ {
				v = vals[strings.TrimSuffix(strings.TrimPrefix(v, "var("), ")")]
			}
			if len(v) != 7 || v[0] != '#' {
				t.Fatalf("%s: %s 색 값을 풀지 못함 %q", name, n, vals[n])
			}
			return v
		}
		check := func(fg, bg string, min float64) {
			if r := contrastRatio(color(fg), color(bg)); r < min {
				t.Errorf("%s: %s on %s = %.2f:1 — %.1f:1 이상이어야 함", name, fg, bg, r, min)
			}
		}
		for _, p := range c.pairs {
			check(p.fg, p.bg, c.min)
		}
		for i := 1; i <= 8; i++ {
			if c.chartEdged[i] {
				check("--ink", "--chart-"+strconv.Itoa(i), 3) // 면을 두르는 먹색 테두리와 면의 경계
				continue
			}
			check("--chart-"+strconv.Itoa(i), "--paper", 3)
		}
	}
}

// baseMarkBg 는 base.css 의 `.page-body mark` 바탕색이다.
func baseMarkBg(t *testing.T) string {
	b, err := os.ReadFile(filepath.Join("..", "web", "base.css"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\.page-body mark \{ background: (#[0-9a-fA-F]{6});`).FindSubmatch(b)
	if m == nil {
		t.Fatal("base.css 에서 .page-body mark 바탕색을 찾지 못함")
	}
	return strings.ToLower(string(m[1]))
}

// contrastRatio 는 WCAG 2.x 대비비다(#rrggbb 둘).
func contrastRatio(a, b string) float64 {
	lum := func(h string) float64 {
		var c [3]float64
		for i := range c {
			n, _ := strconv.ParseUint(h[1+2*i:3+2*i], 16, 8)
			v := float64(n) / 255
			if v <= 0.04045 {
				c[i] = v / 12.92
			} else {
				c[i] = math.Pow((v+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
	}
	x, y := lum(a), lum(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// bold 의 머리글·바닥글(--muted)과 쪽 번호(--accent)는 흰 바탕 위, 맨 구역 장 바닥글·쪽 번호는 강조색 면 위 --accent-ink 다(#40).
// 바닥글이 어느 면 위에 놓이는지는 렌더로 확인했고 여기서는 CSS 규칙과 색 조합만 본다. 실제 계산값은 보지 않는다(미검증).
func TestBoldFootContrast(t *testing.T) {
	th, ok := builtin(t).Get("bold")
	if !ok {
		t.Fatal("bold 없음")
	}
	vals := map[string]string{"--accent-ink": "#ffffff", "--paper": "#ffffff"} // base.css 기본값
	for _, kv := range th.Tokens {
		vals[kv[0]] = kv[1]
	}
	for _, p := range [][2]string{{"--muted", "--paper"}, {"--accent", "--paper"}, {"--accent-ink", "--accent"}} {
		if r := contrastRatio(strings.ToLower(vals[p[0]]), strings.ToLower(vals[p[1]])); r < 4.5 {
			t.Errorf("%s on %s = %.2f:1 — 4.5:1 이상이어야 함", p[0], p[1], r)
		}
	}
	css := th.CSS
	for _, sel := range []string{".page-header", ".page-footer", ".slide-foot"} {
		if !strings.Contains(css, sel+" { color: var(--muted); }") {
			t.Errorf("%s 글자색이 --muted 가 아님", sel)
		}
	}
	if !strings.Contains(css, ".slide-deco.k-section.bare ~ .slide-foot, .slide-deco.k-section.bare ~ .slide-foot .sf-num { color: var(--accent-ink); }") {
		t.Error("맨 구역 장 바닥글·쪽 번호가 면 위 글자색이 아님")
	}
}
