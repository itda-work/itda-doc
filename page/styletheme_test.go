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

// 중립 스타일 테마(slate itda-doc#33 · swiss itda-doc#36 · contrast itda-doc#37)의 글자·면 조합 대비. 작은 글자에 쓰는 색은
// 그 바탕 4.5:1(contrast 는 7:1 — 무드의 목표. 보장은 아래 쌍에 적은 조합까지), 차트 계열은 흰 바탕 3:1 이다. 조합은 테마마다 실제로 쓰는 것만 적는다(theme.css 와 base.css 의 어휘 규칙).
// contrast 의 차트 6~8번(Okabe-Ito 주황·하늘·노랑)은 3:1 에 못 미쳐 이 기준에서 빼고, 대신 그 면을 두르는 테두리 색(--ink)이 3:1 인지 본다 —
// 테두리가 실제로 그려지는지는 배치와 같이 PNG 로 확인한다.
// 쌍의 바탕에는 토큰 말고 base.css 의 고정 색도 쓴다(@base-mark — `.page-body mark` 의 노랑 바탕, base.css 에서 읽는다).
// contrast 의 할 일 표 머리(남색 면) 안 인라인 요소는 theme.css 가 머리 글자색·투명 바탕으로 맞춘다 — base 의 머리 안 code 는 글자색 16% 를
// 까는데(흰색 16% + 남색 = 6.57:1) contrast 는 그 바탕을 뺀다. 그래서 code·del·kbd 의 쌍은 --accent-ink / --accent 하나이고,
// mark 안은 먹색 / 노랑이다. 그 규칙이 실제로 그 요소에 걸리는지는 이 시험이 묶지 못하고 브라우저 계산값·PNG 로 확인한다.
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
			{"--accent-ink", "--accent"}, // 남색 면(actions 머리·steps 번호 원) 위 흰 글자·링크(--link 재정의), 머리 안 code·del·kbd
			{"--ink", "@base-mark"},      // 머리 안 mark: 노랑 바탕 위 먹 글자·링크(theme.css 가 머리 안 mark 의 color·--link 를 --ink 로)
			{"--ink", "--code-bg"}, {"--code-kw", "--code-bg"}, {"--code-str", "--code-bg"}, {"--code-com", "--code-bg"},
			{"--code-num", "--code-bg"}, {"--code-fn", "--code-bg"}, {"--code-type", "--code-bg"}, {"--code-op", "--code-bg"},
			{"--code-title-fg", "--paper"},
		}, chartEdged: map[int]bool{6: true, 7: true, 8: true}},
	}
	ts := builtin(t)
	for name, c := range cases {
		th, ok := ts.Get(name)
		if !ok {
			t.Fatalf("%s 가 없음", name)
		}
		vals := map[string]string{"--paper": "#ffffff", "--accent-ink": "#ffffff", "@base-mark": baseMarkBg(t)} // base.css 기본값(테마가 주지 않으면)
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
