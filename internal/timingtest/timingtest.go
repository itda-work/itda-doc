// Package timingtest 는 시간 기준 시험(이차 시간·느려짐 회귀)을 다른 작업의 부하에 덜 흔들리게 재는 도우미다. 시험에서만 쓴다(#201·#130).
//
// 두 가지로 부하를 견딘다:
//   - 척도: 벽시계 대신 이 프로세스의 CPU 시간(사용자 + 시스템)을 잰다. 패키지 병렬 go test 처럼 다른 프로세스와 CPU 를 나눠 써도
//     기다린 시간은 들어가지 않는다. 이차 시간이면 CPU 시간도 네 배가 되니 잡는 힘은 같다. CPU 시간을 잘게 재지 못하는 곳
//     (Windows — 15.6ms 단위)은 벽시계다.
//   - 다시 재기: 판정이 실패로 보이면 몇 번 더 재어 가장 작은 값으로 다시 판정한다. 부하로 부푼 값은 다시 재면
//     작아지지만, 진짜 이차면 몇 번을 재도 네 배다.
//
// 이차 판정(AssertDoubling)은 수열 전체로 한다 — 두 배를 k 번 하면 마지막 비용이 첫 비용의 3^k 배(+k×Slack)를 넘지 않아야 한다.
// 이차면 4^k 라 잡히고, 작업 집합이 캐시 한 단을 넘는 한 구간에서만 5배 안팎으로 튀는 계단(GitHub 러너의 x86, 앞뒤 구간은 2배 안팎)은
// 통과한다. 쌍마다 판정하면 그 계단에 걸린다.
package timingtest

import (
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
	Slack time.Duration // 수열 판정의 덧셈 여유(두 배 한 번마다): 마지막 비용 ≤ 3^k × 첫 비용 + k × Slack(k = 두 배 횟수)
	Limit time.Duration // 각 크기 비용의 상한(0 이면 보지 않는다)
}

// AssertDoubling 은 크기가 두 배씩 커지는 sizes 마다 prepare(size) 가 돌려준 일을 재어, 수열 전체로 마지막 크기의 비용이
// 첫 크기 비용의 3^k 배 + k×Slack(k = len(sizes)−1, 이차면 4^k 배)을 넘지 않고 각 크기가 Limit 안인지 본다. 쌍마다의 비는 판정하지 않는다.
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
	n := len(ds)
	bound := func() time.Duration { // 3^k × 첫 비용 + k × Slack
		b := ds[0]
		for range n - 1 {
			b *= 3
		}
		return b + time.Duration(n-1)*o.Slack
	}
	growthBad := func() bool { return n > 1 && ds[n-1] > bound() }
	limitBad := func(i int) bool { return o.Limit > 0 && ds[i] > o.Limit }
	bad := func() bool {
		if growthBad() {
			return true
		}
		for i := range ds {
			if limitBad(i) {
				return true
			}
		}
		return false
	}
	for r := 0; r < Retries && bad(); r++ {
		before := append([]time.Duration(nil), ds...)
		for i := range ds { // 모든 크기를 같은 때에 잰다(작은 쪽부터)
			ds[i] = min(ds[i], Cost(works[i]))
		}
		t.Logf("%s: 다시 잼(%d) %v → %v", name, r+1, before, ds)
	}
	t.Logf("%s: %v", name, ds)
	if growthBad() {
		t.Errorf("%s: 크기 %d → %d(두 배 %d 번)에 비용 %v → %v, 상한 %v(3^%d 배 + %d×%v) (이차?)",
			name, sizes[0], sizes[n-1], n-1, ds[0], ds[n-1], bound(), n-1, n-1, o.Slack)
	}
	for i := range ds {
		if limitBad(i) {
			t.Errorf("%s: 크기 %d 에 %v (상한 %v)", name, sizes[i], ds[i], o.Limit)
		}
	}
}
