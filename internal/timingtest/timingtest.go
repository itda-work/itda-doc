// Package timingtest 는 시간 기준 시험(이차 시간·느려짐 회귀)을 다른 작업의 부하에 덜 흔들리게 재는 도우미다. 시험에서만 쓴다(#201·#130).
//
// 두 가지로 부하를 견딘다:
//   - 척도: 벽시계 대신 이 프로세스의 CPU 시간(사용자 + 시스템)을 잰다. 패키지 병렬 go test 처럼 다른 프로세스와 CPU 를 나눠 써도
//     기다린 시간은 들어가지 않는다. 이차 시간이면 CPU 시간도 네 배가 되니 잡는 힘은 같다. CPU 시간을 잘게 재지 못하는 곳
//     (Windows — 15.6ms 단위)은 벽시계다.
//   - 다시 재기: 판정이 실패로 보이면 몇 번 더 재어 가장 작은 값으로 다시 판정한다. 부하로 부푼 값은 다시 재면
//     작아지지만, 진짜 이차면 몇 번을 재도 네 배다.
//
// 이차 판정(AssertDoubling — 규칙은 judge)은 크기 수에 따라 다르다:
//   - 크기 셋 이하: 쌍마다 — 크기가 두 배가 될 때 비용이 앞 값의 3배 + Slack 을 넘지 않는다.
//   - 크기 넷 이상: (i) 수열 전체 — 두 배를 k 번 하면 마지막 비용이 첫 비용의 3^k 배 + k×Slack 을 넘지 않고, (ii) 연속한 두 쌍이
//     모두 3배 + Slack 을 넘지 않는다. 작업 집합이 캐시 한 단을 넘는 한 구간에서만 5배 안팎으로 튀는 계단(GitHub 러너의 x86, 앞뒤
//     구간은 2배 안팎)은 한 쌍에서만 튀므로 통과하고, 이차(4^k)·평평하다 이차로 꺾이는 수열은 (i) 이나 (ii) 에 걸린다.
//
// 알려진 틈: 4점에서 마지막 쌍만 튀는 C+q 꼴(예 14·26·74·266)은 계단과 가르지 못한다 — main 의 3점 판정도 같은 감도였다(리뷰 1회차 [1]).
package timingtest

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// Retries 는 실패로 보이는 판정을 다시 재는 횟수다.
const Retries = 4

// Cost 는 f 한 번의 비용이다(CPU 시간, 없으면 벽시계). 재기 전에 GC 를 돌려 앞 일의 쓰레기 치우기가 섞이지 않게 한다.
func Cost(f func()) time.Duration {
	runtime.GC()
	if c0, ok := cpuTime(); ok {
		f()
		c1, _ := cpuTime()
		return c1 - c0
	}
	t0 := time.Now()
	f()
	return time.Since(t0)
}

// Best 는 f 를 n 번 재어 가장 작은 비용이다.
func Best(n int, f func()) time.Duration {
	best := time.Duration(1<<63 - 1)
	for range max(n, 1) {
		best = min(best, Cost(f))
	}
	return best
}

// Within 은 f 의 비용이 limit 안인지 본다. 넘으면 Retries 번까지 다시 재어 가장 작은 값으로 판정한다.
// f 는 적어도 한 번 돈다(결과 검사에 쓰도록).
func Within(limit time.Duration, f func()) (time.Duration, bool) {
	d := Cost(f)
	for r := 0; d > limit && r < Retries; r++ {
		d = min(d, Cost(f))
	}
	return d, d <= limit
}

// Doubling 은 AssertDoubling 의 판정 기준이다.
type Doubling struct {
	Reps  int           // 크기마다 처음 잴 때 잰 횟수(가장 작은 값)
	Slack time.Duration // 덧셈 여유(두 배 한 번마다): 쌍은 ≤ 3 × 앞 값 + Slack, 수열 전체는 ≤ 3^k × 첫 비용 + k × Slack(k = 두 배 횟수)
	Limit time.Duration // 각 크기 비용의 상한(0 이면 보지 않는다)
}

// AssertDoubling 은 크기가 두 배씩 커지는 sizes 마다 prepare(size) 가 돌려준 일을 재어, 비용이 이차로 자라지 않고 각 크기가
// Limit 안인지 본다(규칙은 judge — 크기 셋 이하는 쌍마다, 넷 이상은 수열 전체와 연속한 두 쌍).
// 입력 만들기는 prepare 에서 하므로 재는 데 들어가지 않는다. 잰 수열은 늘 t.Logf 로 남긴다(-v 와 실패 때 보인다).
// 실패로 보이면 모든 크기를 Retries 번까지 다시 재어 크기마다 가장 작은 값으로 다시 판정한다 — 다시 잰 일도 t.Log 로 남긴다.
func AssertDoubling(t testing.TB, name string, sizes []int, prepare func(size int) func(), o Doubling) {
	t.Helper()
	works := make([]func(), len(sizes))
	ds := make([]time.Duration, len(sizes))
	for i, size := range sizes {
		works[i] = prepare(size)
		ds[i] = Best(o.Reps, works[i])
	}
	for r := 0; r < Retries && len(judge(ds, sizes, o)) > 0; r++ {
		before := append([]time.Duration(nil), ds...)
		for i := range ds { // 모든 크기를 같은 때에 잰다(작은 쪽부터)
			ds[i] = min(ds[i], Cost(works[i]))
		}
		t.Logf("%s: 다시 잼(%d) %v → %v", name, r+1, before, ds)
	}
	t.Logf("%s: %v (크기 %v)", name, ds, sizes)
	for _, why := range judge(ds, sizes, o) {
		t.Errorf("%s: %s", name, why)
	}
}

// judge 는 잰 수열 ds(크기 sizes 가 두 배씩, ds 와 같은 길이)의 실패 사유들이다 — 비면 통과. 사유에는 차례 번호 대신 실제 크기를 적는다.
//   - 크기 셋 이하: 쌍마다 ds[i] ≤ 3 × ds[i-1] + Slack.
//   - 크기 넷 이상: (i) ds[n-1] ≤ 3^(n-1) × ds[0] + (n-1) × Slack, (ii) 3 × 앞 값 + Slack 을 넘는 쌍이 연달아 둘 있지 않다.
//   - 모든 n: Limit > 0 이면 각 크기 ≤ Limit.
func judge(ds []time.Duration, sizes []int, o Doubling) []string {
	var out []string
	n := len(ds)
	pairBad := func(i int) bool { return ds[i] > 3*ds[i-1]+o.Slack }
	switch {
	case n >= 2 && n <= 3:
		for i := 1; i < n; i++ {
			if pairBad(i) {
				out = append(out, fmt.Sprintf("크기 %d → %d 에 비용 %v → %v, 상한 %v(3배 + %v) (이차?)", sizes[i-1], sizes[i], ds[i-1], ds[i], 3*ds[i-1]+o.Slack, o.Slack))
			}
		}
	case n >= 4:
		b := ds[0]
		for range n - 1 {
			b *= 3
		}
		b += time.Duration(n-1) * o.Slack
		if ds[n-1] > b {
			out = append(out, fmt.Sprintf("수열 전체(크기 %d → %d)에 비용 %v → %v, 상한 %v(3^%d 배 + %d×%v) (이차?)", sizes[0], sizes[n-1], ds[0], ds[n-1], b, n-1, n-1, o.Slack))
		}
		for i := 2; i < n; i++ {
			if pairBad(i-1) && pairBad(i) {
				out = append(out, fmt.Sprintf("연속한 두 쌍(크기 %d→%d→%d)이 모두 3배 + %v 를 넘음: %v → %v → %v (이차?)", sizes[i-2], sizes[i-1], sizes[i], o.Slack, ds[i-2], ds[i-1], ds[i]))
			}
		}
	}
	if o.Limit > 0 {
		for i, d := range ds {
			if d > o.Limit {
				out = append(out, fmt.Sprintf("크기 %d 에 %v (상한 %v)", sizes[i], d, o.Limit))
			}
		}
	}
	return out
}
