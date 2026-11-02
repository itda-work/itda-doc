package mdoc

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// attrTokenCase 는 testdata/attr-tokens.json 한 줄이다 — 속성 중괄호 안 글을 커서까지 자른 입력과, 그 자리가 어느
// 토큰의 어디인지(kind). 같은 표를 문서 창 편집기의 자동 완성 판정(dist/edit-util.js attrTail)이 읽는다
// (Itda Hyve 의 문서 창(docview) TestAttrTokensMatchEditor). 여기서는 그 판정이 parseAttrs 의 토큰 규칙과 맞는지 본다(#434).
type attrTokenCase struct {
	In   string `json:"in"`
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Text string `json:"text"`
	Q    string `json:"q"`
}

func lastData(a attrs) (string, string, bool) {
	if len(a.Data) == 0 {
		return "", "", false
	}
	kv := a.Data[len(a.Data)-1]
	return kv[0], kv[1], true
}

// TestAttrTokenTable 은 표의 kind 마다 parseAttrs 로 그 자리를 확인한다 — 입력에 글자를 이어 붙였을 때 렌더러가
// 그 글자를 어느 토큰에 넣는지로 본다(이름을 치는 중이면 "=1" 을 붙여 그 키가 되고, 값 안이면 이은 글자가 값이 된다 등).
func TestAttrTokenTable(t *testing.T) {
	raw, err := os.ReadFile("testdata/attr-tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []attrTokenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 40 {
		t.Fatalf("표가 너무 짧다: %d", len(cases))
	}
	for _, c := range cases {
		fail := func(why string, args ...any) { t.Errorf("%q (%s): "+why, append([]any{c.In, c.Kind}, args...)...) }
		switch c.Kind {
		case "boundary": // 토큰 사이 — 지금까지 읽히고, 새 key=value 가 따로 붙는다
			a, err := parseAttrs(c.In)
			b, err2 := parseAttrs(c.In + "zz=1")
			k, v, _ := lastData(b)
			if err != nil || err2 != nil || len(b.Data) != len(a.Data)+1 || k != "zz" || v != "1" {
				fail("토큰 사이가 아니다: %v / %v %v", err, err2, b.Data)
			}
		case "name": // 키를 치는 중 — "=1" 을 붙이면 그 키(소문자)
			b, err := parseAttrs(c.In + "=1")
			k, v, _ := lastData(b)
			if err != nil || k != strings.ToLower(c.Text) || v != "1" || !strings.HasSuffix(c.In, c.Text) {
				fail("키 자리가 아니다: %v %v", err, b.Data)
			}
		case "value": // 따옴표 없는 값 안 — 이은 글자도 그 값
			a, err := parseAttrs(c.In)
			k, v, _ := lastData(a)
			b, err2 := parseAttrs(c.In + "z")
			k2, v2, _ := lastData(b)
			if err != nil || err2 != nil || k != strings.ToLower(c.Key) || v != c.Text || k2 != k || v2 != c.Text+"z" {
				fail("값 자리가 아니다: %v %v / %v %v", err, a.Data, err2, b.Data)
			}
		case "qvalue": // 닫히지 않은 따옴표 값 — 지금은 못 읽고, 따옴표를 닫으면 그 값
			_, err := parseAttrs(c.In)
			b, err2 := parseAttrs(c.In + c.Q)
			k, v, _ := lastData(b)
			if err == nil || err2 != nil || k != strings.ToLower(c.Key) || v != c.Text {
				fail("열린 따옴표 값이 아니다: %v / %v %v", err, err2, b.Data)
			}
		case "closed": // 따옴표 값을 막 닫음 — 붙여 쓴 글자는 못 읽는다
			_, err := parseAttrs(c.In)
			_, err2 := parseAttrs(c.In + "x")
			if err != nil || err2 == nil {
				fail("닫힌 따옴표 뒤가 아니다: %v / %v", err, err2)
			}
		case "id":
			b, err := parseAttrs(c.In + "x")
			if err != nil || b.ID != c.Text+"x" {
				fail("id 안이 아니다: %v %q", err, b.ID)
			}
		case "class":
			b, err := parseAttrs(c.In + "x")
			if err != nil || len(b.Classes) == 0 || b.Classes[len(b.Classes)-1] != c.Text+"x" {
				fail("class 안이 아니다: %v %v", err, b.Classes)
			}
		case "flag":
			a, err := parseAttrs(c.In)
			_, err2 := parseAttrs(c.In + "x")
			if err != nil || !a.Unnumbered || err2 == nil {
				fail("홀로 선 - 가 아니다: %v / %v", err, err2)
			}
		case "invalid": // 이미 못 읽는다 — 키·값·따옴표를 이어도 살아나지 않는다
			for _, tail := range []string{"", "=1", `"`, "'", "x"} {
				if _, err := parseAttrs(c.In + tail); err == nil {
					fail("%q 를 이으면 읽힌다", tail)
				}
			}
		default:
			fail("모르는 kind")
		}
	}
}
