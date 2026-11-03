package gantt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 날짜 꼴(dateFormat)·축 라벨 꼴(axisFormat)·기간·눈금 단위. 시각은 time.Time(UTC) 로만 다룬다 —
// 로캘·시간대·오늘 날짜에 의존하지 않는다(itda-hyve#409 공통 규칙 4). 요일·월 이름 지시자(%a·%b …)는 지원하지 않는다.

// ---- 날짜 꼴(dateFormat)

// 토큰은 YYYY·MM·DD·HH·mm 넷(폭 고정), 구분자는 -·/·.·공백·: 다(Mermaid dayjs 토큰의 부분집합).
// 다른 토큰(X·D·M·ss …)은 이 판이 그리지 않는다.

type datePart struct {
	tok string // 빈 문자열이면 구분자
	w   int    // 토큰의 자릿수
	sep byte   // 구분자 글자(tok 이 빈 문자열일 때)
}

// dateFmt 는 읽은 dateFormat 이다.
type dateFmt struct {
	parts []datePart
	fmt   string
}

var dateToks = []struct {
	name string
	w    int
}{
	{"YYYY", 4}, {"MM", 2}, {"DD", 2}, {"HH", 2}, {"mm", 2},
}

func isDateSep(c byte) bool {
	return c == '-' || c == '/' || c == '.' || c == ':' || c == ' ' || c == '\t'
}

// parseDateFormat 은 dateFormat 꼴을 읽는다(토큰은 긴 것부터, 토큰 사이 구분자). 다른 토큰·글자는 오류다.
func parseDateFormat(s string) (*dateFmt, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("dateFormat 값이 비어 있음")
	}
	var parts []datePart
	for i := 0; i < len(s); {
		matched := false
		for _, t := range dateToks {
			if strings.HasPrefix(s[i:], t.name) {
				for _, q := range parts {
					if q.tok == t.name {
						return nil, fmt.Errorf("dateFormat 토큰 %s 가 두 번 있음", t.name)
					}
				}
				parts = append(parts, datePart{tok: t.name, w: t.w})
				i += len(t.name)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if isDateSep(s[i]) {
			parts = append(parts, datePart{sep: s[i]}) // 구분자는 한 글자씩
			i++
			continue
		}
		return nil, fmt.Errorf("dateFormat 토큰 %q 를 지원하지 않음(YYYY·MM·DD·HH·mm 와 구분자 -·/·.·공백·:)", clipB(s[i:]))
	}
	return &dateFmt{parts: parts, fmt: s}, nil
}

func clipB(s string) string {
	r := []rune(s)
	if len(r) > 12 {
		return string(r[:12]) + "…"
	}
	return s
}

// parse 는 dateFormat 꼴로 날짜 값을 읽는다. 값이 꼴과 다르거나(남는 글자·자릿수) 날짜로 성립하지 않으면 오류다 —
// Mermaid 는 new Date() 로 물러가지만 이 판은 그리지 않는다(원문 + 경고).
func (d *dateFmt) parse(v string) (time.Time, error) {
	year, month, day, hour, min := 1, 1, 1, 0, 0
	i := 0
	for _, p := range d.parts {
		if p.tok == "" { // 구분자 — 꼴과 같은 글자여야 한다(2026/01/01 을 YYYY-MM-DD 로 안 읽는다)
			if i >= len(v) || v[i] != p.sep {
				return time.Time{}, fmt.Errorf("dateFormat %s 와 맞지 않는 날짜 %q", d.fmt, clipB(v))
			}
			i++
			continue
		}
		if i+p.w > len(v) {
			return time.Time{}, fmt.Errorf("dateFormat %s 와 맞지 않는 날짜 %q", d.fmt, clipB(v))
		}
		n, ok := digits(v[i : i+p.w]) // 부호(+1)·빈칸은 받지 않는다 — 자릿수 모두 ASCII 숫자
		if !ok {
			return time.Time{}, fmt.Errorf("dateFormat %s 와 맞지 않는 날짜 %q", d.fmt, clipB(v))
		}
		switch p.tok {
		case "YYYY":
			year = n
		case "MM":
			month = n
		case "DD":
			day = n
		case "HH":
			hour = n
		case "mm":
			min = n
		}
		i += p.w
	}
	if strings.TrimSpace(v[i:]) != "" {
		return time.Time{}, fmt.Errorf("dateFormat %s 와 맞지 않는 날짜 %q", d.fmt, clipB(v))
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || min > 59 {
		return time.Time{}, fmt.Errorf("dateFormat %s 와 맞지 않는 날짜 %q", d.fmt, clipB(v))
	}
	t := time.Date(year, time.Month(month), day, hour, min, 0, 0, time.UTC)
	if t.Day() != day { // 2월 30일 같은 날
		return time.Time{}, fmt.Errorf("dateFormat %s 와 맞지 않는 날짜 %q", d.fmt, clipB(v))
	}
	return t, nil
}

// digits 는 고정 자릿수 토큰 값을 읽는다 — 모든 바이트가 ASCII 숫자일 때만(strconv.Atoi 는 +1·-1 을 받는다).
func digits(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, s != ""
}

// 꼴의 토큰을 작은 단위부터 — inclusiveEndDates 의 최소 단위·시각 꼴 판정에 쓴다.
var tokRank = map[string]int{"mm": 0, "HH": 1, "DD": 2, "MM": 3, "YYYY": 4}

// minTok 은 꼴의 모든 토큰 가운데 가장 작은 단위다(토큰이 없으면 "").
func (d *dateFmt) minTok() string {
	best := ""
	for _, p := range d.parts {
		if p.tok != "" && (best == "" || tokRank[p.tok] < tokRank[best]) {
			best = p.tok
		}
	}
	return best
}

// has 는 꼴에 토큰이 있는지 본다.
func (d *dateFmt) has(tok string) bool {
	for _, p := range d.parts {
		if p.tok == tok {
			return true
		}
	}
	return false
}

// dateOnly 는 꼴에서 시각부(첫 시각 토큰 HH·mm 부터 그 뒤 전부와 그 앞의 구분자까지)를 뺀 부분 꼴이다 —
// 시각이 있는 꼴의 excludes 값 가운데 날짜 부분만 적은 것(「2026-01-06」)을 가려낼 때 쓴다(parse.go resolveExcludes).
func (d *dateFmt) dateOnly() *dateFmt {
	var parts []datePart
	for _, p := range d.parts {
		if p.tok == "HH" || p.tok == "mm" {
			break
		}
		parts = append(parts, p)
	}
	if len(parts) > 0 && parts[len(parts)-1].tok == "" { // 시각부 앞의 구분자(보통 공백)도 뺀다
		parts = parts[:len(parts)-1]
	}
	return &dateFmt{parts: parts, fmt: d.fmt}
}

// inclusiveStep 은 inclusiveEndDates 의 더하는 양이다 — 꼴의 모든 토큰을 본 뒤 가장 작은 단위 하나만큼:
// 분이 있으면 1분, 시까지면 1시간, 일까지면 하루, 달까지면 한 달, 년만 있으면 1년.
func (d *dateFmt) inclusiveStep(t time.Time) time.Time {
	switch d.minTok() {
	case "mm":
		return t.Add(time.Minute)
	case "HH":
		return t.Add(time.Hour)
	case "MM":
		return t.AddDate(0, 1, 0)
	case "YYYY":
		return t.AddDate(1, 0, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// ---- 축 라벨 꼴(axisFormat)

// 지시자 %Y·%y·%m·%d·%e·%H·%M 과 글자(한글 포함). %e 는 0 없는 일(d3-time-format 과 겉보기가 같다 —
// SVG 글자의 앞공백은 무시된다). 다른 % 지시자는 이 판이 그리지 않는다.

type axisPart struct {
	d byte // 0 이면 글자
	s string
}

// axisFmt 는 읽은 axisFormat 이다.
type axisFmt struct {
	parts []axisPart
}

func parseAxisFormat(s string) (*axisFmt, error) {
	var parts []axisPart
	for i := 0; i < len(s); {
		if s[i] != '%' { // 글자는 다음 % 까지 바이트 구간 그대로(한글 UTF-8 을 깨지 않는다)
			j := strings.IndexByte(s[i:], '%')
			if j < 0 {
				j = len(s) - i
			}
			parts = append(parts, axisPart{s: s[i : i+j]})
			i += j
			continue
		}
		if i+1 >= len(s) {
			return nil, fmt.Errorf("axisFormat 의 %% 뒤가 비어 있음")
		}
		switch s[i+1] {
		case 'Y', 'y', 'm', 'd', 'e', 'H', 'M':
			parts = append(parts, axisPart{d: s[i+1]})
		default:
			r := []rune(s[i+1:])
			return nil, fmt.Errorf("axisFormat 지시자 %%%c 를 지원하지 않음(%%Y·%%y·%%m·%%d·%%e·%%H·%%M)", r[0])
		}
		i += 2
	}
	return &axisFmt{parts: parts}, nil
}

// has 는 지시자 하나가 있는지 본다.
func (a *axisFmt) has(d byte) bool {
	for _, p := range a.parts {
		if p.d == d {
			return true
		}
	}
	return false
}

func (a *axisFmt) format(t time.Time) string {
	var b strings.Builder
	for _, p := range a.parts {
		switch p.d {
		case 0:
			b.WriteString(p.s)
		case 'Y':
			b.WriteString(strconv.Itoa(t.Year()))
		case 'y':
			b.WriteString(fmt.Sprintf("%02d", t.Year()%100))
		case 'm':
			b.WriteString(fmt.Sprintf("%02d", int(t.Month())))
		case 'd':
			b.WriteString(fmt.Sprintf("%02d", t.Day()))
		case 'e':
			b.WriteString(strconv.Itoa(t.Day()))
		case 'H':
			b.WriteString(fmt.Sprintf("%02d", t.Hour()))
		case 'M':
			b.WriteString(fmt.Sprintf("%02d", t.Minute()))
		}
	}
	return b.String()
}

// ---- 기간(Nd·Nw·Nh·Nm·Ns·Ny·NM·Nms)

// 단위는 Mermaid ganttDb 그대로다(ms·s·m·h·d·w·M·y — 실측 2026-10-29). 이 판은 정수만 받는다(Mermaid 는 소수도 받는다).
var durUnits = []string{"ms", "y", "M", "w", "d", "h", "m", "s"}

// durMax 는 단위마다 받는 가장 큰 수다 — 날짜 범위 상한(MaxYears 년, 1년 = 365.25일)을 그 단위로 잰 값.
// 곱하기·AddDate 전에 본다(5124096h 가 int64 곱셈에서 25분으로 돌아오는 것을 막는다). 가장 큰 ms 도
// time.Duration(int64 나노초)에 들어간다(3.16e12 ms × 1e6 < 9.2e18).
var durMax = map[string]int64{
	"y": MaxYears, "M": MaxYears * 12, "w": MaxYears * 36525 / 700, "d": MaxYears * 36525 / 100,
	"h": MaxYears * 8766, "m": MaxYears * 8766 * 60, "s": MaxYears * 8766 * 3600, "ms": MaxYears * 8766 * 3600 * 1000,
}

type dur struct {
	n    int
	unit string
}

// parseDur 는 기간을 읽는다. 기간 꼴이 아니면 ok=false, 꼴이지만 수가 단위 상한(durMax)을 넘으면 err 다.
func parseDur(s string) (d dur, ok bool, err error) {
	for _, u := range durUnits {
		if strings.HasSuffix(s, u) {
			num := strings.TrimSuffix(s, u)
			if len(num) > 16 { // 상한(가장 큰 ms 가 16자리)을 넘는 자릿수 — 읽기 전에(int64 넘침 없이) 판정
				return dur{}, true, fmt.Errorf("기간 %q 이 %d년을 넘음", clipB(s), MaxYears)
			}
			n, okd := digits(num)
			if !okd {
				return dur{}, false, nil
			}
			if int64(n) > durMax[u] {
				return dur{}, true, fmt.Errorf("기간 %q 이 %d년을 넘음", clipB(s), MaxYears)
			}
			return dur{n: n, unit: u}, true, nil
		}
	}
	return dur{}, false, nil
}

// add 는 달력 단위(y·M·w·d 는 AddDate, 그 아래는 시간)로 더한다.
func (d dur) add(t time.Time) time.Time {
	switch d.unit {
	case "y":
		return t.AddDate(d.n, 0, 0)
	case "M":
		return t.AddDate(0, d.n, 0)
	case "w":
		return t.AddDate(0, 0, 7*d.n)
	case "d":
		return t.AddDate(0, 0, d.n)
	case "h":
		return t.Add(time.Duration(d.n) * time.Hour)
	case "m":
		return t.Add(time.Duration(d.n) * time.Minute)
	case "s":
		return t.Add(time.Duration(d.n) * time.Second)
	default: // ms
		return t.Add(time.Duration(d.n) * time.Millisecond)
	}
}

// ---- 눈금 간격(사다리)

// 눈금은 d3 scaleTime().ticks() 와 같은 꼴이다(Mermaid 가 그것을 쓴다): 후보 간격의 사다리에서 목표 눈금 수에 가장 가까운
// 간격을 고르고, 눈금은 그 간격의 달력 경계(분·시·일·달은 상위 단위 안에서 n 의 배수, 주는 일요일, 년은 n 의 배수)에 선다.
// 격자선과 라벨은 같은 눈금이다(걸러 찍지 않는다).

type tickUnit int

const (
	tickMinute tickUnit = iota
	tickHour
	tickDay
	tickWeek // 일요일 시작(d3 timeWeek 와 같다)
	tickMonth
	tickYear
)

// MaxTickEvery 는 tickInterval 의 수 상한이다(1000day·1000week·1000month) — AddDate 가 제자리로 돌아오는 큰 수를 막는다.
const MaxTickEvery = 1000

// rung 은 눈금 간격 하나(단위 × n)다.
type rung struct {
	u tickUnit
	n int
}

// approx 는 간격의 어림 길이다(사다리에서 목표에 가까운 칸을 고를 때만 — d3 의 durationMonth 30일·durationYear 365일과 같다).
func (r rung) approx() time.Duration {
	d := map[tickUnit]time.Duration{tickMinute: time.Minute, tickHour: time.Hour, tickDay: 24 * time.Hour,
		tickWeek: 7 * 24 * time.Hour, tickMonth: 30 * 24 * time.Hour, tickYear: 365 * 24 * time.Hour}[r.u]
	return d * time.Duration(r.n)
}

func (r rung) String() string {
	return strconv.Itoa(r.n) + [...]string{"minute", "hour", "day", "week", "month", "year"}[r.u]
}

// ladder 는 후보 간격의 사다리다 — d3 tickIntervals 의 분·시·일·주·달·년(초는 꼴에 없어 뺀다)에 d3 가 년 위에서 쓰는
// 1·2·5 배수를 100년까지 더한다. 분 칸은 꼴에 분이 있을 때만, 시 칸은 꼴에 시(또는 분)가 있을 때만 넣는다.
// d3 와 다른 점 하나: 1주와 1개월 사이에 2주 칸을 둔다 — 이 판은 라벨이 겹치면 다음 칸으로 가므로(d3 는 겹친 채 그린다)
// 두 달 남짓한 그림이 1개월(라벨 2~3개)로 뛰지 않게. 2주 눈금의 자리는 d3 timeWeek.every(2) 와 같다.
func ladder(minutes, hours bool) []rung {
	var out []rung
	if minutes {
		out = append(out, rung{tickMinute, 1}, rung{tickMinute, 5}, rung{tickMinute, 15}, rung{tickMinute, 30})
	}
	if minutes || hours {
		out = append(out, rung{tickHour, 1}, rung{tickHour, 3}, rung{tickHour, 6}, rung{tickHour, 12})
	}
	out = append(out, rung{tickDay, 1}, rung{tickDay, 2}, rung{tickWeek, 1}, rung{tickWeek, 2}, rung{tickMonth, 1}, rung{tickMonth, 3})
	for _, n := range []int{1, 2, 5, 10, 20, 50, 100} {
		out = append(out, rung{tickYear, n})
	}
	return out
}

// nearest 는 사다리에서 목표 간격 target 에 가장 가까운 칸(로그 거리 — d3 의 tickInterval 과 같은 판정)의 차례다.
func nearest(lad []rung, target time.Duration) int {
	i := 0
	for i < len(lad) && lad[i].approx() <= target { // bisector.right
		i++
	}
	switch {
	case i == 0:
		return 0
	case i == len(lad):
		return len(lad) - 1
	}
	if float64(target)/float64(lad[i-1].approx()) < float64(lad[i].approx())/float64(target) {
		return i - 1
	}
	return i
}

// parseTickInterval 은 tickInterval 값을 읽는다(Mermaid 의 패턴에서 이 판이 받는 단위는 day·week·month 다).
func parseTickInterval(s string) (rung, error) {
	v := strings.TrimSpace(s)
	for _, u := range []struct {
		name string
		tu   tickUnit
	}{
		{"day", tickDay}, {"week", tickWeek}, {"month", tickMonth},
	} {
		if !strings.HasSuffix(v, u.name) {
			continue
		}
		num := strings.TrimSuffix(v, u.name)
		n, ok := digits(num)
		if !ok || n < 1 {
			return rung{}, fmt.Errorf("tickInterval 값 %q 이 숫자+단위가 아님", clipB(v))
		}
		if len(num) > 4 || n > MaxTickEvery {
			return rung{}, fmt.Errorf("tickInterval 의 수 %s 가 %d 을 넘음", clipB(num), MaxTickEvery)
		}
		return rung{u.tu, n}, nil
	}
	// Mermaid 는 millisecond·second·minute·hour 도 받는다 — 이 판은 그리지 않는다
	for _, u := range []string{"millisecond", "second", "minute", "hour"} {
		if strings.HasSuffix(v, u) {
			return rung{}, fmt.Errorf("tickInterval 단위 %s 를 지원하지 않음(day·week·month)", u)
		}
	}
	return rung{}, fmt.Errorf("tickInterval 값 %q 을 지원하지 않음(숫자 + day·week·month)", clipB(v))
}

// weekEpoch 는 d3 timeWeek.count(0, …) 의 기준 — 1970-01-01 앞의 일요일. n 주 간격은 이 날부터 센 주 번호의 n 배수에 선다.
var weekEpoch = time.Date(1969, 12, 28, 0, 0, 0, 0, time.UTC)

func floorDiv(a, n int) int {
	q := a / n
	if a%n != 0 && (a < 0) != (n < 0) {
		q--
	}
	return q
}

// alignFloor 는 t 이하에서 가장 늦은 눈금 자리다(d3 interval.every(n) 의 필드 규칙: 분·시·일·달은 상위 단위 안의
// 필드 값이 n 의 배수, 주는 기준 일요일부터의 주 번호가 n 의 배수, 년은 연도가 n 의 배수).
func alignFloor(t time.Time, r rung) time.Time {
	y, mo, d := t.Date()
	switch r.u {
	case tickMinute:
		return time.Date(y, mo, d, t.Hour(), t.Minute()/r.n*r.n, 0, 0, time.UTC)
	case tickHour:
		return time.Date(y, mo, d, t.Hour()/r.n*r.n, 0, 0, 0, time.UTC)
	case tickDay:
		return time.Date(y, mo, (d-1)/r.n*r.n+1, 0, 0, 0, 0, time.UTC)
	case tickWeek:
		day := time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
		day = day.AddDate(0, 0, -int(day.Weekday()))
		// 주 번호는 Unix 초의 정수 차로 센다 — time.Duration(Sub)은 ±292년에서 포화돼 1500년·2500년의 주 번호가 틀린다.
		// day·weekEpoch 모두 UTC 일요일 0시라 차는 7일(604800초)의 정확한 배수다(음수도 나머지 없이 나뉜다).
		k := int((day.Unix() - weekEpoch.Unix()) / (7 * 86400))
		return day.AddDate(0, 0, -7*(k-floorDiv(k, r.n)*r.n))
	case tickMonth:
		return time.Date(y, time.Month((int(mo)-1)/r.n*r.n+1), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(floorDiv(y, r.n)*r.n, 1, 1, 0, 0, 0, 0, time.UTC)
	}
}

// nextTick 은 눈금 다음 눈금이다 — 상위 단위의 경계에서 다시 센다(d3 와 같게: 2일 간격은 1·3·…·31 일 뒤 다음 달 1일).
func nextTick(t time.Time, r rung) time.Time {
	y, mo, d := t.Date()
	var n, up time.Time
	switch r.u {
	case tickMinute:
		n, up = t.Add(time.Duration(r.n)*time.Minute), time.Date(y, mo, d, t.Hour()+1, 0, 0, 0, time.UTC)
	case tickHour:
		n, up = t.Add(time.Duration(r.n)*time.Hour), time.Date(y, mo, d+1, 0, 0, 0, 0, time.UTC)
	case tickDay:
		n, up = t.AddDate(0, 0, r.n), time.Date(y, mo+1, 1, 0, 0, 0, 0, time.UTC)
	case tickWeek:
		return t.AddDate(0, 0, 7*r.n)
	case tickMonth:
		n, up = t.AddDate(0, r.n, 0), time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return t.AddDate(r.n, 0, 0)
	}
	if up.Before(n) {
		return up
	}
	return n
}

// genTicks 는 lo..hi 의 눈금 시각이다 — 첫 눈금은 lo 를 간격의 경계로 올린 시각(lo 이상), 마지막은 hi 이하.
// 많아야 MaxTicks+1 개까지만 만든다(넘었는지는 부르는 쪽이 길이로 본다). 다음 시각이 지금보다 늦지 않으면(달력 덧셈이
// 제자리로 돌아오는 큰 수) 그 자리에서 오류로 멈춘다 — 눈금 생성이 끝나지 않는 일이 없다.
func genTicks(lo, hi time.Time, r rung) ([]time.Time, error) {
	t := alignFloor(lo, r)
	if t.Before(lo) {
		nt := nextTick(t, r)
		if !nt.After(t) {
			return nil, fmt.Errorf("눈금 간격 %s 이 앞으로 나아가지 않음", r)
		}
		t = nt
	}
	var out []time.Time
	for !t.After(hi) {
		out = append(out, t)
		if len(out) > MaxTicks {
			break
		}
		nt := nextTick(t, r)
		if !nt.After(t) {
			return nil, fmt.Errorf("눈금 간격 %s 이 앞으로 나아가지 않음", r)
		}
		t = nt
	}
	return out, nil
}

// ---- 제외 날짜

// excluded 는 시각 t 가 제외인지 본다 — Mermaid ganttDb.isInvalidDate 와 같은 규칙: 주말(토·일), 요일 이름,
// 그리고 dateFormat 으로 포맷한 날짜 문자열의 일치(문자열 비교라 dateFormat 이 YYYY-MM 인 값은 그 달의 모든 날에 걸린다).
type excludes struct {
	weekends bool
	weekdays [7]bool
	dates    map[string]bool
	none     bool // 아무 것도 없다(판정 비용을 덜기 위한 표시)
}

func (e *excludes) check(t time.Time, df *dateFmt) bool {
	if e.none {
		return false
	}
	if e.weekends && (t.Weekday() == time.Saturday || t.Weekday() == time.Sunday) {
		return true
	}
	if e.weekdays[t.Weekday()] {
		return true
	}
	return e.dates[df.fmtString(t)]
}

// fmtString 은 시각을 dateFormat 꼴로 포맷한다(parse 의 역방향).
func (d *dateFmt) fmtString(t time.Time) string {
	var b strings.Builder
	for _, p := range d.parts {
		switch p.tok {
		case "":
			b.WriteByte(p.sep)
		case "YYYY":
			b.WriteString(fmt.Sprintf("%04d", t.Year()))
		case "MM":
			b.WriteString(fmt.Sprintf("%02d", int(t.Month())))
		case "DD":
			b.WriteString(fmt.Sprintf("%02d", t.Day()))
		case "HH":
			b.WriteString(fmt.Sprintf("%02d", t.Hour()))
		case "mm":
			b.WriteString(fmt.Sprintf("%02d", t.Minute()))
		}
	}
	return b.String()
}

var weekdayNames = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}
