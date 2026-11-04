package page

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// slate(내장 중립 스타일 테마, itda-doc#33)의 글자·면 조합 대비. 작은 글자에 쓰는 색은 바탕 4.5:1, 차트 계열은 3:1,
// 강조색 면 위 글자는 4.5:1 이다. 구역 슬라이드의 바닥글(--muted)이 남색 면이 아니라 흰 바탕 위에 놓이는지(면이 바닥글 줄
// 위에서 끝나는지)는 배치 문제라 이 시험이 묶지 못한다 — 저장소 시험에는 레이아웃을 재는 브라우저가 없고, CSS 문자열 검사로는
// 값이 틀려도 통과한다. 그 배치는 비율·패널별 렌더 검수(PNG)로 확인한다. 여기서는 흰 바탕 위 --muted 가 4.5:1 인지만 본다.
func TestSlateContrast(t *testing.T) {
	th, ok := builtin(t).Get("slate")
	if !ok {
		t.Fatal("slate 가 없음")
	}
	vals := map[string]string{"--paper": "#ffffff", "--accent-ink": "#ffffff"} // base.css 기본값(slate 가 주지 않음)
	for _, kv := range th.Tokens {
		vals[kv[0]] = kv[1]
	}
	color := func(name string) string {
		v := vals[name]
		for i := 0; i < 4 && strings.HasPrefix(v, "var("); i++ {
			v = vals[strings.TrimSuffix(strings.TrimPrefix(v, "var("), ")")]
		}
		if len(v) != 7 || v[0] != '#' {
			t.Fatalf("%s: 색 값을 풀지 못함 %q", name, vals[name])
		}
		return v
	}
	check := func(fg, bg string, min float64) {
		if r := contrastRatio(color(fg), color(bg)); r < min {
			t.Errorf("%s on %s = %.2f:1 — %.1f:1 이상이어야 함", fg, bg, r, min)
		}
	}
	for _, fg := range []string{"--ink", "--muted", "--accent", "--highlight", "--link", "--alert-note"} {
		check(fg, "--paper", 4.5)
	}
	check("--ink", "--lead-bg", 4.5)
	check("--ink", "--accent-soft", 4.5)
	check("--accent", "--accent-soft", 4.5)
	check("--accent-ink", "--accent", 4.5)
	check("--accent-ink", "--highlight", 4.5)
	for i := 1; i <= 8; i++ {
		check("--chart-"+strconv.Itoa(i), "--paper", 3)
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
