package timingtest

import (
	"strings"
	"testing"
	"time"
)

// judge 의 규칙(크기 셋 이하는 쌍마다, 넷 이상은 수열 전체 + 연속한 두 쌍, Limit 은 늘)을 수열로 고정한다(리뷰 1회차 [1]).
func TestJudge(t *testing.T) {
	ms := func(v ...float64) []time.Duration {
		out := make([]time.Duration, len(v))
		for i, x := range v {
			out[i] = time.Duration(x * float64(time.Millisecond))
		}
		return out
	}
	s5 := Doubling{Slack: 5 * time.Millisecond}
	for _, c := range []struct {
		name string
		ds   []time.Duration
		o    Doubling
		want string // "" 이면 통과, 아니면 사유에 든 글자
	}{
		{"4점 계단(한 쌍만 튐)", ms(6, 12, 60, 120), s5, ""},
		{"순수 이차", ms(4, 16, 64, 256), s5, "이차"},
		{"평평→이차", ms(10, 10, 40, 160), s5, "연속한 두 쌍"},
		{"선형", ms(10, 20, 40, 80), s5, ""},
		{"러너 실측 1", ms(4.814, 11.567, 25.675, 42.951), s5, ""},
		{"러너 실측 2", ms(1.439, 3.846, 17.661, 21.499), s5, ""},
		{"3점 평평→이차(옛 쌍 판정)", ms(20, 20, 80), Doubling{Slack: 10 * time.Millisecond}, "이차"},
		{"3점 선형", ms(10, 20, 40), s5, ""},
		{"한 점", ms(500), s5, ""},
		{"상한 넘음", ms(10, 20, 40, 80), Doubling{Slack: 5 * time.Millisecond, Limit: 50 * time.Millisecond}, "상한"},
	} {
		got := judge(c.ds, c.o)
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%s: 통과해야 함 — %v", c.name, got)
			}
			continue
		}
		if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), c.want) {
			t.Errorf("%s: %q 가 든 실패여야 함 — %v", c.name, c.want, got)
		}
	}
}
