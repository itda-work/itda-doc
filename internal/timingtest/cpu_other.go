//go:build !unix

package timingtest

import "time"

// cpuTime 은 잘게 잴 수 없어(Windows 의 프로세스 시간은 15.6ms 단위) 벽시계로 대신하게 한다.
func cpuTime() (time.Duration, bool) { return 0, false }
