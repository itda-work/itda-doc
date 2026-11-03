package mdoc

import "strings"

// frontmatter 모르는 키의 오타 짐작(itda-doc#14). 모르는 키가 아는 키(정본·별칭)와 아주 가까우면 최상위는
// frontmatter-typo 경고를 새로 내고, slides.·code. 묶음 안은 기존 frontmatter-value 문구에 「X 의 오타?」를
// 덧붙인다 — 값은 지금처럼 Extra 에 남고(최상위) 렌더는 그대로다.
// 가까움은 오탐을 줄이는 쪽으로 좁게 잡는다:
//   - 비교 전 정규화: 소문자, `_`·`-` 제거. 정규화 뒤 같으면 가깝다(Toc_Depth → toc-depth).
//   - 정규화한 이름끼리 편집 거리(인접 전치 포함): 짧은 쪽 길이 3 이하면 0(정규화 일치만),
//     4~7 이면 1 이하, 8 이상이면 2 이하.
//   - 후보가 여럿이면 거리가 가장 작은 것, 같으면 스키마 표 순서의 첫 것. 제안은 정본 키 이름이다.
//   - ASCII 아닌 글자가 든 키(수신·참조 같은 테마 자유 키)는 비교하지 않는다.

// typoSuggest 는 묶음 group(""·"slides"·"code") 의 모르는 키 key 와 가까운 아는 키의 정본 이름을 돌려준다.
func typoSuggest(group, key string) (string, bool) {
	for i := 0; i < len(key); i++ {
		if key[i] >= 0x80 {
			return "", false
		}
	}
	nk := typoNorm(key)
	if nk == "" {
		return "", false
	}
	best, bestD := "", -1
	for _, name := range schemaNames(group) {
		nn := typoNorm(name)
		limit := typoLimit(min(len(nk), len(nn)))
		if abs(len(nk)-len(nn)) > limit { // 거리는 길이 차 이상 — 긴 키에 표를 만들지 않는다
			continue
		}
		d := osaDistance(nk, nn)
		if d > limit {
			continue
		}
		if bestD < 0 || d < bestD {
			best, bestD = normalizeAlias(group, name), d
		}
	}
	return best, bestD >= 0
}

// typoNorm 은 비교용 이름이다(소문자, `_`·`-` 제거).
func typoNorm(s string) string {
	s = strings.ToLower(s)
	return strings.NewReplacer("_", "", "-", "").Replace(s)
}

// typoLimit 은 짧은 쪽 길이 n 에서 받는 최대 편집 거리다.
func typoLimit(n int) int {
	switch {
	case n <= 3:
		return 0
	case n <= 7:
		return 1
	default:
		return 2
	}
}

// osaDistance 는 두 ASCII 글자열의 편집 거리다(삽입·삭제·바꿈·인접 전치 각 1 — optimal string alignment).
func osaDistance(a, b string) int {
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(a)][len(b)]
}

// typoHint 는 묶음 안 모르는 키 경고 문구에 넣는 덧말이다(가까운 키가 없으면 빈 글자).
func typoHint(group, key string) string {
	if s, ok := typoSuggest(group, key); ok {
		return " — " + s + " 의 오타?"
	}
	return ""
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
