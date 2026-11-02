package mdoc

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// 차트(1.11). :::chart{type=… unit=… labels=off table=below ref=… ref-label=… line=… axis2=… unit2=…} 안의 GFM 표를 인라인 SVG 로 그린다.
// 데이터는 표 그대로다 — 다른 뷰어에서는 표로 보인다. 첫 열은 항목, 나머지 열은 계열(머리 행이 계열 이름).
// 산점도·버블(scatter·bubble)만 표 꼴이 라벨 | x | y [| 크기] 다(chartscatter.go).
// 표 캡션(Table: …)이 차트 캡션이 된다.
//
// 그리기(chartsvg.go)는 스크립트·이벤트 속성·style 없이 허용 목록의 SVG 요소·속성만 쓴다. 색·선은 클래스(s1…s8)이고
// 값은 테마 토큰(--chart-1…)이 준다. 글자는 문서 글꼴이라 PDF 에서 선택·검색된다.

// Chart 는 :::chart 를 읽은 결과다. 렌더러가 SVG 로 그린다.
type Chart struct {
	ast.BaseBlock
	ChartType  string // bar · hbar · stacked · line · area · pie · donut · scatter · bubble
	Unit       string
	Labels     bool // 값 표시(labels=off 면 거짓)
	TableBelow bool // 표도 함께 보인다(table=below)
	ID         string
	Classes    []string
	Cats       []string
	Series     []chartSeries
	Caption    string
	Decimals   int // 값을 적을 소수 자리(표에 적힌 자리 중 가장 긴 것, 3 까지 — 0 으로 보일 값만 더 길게, valText)
	Ref        *chartRef
	Line       int // 선으로 그릴 계열(line="계열 이름", bar 에서만 — 나머지는 묶은 막대). 없으면 -1
	// 보조 축(axis2="계열 이름" unit2="%", bar·line 에서만): 그 계열을 오른쪽 축으로 그린다. 단위·소수 자리·% 추론은 축마다다 —
	// Unit·Decimals 는 왼쪽 축(나머지 계열), Unit2·Decimals2 는 오른쪽 축. bar 에서는 그 계열이 선이다(Line 과 같다). 없으면 Axis2 = -1
	Axis2     int
	Unit2     string
	Decimals2 int
	Scatter   *chartScatter // 산점도·버블(type=scatter·bubble)의 점 — 이 두 종류는 Cats·Series 를 쓰지 않는다(chartscatter.go)
}

// chartRef 는 기준선(ref=40 ref-label="목표")이다. 값 축의 값 하나에 점선을 긋고 라벨을 단다. 축 범위에 들어간다.
type chartRef struct {
	Value    float64
	Decimals int // ref 에 적힌 소수 자리(3 까지) — 값 글자보다 길면 이 자리로 적는다
	Label    string
}

type chartSeries struct {
	Name   string
	Col    int // 이 계열을 읽은 표의 열(0 부터 — 0 은 항목 열). 그림에는 쓰지 않는다(ChartValues 가 원본 열과 맞추는 데 쓴다)
	Values []float64
	Has    []bool // 빈 칸이면 거짓
}

var KindChart = ast.NewNodeKind("Chart")

func (n *Chart) Kind() ast.NodeKind { return KindChart }
func (n *Chart) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Type": n.ChartType}, nil)
}

// 경고 코드(차트).
const (
	WarnChart = "chart" // 차트를 그리지 못했거나 일부 값을 버림
)

// 상한. 넘으면 자르고 경고한다 — 색은 테마 토큰 8개이고, 그보다 많은 계열은 색으로 가릴 수 없다.
const (
	chartMaxSeries = 8
	chartMaxCats   = 60
	chartMaxHBar   = 30
	chartMaxSlices = 8 // 넘으면 앞의 7개 + "기타"
)

var chartTypes = map[string]bool{"bar": true, "hbar": true, "stacked": true, "line": true, "area": true, "pie": true, "donut": true,
	"scatter": true, "bubble": true}

// chartTypeList 는 경고에 적는 종류 목록이다(차례 고정). chartAttrList 는 받는 속성(#id·.class 말고)이다 — 안내서 2장의 속성 표와 테스트가 맞춰 본다.
const (
	chartTypeList = "bar·hbar·stacked·line·area·pie·donut·scatter·bubble"
	chartAttrList = "type·unit·labels·table·ref·ref-label·line·axis2·unit2"
)

// ChartTypeNames 는 받는 종류를 정한 차례로 돌려준다(문서 창의 "CSV 를 표로" 대화상자가 차트로 감쌀 종류를 고르는 데 쓴다).
func ChartTypeNames() []string { return strings.Split(chartTypeList, "·") }

// ChartRowLimit 은 그 종류가 그리는 행(항목·점)의 상한이다. 넘는 행은 그리지 않고 경고한다(표 데이터는 문서에 남는다).
// 모르는 종류는 0 이다.
func ChartRowLimit(typ string) int {
	switch strings.ToLower(typ) {
	case "hbar":
		return chartMaxHBar
	case "scatter":
		return chartMaxPoints
	case "bubble":
		return chartMaxBubbles
	}
	if chartTypes[strings.ToLower(typ)] {
		return chartMaxCats
	}
	return 0
}

// axis 는 계열 si 의 값 축이다: 0 왼쪽(주 축), 1 오른쪽(보조 축).
func (ch *Chart) axis(si int) int {
	if si == ch.Axis2 && si >= 0 {
		return 1
	}
	return 0
}

// unitOf 는 축 ax 의 단위와 소수 자리다.
func (ch *Chart) unitOf(ax int) (string, int) {
	if ax == 1 {
		return ch.Unit2, ch.Decimals2
	}
	return ch.Unit, ch.Decimals
}

// chart 는 :::chart 컨테이너를 차트 노드로 바꾼다. 표가 없거나 계열이 없으면 그대로 두고 경고한다(안의 내용이 그대로 나간다).
func (st *state) chart(c *Container) {
	line := st.lines.line(c.Pos())
	var tbl *extast.Table
	for n := c.FirstChild(); n != nil; n = n.NextSibling() {
		if t, ok := n.(*extast.Table); ok {
			tbl = t
			break
		}
	}
	if tbl == nil {
		st.warn(line, WarnChart, ":::chart 안에 표가 없어 차트를 그리지 않음")
		return
	}
	// 모르는 type 은 그리지 않고 표를 그대로 둔다 — 새 판의 종류(예: 0.16.0 판에게 산점도의 x·y 열)를 옛 판이 막대로 잘못 그리지 않게.
	// 다른 속성은 그 종류의 것일 수 있어 따로 경고하지 않는다.
	for _, kv := range c.Attrs.Data {
		if v := strings.TrimSpace(kv[1]); kv[0] == "type" && !chartTypes[strings.ToLower(v)] {
			st.warn(line, WarnChart, fmt.Sprintf("차트 type %q 를 몰라 차트를 그리지 않고 표로 둠(이 판이 아는 종류: %s)", v, chartTypeList))
			return
		}
	}
	ch := &Chart{ChartType: "bar", Labels: true, ID: c.Attrs.ID, Line: -1, Axis2: -1}
	for _, cls := range c.Attrs.Classes {
		if cls != "chart" {
			ch.Classes = append(ch.Classes, cls)
		}
	}
	var refRaw, refLabel, lineName, axis2Name string
	var hasRef, hasRefLabel, hasLine, hasAxis2, hasUnit2 bool
	for _, kv := range c.Attrs.Data {
		switch k, v := kv[0], strings.TrimSpace(kv[1]); k {
		case "type":
			ch.ChartType = strings.ToLower(v) // 모르는 종류는 위에서 돌려보냈다
		case "ref":
			refRaw, hasRef = v, true
		case "ref-label":
			refLabel, hasRefLabel = strings.Join(strings.Fields(v), " "), true
		case "line":
			lineName, hasLine = strings.Join(strings.Fields(v), " "), true
		case "axis2":
			axis2Name, hasAxis2 = strings.Join(strings.Fields(v), " "), true
		case "unit":
			ch.Unit = v
		case "unit2":
			ch.Unit2, hasUnit2 = v, true
		case "labels":
			ch.Labels = !isOff(v)
		case "table":
			ch.TableBelow = strings.EqualFold(v, "below") || strings.EqualFold(v, "on")
		default:
			st.warn(line, WarnChart, fmt.Sprintf("차트 속성 %s 는 쓰지 않음(%s 만 — 색·글꼴은 테마가 정한다)", k, chartAttrList))
		}
	}

	var header []ast.Node
	var rows [][]ast.Node
	for n := tbl.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *TableCaption:
			ch.Caption = strings.TrimSpace(plainText(n, st.src))
		case *extast.TableHeader:
			header = cells(n)
		case *extast.TableRow:
			rows = append(rows, cells(n))
		}
	}
	if len(header) < 2 || len(rows) == 0 {
		st.warn(line, WarnChart, ":::chart 의 표에 계열(둘째 열부터)이나 행이 없어 차트를 그리지 않음")
		return
	}
	if hasLine && !st.readLine(ch, line, lineName, header) {
		return // 선 계열을 정하지 못하면 표 그대로 — 엉뚱한 계열을 선으로 그리지 않게
	}
	if hasAxis2 && !st.readAxis2(ch, line, axis2Name, header, hasLine) {
		return // 오른쪽 축 계열을 정하지 못하면 표 그대로 — 엉뚱한 계열을 다른 눈금으로 읽히지 않게
	}
	if hasUnit2 && ch.Axis2 < 0 {
		st.warn(line, WarnChart, "unit2 는 axis2(보조 축)와 함께 써야 해 무시함")
		ch.Unit2 = ""
	}
	if ch.ChartType == "scatter" || ch.ChartType == "bubble" {
		// 산점도·버블은 표 꼴(라벨 | x | y [| 크기])이 달라 따로 읽는다 — 계열·항목 상한도 따로다
		if !st.readScatter(ch, line, header, rows) {
			return
		}
		st.readRef(ch, line, refRaw, refLabel, hasRef, hasRefLabel)
		st.placeChart(c, ch, tbl)
		return
	}
	nser := len(header) - 1
	if nser > chartMaxSeries {
		st.warn(line, WarnChart, fmt.Sprintf("계열이 %d 개라 앞의 %d 개만 그림(색으로 가릴 수 있는 수)", nser, chartMaxSeries))
		nser = chartMaxSeries
	}
	if ch.ChartType == "pie" || ch.ChartType == "donut" {
		nser = 1 // 원 차트는 첫 계열만 쓴다
	}
	maxCats := chartMaxCats
	if ch.ChartType == "hbar" {
		maxCats = chartMaxHBar
	}
	if len(rows) > maxCats {
		st.warn(line, WarnChart, fmt.Sprintf("항목이 %d 개라 앞의 %d 개만 그림", len(rows), maxCats))
		rows = rows[:maxCats]
	}
	for i := 0; i < nser; i++ {
		ch.Series = append(ch.Series, chartSeries{Name: cellText(header[i+1], st.src), Col: i + 1})
	}
	// 축마다 % 로 적은 칸과 % 없이 적은 칸(첫 칸을 경고에 적는다)
	var pct, unitless [2]int
	var pctEx, plainEx [2]string
	attrUnit := [2]string{ch.Unit, ch.Unit2}
	for _, row := range rows {
		cat := ""
		if len(row) > 0 {
			cat = cellText(row[0], st.src)
		}
		ch.Cats = append(ch.Cats, cat)
		for si := range ch.Series {
			s := &ch.Series[si]
			raw := ""
			if si+1 < len(row) {
				raw = cellText(row[si+1], st.src)
			}
			ax := ch.axis(si)
			v, dec, isPct, ok, empty := parseChartNumber(raw, attrUnit[ax])
			if !ok && !empty {
				st.warn(st.nodeLine(row[si+1], line), WarnChart, fmt.Sprintf("차트 값 %q 를 숫자로 읽지 못해 빈 값으로 둠", clip(raw)))
			}
			if ok {
				if ax == 1 {
					ch.Decimals2 = max(ch.Decimals2, min(dec, 3))
				} else {
					ch.Decimals = max(ch.Decimals, min(dec, 3))
				}
				if isPct {
					pct[ax]++
					if pctEx[ax] == "" {
						pctEx[ax] = raw
					}
				} else {
					unitless[ax]++
					if plainEx[ax] == "" {
						plainEx[ax] = raw
					}
				}
			}
			s.Values = append(s.Values, v)
			s.Has = append(s.Has, ok)
		}
	}
	st.unitMix(ch, line, pct, unitless, pctEx, plainEx)
	if ch.ChartType == "pie" || ch.ChartType == "donut" {
		st.pieValues(ch, line)
	}
	any := false
	for _, s := range ch.Series {
		for _, h := range s.Has {
			any = any || h
		}
	}
	if !any {
		st.warn(line, WarnChart, ":::chart 의 표에 읽을 수 있는 숫자가 없어 차트를 그리지 않음")
		return
	}
	if ch.Axis2 >= 0 && !slices.Contains(ch.Series[ch.Axis2].Has, true) {
		// 오른쪽 축에 그릴 숫자가 없다 — 빈 눈금을 세우지 않는다(bar 면 그 계열은 그대로 빈 선이다)
		st.warn(line, WarnChart, fmt.Sprintf("axis2 %q 계열에 읽을 수 있는 숫자가 없어 오른쪽 축 없이 그림", clip(ch.Series[ch.Axis2].Name)))
		ch.Axis2, ch.Unit2 = -1, ""
	}
	st.readRef(ch, line, refRaw, refLabel, hasRef, hasRefLabel)
	st.placeChart(c, ch, tbl)
}

// placeChart 는 컨테이너 자리를 차트로 바꾼다. 표도 보일 때는 표(캡션 뺀)를 자식으로 남겨 그림 뒤에 낸다.
func (st *state) placeChart(c *Container, ch *Chart, tbl *extast.Table) {
	ch.SetPos(c.Pos())
	for n := c.FirstChild(); n != nil; {
		next := n.NextSibling()
		if n == ast.Node(tbl) {
			if ch.TableBelow {
				if cp, ok := tbl.FirstChild().(*TableCaption); ok {
					tbl.RemoveChild(tbl, cp)
				}
				ch.AppendChild(ch, n)
			}
		} else {
			ch.AppendChild(ch, n) // 표 말고 든 것(설명 문단 등)은 그림 뒤에 그대로 낸다
		}
		n = next
	}
	c.Parent().ReplaceChild(c.Parent(), c, ch)
}

// readLine 은 혼합 차트(막대 + 선, line="계열 이름")의 선 계열을 정한다. bar 에서만 받고, 계열은 이름으로 가리킨다 —
// 공백을 하나로 줄인 이름이 계열 머리 칸 하나와 정확히(대소문자까지) 같아야 한다. 숫자 이름을 열 번호로 읽지 않는다.
// 못 정하면(다른 종류·빈 이름·없는 이름·같은 이름 여럿·계열 상한 밖·막대가 남지 않음) 경고하고 거짓 — 차트 없이 표 그대로다.
func (st *state) readLine(ch *Chart, line int, name string, header []ast.Node) bool {
	if ch.ChartType != "bar" {
		st.warn(line, WarnChart, fmt.Sprintf("line(막대 + 선)은 bar 에만 써 차트를 그리지 않고 표로 둠(type=%s)", ch.ChartType))
		return false
	}
	si, ok := st.seriesByName(line, "line", "선으로 그릴", name, header)
	switch {
	case !ok:
	case len(header) == 2:
		st.warn(line, WarnChart, fmt.Sprintf("line %q 말고 막대로 그릴 계열이 없어 차트를 그리지 않고 표로 둠(선만이면 type=line)", clip(name)))
	default:
		ch.Line = si
		return true
	}
	return false
}

// seriesByName 은 속성 attr(line·axis2)이 이름으로 가리키는 계열(0 부터)이다. 공백을 하나로 줄인 이름이 계열 머리 칸 하나와
// 정확히(대소문자까지) 같아야 한다 — 숫자 이름을 열 번호로 읽지 않는다. 못 정하면(빈 이름·없는 이름·같은 이름 여럿·첫 열·
// 계열 상한 밖) 경고하고 거짓이다.
func (st *state) seriesByName(line int, attr, use, name string, header []ast.Node) (int, bool) {
	if name == "" {
		st.warn(line, WarnChart, attr+" 의 계열 이름이 비어 차트를 그리지 않고 표로 둠")
		return -1, false
	}
	var hit []int
	for i := 1; i < len(header); i++ {
		if cellText(header[i], st.src) == name {
			hit = append(hit, i-1)
		}
	}
	switch {
	case len(hit) == 0 && cellText(header[0], st.src) == name:
		st.warn(line, WarnChart, fmt.Sprintf("%s %q 는 첫 열(항목)이라 %s 수 없어 차트를 그리지 않고 표로 둠", attr, clip(name), use))
	case len(hit) == 0:
		st.warn(line, WarnChart, fmt.Sprintf("%s %q 와 같은 이름의 계열이 없어 차트를 그리지 않고 표로 둠", attr, clip(name)))
	case len(hit) > 1:
		st.warn(line, WarnChart, fmt.Sprintf("%s %q 와 같은 이름의 계열이 %d 개라 차트를 그리지 않고 표로 둠", attr, clip(name), len(hit)))
	case hit[0] >= chartMaxSeries:
		st.warn(line, WarnChart, fmt.Sprintf("%s %q 는 %d 번째 계열이라 상한(%d) 밖이어서 차트를 그리지 않고 표로 둠", attr, clip(name), hit[0]+1, chartMaxSeries))
	default:
		return hit[0], true
	}
	return -1, false
}

// readAxis2 는 보조 축(axis2="계열 이름")의 계열을 정한다. bar·line 에서만 받는다 — bar 에서는 그 계열을 선으로 그린다(막대를 두 눈금에
// 나눠 그리면 높이를 견주게 된다). line 이 함께 있으면 같은 계열이어야 한다(선은 하나). 왼쪽 축에 계열이 남지 않으면 받지 않는다.
// 못 정하면 경고하고 거짓 — 차트 없이 표 그대로다(line 과 같은 원칙).
func (st *state) readAxis2(ch *Chart, line int, name string, header []ast.Node, hasLine bool) bool {
	if ch.ChartType != "bar" && ch.ChartType != "line" {
		st.warn(line, WarnChart, fmt.Sprintf("axis2(보조 축)는 bar·line 에만 써 차트를 그리지 않고 표로 둠(type=%s)", ch.ChartType))
		return false
	}
	si, ok := st.seriesByName(line, "axis2", "오른쪽 축에 그릴", name, header)
	switch {
	case !ok:
	case len(header) == 2:
		st.warn(line, WarnChart, fmt.Sprintf("axis2 %q 말고 왼쪽 축에 그릴 계열이 없어 차트를 그리지 않고 표로 둠", clip(name)))
	case hasLine && ch.Line != si:
		st.warn(line, WarnChart, fmt.Sprintf("axis2 %q 와 line %q 가 다른 계열이라 차트를 그리지 않고 표로 둠(bar 의 오른쪽 축 계열은 선이고, 선은 하나)", clip(name), clip(cellText(header[ch.Line+1], st.src))))
	default:
		ch.Axis2 = si
		if ch.ChartType == "bar" {
			ch.Line = si
		}
		return true
	}
	return false
}

// unitMix 는 축마다 % 추론과 단위 혼합 경고다. unit 이 없고 읽은 값이 모두 % 면 그 축의 단위는 % 다. % 를 붙인 칸과 붙이지 않은 칸이
// 섞이면(unit 이 없을 때), 또는 unit 이 % 아닌 단위인데 % 를 붙인 칸이 있으면 경고한다 — 값은 % 를 떼고 같은 눈금에 그린다.
func (st *state) unitMix(ch *Chart, line int, pct, unitless [2]int, pctEx, plainEx [2]string) {
	for ax := 0; ax < 2; ax++ {
		unit := &ch.Unit
		name := "unit"
		if ax == 1 {
			if ch.Axis2 < 0 {
				break
			}
			unit, name = &ch.Unit2, "unit2"
		}
		where := ""
		if ch.Axis2 >= 0 {
			where = [2]string{"왼쪽 축의 ", "오른쪽 축의 "}[ax]
		}
		switch {
		case *unit == "" && pct[ax] > 0 && unitless[ax] == 0:
			*unit = "%"
		case *unit == "" && pct[ax] > 0:
			st.warn(line, WarnChart, fmt.Sprintf("%s값에 %% 를 붙인 칸(%q)과 붙이지 않은 칸(%q)이 섞여 단위 없이 그림(%% 를 모두 붙이거나 %s 로 단위를 적는다)",
				where, clip(pctEx[ax]), clip(plainEx[ax]), name))
		case *unit != "" && pct[ax] > 0:
			st.warn(line, WarnChart, fmt.Sprintf("%s=%q 인데 %s값에 %% 를 붙인 칸(%q)이 있어 %% 를 떼고 그림", name, clip(*unit), where, clip(pctEx[ax])))
		}
	}
}

// readRef 는 기준선 속성을 읽는다. 읽지 못하면 경고하고 기준선 없이 그린다(차트는 그대로).
func (st *state) readRef(ch *Chart, line int, raw, label string, has, hasLabel bool) {
	if !has {
		if hasLabel {
			st.warn(line, WarnChart, "ref-label 은 ref(기준선 값)와 함께 써야 해 무시함")
		}
		return
	}
	if ch.ChartType == "pie" || ch.ChartType == "donut" {
		st.warn(line, WarnChart, "원·도넛에는 기준선(ref)을 긋지 않음")
		return
	}
	if ch.Scatter != nil {
		// 값 축이 둘(x·y)이라 어느 축의 값인지 정해지지 않는다
		st.warn(line, WarnChart, "산점도·버블에는 기준선(ref)을 긋지 않음")
		return
	}
	if ch.Axis2 >= 0 {
		// 기준선 라벨의 자리(그림 영역 오른쪽 밖)를 오른쪽 축 눈금이 쓴다. 어느 축의 값인지도 헷갈린다
		st.warn(line, WarnChart, "보조 축(axis2)이 있는 차트에는 기준선(ref)을 긋지 않음")
		return
	}
	v, dec, _, ok, empty := parseChartNumber(raw, ch.Unit)
	if !ok {
		if empty {
			st.warn(line, WarnChart, "기준선 ref 의 값이 비어 긋지 않음")
		} else {
			st.warn(line, WarnChart, fmt.Sprintf("기준선 ref %q 를 숫자로 읽지 못해 긋지 않음", clip(raw)))
		}
		return
	}
	ch.Ref = &chartRef{Value: v, Decimals: min(dec, 3), Label: label}
}

// pieValues 는 원 차트의 값을 고른다: 음수·0 은 그리지 않고(경고), 조각이 많으면 앞의 7개 + 기타.
func (st *state) pieValues(ch *Chart, line int) {
	s := &ch.Series[0]
	for i, v := range s.Values {
		if s.Has[i] && v < 0 {
			st.warn(line, WarnChart, fmt.Sprintf("원 차트는 음수를 그리지 못해 %q 를 뺌", clip(ch.Cats[i])))
			s.Has[i] = false
		}
	}
	var idx []int
	for i := range s.Values {
		if s.Has[i] && s.Values[i] > 0 {
			idx = append(idx, i)
		}
	}
	if len(idx) <= chartMaxSlices {
		return
	}
	st.warn(line, WarnChart, fmt.Sprintf("원 차트 조각이 %d 개라 앞의 %d 개 말고는 \"기타\" 로 묶음", len(idx), chartMaxSlices-1))
	var cats []string
	var vals []float64
	var has []bool
	other := 0.0
	for k, i := range idx {
		if k < chartMaxSlices-1 {
			cats, vals, has = append(cats, ch.Cats[i]), append(vals, s.Values[i]), append(has, true)
		} else {
			other += s.Values[i]
		}
	}
	ch.Cats = append(cats, "기타")
	s.Values, s.Has = append(vals, other), append(has, true)
}

// nodeLine 은 노드 안 첫 글자의 원문 줄이다(표 칸은 위치를 따로 갖지 않는다). 못 찾으면 fallback.
func (st *state) nodeLine(n ast.Node, fallback int) int {
	line := 0
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering && line == 0 {
			line = st.lines.line(t.Segment.Start)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if line == 0 {
		return fallback
	}
	return line
}

func isOff(v string) bool {
	switch strings.ToLower(v) {
	case "off", "false", "no", "none", "0":
		return true
	}
	return false
}

func cells(row ast.Node) []ast.Node {
	var out []ast.Node
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*extast.TableCell); ok {
			out = append(out, c)
		}
	}
	return out
}

func cellText(n ast.Node, src []byte) string {
	return strings.Join(strings.Fields(plainText(n, src)), " ")
}

// 숫자 꼴: 1,234.5 · -3 · 12% · +4.5 · −2(유니코드 빼기). 천 단위 쉼표는 세 자리마다만.
var chartNumRe = regexp.MustCompile(`^([+\-−]?)((?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?|\.\d+)$`)

// parseChartNumber 는 칸 글자를 숫자로 읽는다. 빈 칸·"-"·"—" 는 빈 값이다(경고 없음).
// 단위가 적혀 있으면(unit="억 원" 인데 칸이 "38.2억 원") 떼고 읽는다.
func parseChartNumber(s, unit string) (v float64, dec int, pct, ok, empty bool) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", "-", "—", "–", "·", "n/a", "na":
		return 0, 0, false, false, true
	}
	if u := strings.TrimSpace(unit); u != "" && strings.HasSuffix(s, u) {
		s = strings.TrimSpace(strings.TrimSuffix(s, u))
	}
	if strings.HasSuffix(s, "%") {
		pct = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	}
	m := chartNumRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false, false, false
	}
	num := strings.ReplaceAll(m[2], ",", "")
	f, err := strconv.ParseFloat(num, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) || math.Abs(f) > 1e15 {
		return 0, 0, false, false, false
	}
	if m[1] == "-" || m[1] == "−" {
		f = -f
	}
	if i := strings.IndexByte(num, '.'); i >= 0 {
		dec = len(num) - i - 1
	}
	return f, dec, pct, true, false
}
