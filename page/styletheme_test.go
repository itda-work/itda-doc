package page

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// 중립 스타일 테마(slate itda-doc#33 · swiss itda-doc#36)의 글자·면 조합 대비. 작은 글자에 쓰는 색은 그 바탕 4.5:1,
// 차트 계열은 흰 바탕 3:1 이다. 조합은 테마마다 실제로 쓰는 것만 적는다(theme.css 와 base.css 의 어휘 규칙).
// 바닥글·글자가 어느 면 위에 놓이는지(예: slate 구역 장의 남색 면이 바닥글 위에서 끝나는지, swiss 맨 구역 장의 빨강 면 위 바닥글)는
// 배치 문제라 이 시험이 묶지 못한다 — 저장소 시험에는 레이아웃을 재는 브라우저가 없고, CSS 문자열 검사로는 값이 틀려도 통과한다.
// 그 배치는 비율·패널별 렌더 검수(PNG)로 확인한다. 여기서는 놓일 자리의 색 조합만 본다.
func TestStyleThemeContrast(t *testing.T) {
	type pair struct{ fg, bg string }
	cases := map[string][]pair{
		"slate": {
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--highlight", "--paper"},
			{"--link", "--paper"}, {"--alert-note", "--paper"},
			{"--ink", "--lead-bg"}, {"--ink", "--accent-soft"}, {"--accent", "--accent-soft"},
			{"--accent-ink", "--accent"}, {"--accent-ink", "--highlight"},
		},
		"swiss": {
			{"--ink", "--paper"}, {"--muted", "--paper"}, {"--accent", "--paper"}, {"--accent-strong", "--paper"},
			{"--link", "--paper"},
			{"--ink", "--panel"}, {"--accent-strong", "--panel"}, {"--accent-strong", "--accent-soft"}, {"--ink", "--accent-soft"},
			{"--accent-ink", "--accent"}, // 빨강 면(표지·제목 장·맨 구역 장·actions 머리) 위 흰 글자
		},
	}
	ts := builtin(t)
	for name, pairs := range cases {
		th, ok := ts.Get(name)
		if !ok {
			t.Fatalf("%s 가 없음", name)
		}
		vals := map[string]string{"--paper": "#ffffff", "--accent-ink": "#ffffff"} // base.css 기본값(테마가 주지 않으면)
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
		for _, p := range pairs {
			check(p.fg, p.bg, 4.5)
		}
		for i := 1; i <= 8; i++ {
			check("--chart-"+strconv.Itoa(i), "--paper", 3)
		}
	}
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
