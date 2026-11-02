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
		// timingtest.go 패키지 주석의 「알려진 틈」 — 4점에서 마지막 쌍만 튀는 C+q 꼴은 계단과 가르지 못해 통과한다(리뷰 2회차 [7]).
		{"4점 C+q(알려진 틈)", ms(14, 26, 74, 266), s5, ""},
		{"3점 C+q(138 > 3×42+10 = 136)", ms(18, 42, 138), Doubling{Slack: 10 * time.Millisecond}, "이차"},
		{"상한 넘음", ms(10, 20, 40, 80), Doubling{Slack: 5 * time.Millisecond, Limit: 50 * time.Millisecond}, "상한"},
	} {
		sizes := make([]int, len(c.ds)) // 크기는 문구에만 쓰인다 — 1000 부터 두 배씩
		for i := range sizes {
			sizes[i] = 1000 << i
		}
		got := judge(c.ds, sizes, c.o)
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%s: 통과해야 함 — %v", c.name, got)
			}
			continue
		}
		if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), c.want) {
			t.Errorf("%s: %q 가 든 실패여야 함 — %v", c.name, c.want, got)
		}
		// 실패 문구는 차례 번호가 아니라 실제 크기를 적는다(리뷰 2회차 [8]) — 상한을 넘은 것은 80ms 인 크기 8000 하나다.
		if c.name == "상한 넘음" && !strings.Contains(strings.Join(got, "\n"), "크기 8000 에") {
			t.Errorf("%s: 문구에 실제 크기 8000 이 없음 — %v", c.name, got)
		}
	}
}
