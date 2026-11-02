package mdoc

// frontmatter 키의 스키마 표(#474 ①). 파서(meta.go fill·parseSlides·parseCode)가 받는 키·별칭의
// 정본을 한 곳에 둔다 — 시험(schema_test.go)이 이 표와 파서의 case 를 양방향으로 맞춘다(표에 없는 case·
// case 에 없는 표 항목·사라진 별칭 모두 실패). 편집 화면(#474 ②)과 MCP 도구가 같은 표를 쓴다.
//
// 키 변경 규칙(https://github.com/itda-work/itda-hyve/blob/main/docs/DESIGN-markdown-docs.md 1.2): 추가는 자유(옛 판은 모르는 키로 둔다). 이름을 바꾸면
// 옛 이름을 Aliases 에 넣어 계속 받는다. 폐기하는 키는 읽되 경고를 낸다. 같은 키의 뜻은 바꾸지 않는다 — 새 키를 만든다.

// MetaKey 는 frontmatter 키 하나의 스키마다. 값 문장은 모두 한국어다.
type MetaKey struct {
	// Key 는 정본 키 이름이다(kebab-case).
	Key string `json:"key"`
	// Group 은 묶음 키다: ""(최상위) · "slides" · "code".
	Group string `json:"group,omitempty"`
	// Kind 는 값의 종류다(예: "글자", "참·거짓", "정수", "매핑 또는 참·거짓").
	Kind string `json:"kind"`
	// Range 는 값의 범위·후보다(예: "1~6", "a4·a4-landscape"). 종류가 범위를 다 말하면 비었다.
	Range string `json:"range,omitempty"`
	// Default 는 정하지 않았을 때의 값다("없음" 포함).
	Default string `json:"default,omitempty"`
	// Desc 는 이 키가 무엇을 정하는지 한 줄이다.
	Desc string `json:"desc"`
	// Aliases 는 옛 이름·다른 표기다. 파서는 이 이름들도 받는다.
	Aliases []string `json:"aliases,omitempty"`

	// 아래는 편집 화면(#474 ②)이 컨트롤을 고르는 기계용 필드다 — 사람용 Kind·Range 와 함께 둔다.
	// Input 이 빈 키는 화면에 줄이 없다: author(글자 목록·`- name: …` 매핑이라 한 줄 컨트롤이 맞지
	// 않음), slides·code 최상위(하위 키로만 다룬다).
	Input string `json:"input,omitempty"` // text · bool · int · number · enum · theme · list-style
	// Values 는 enum·list-style 의 후보다(파서가 받는 정규 표기). theme 은 화면이 테마 목록으로 채운다.
	Values []string `json:"values,omitempty"`
	// Min·Max 는 int·number 의 범위다(0 은 제한 없음 — 현재 키는 모두 양수 범위다).
	Min float64 `json:"min,omitempty"`
	Max float64 `json:"max,omitempty"`
	// Step 은 number 칸의 조절 단위다(0 이면 기본).
	Step float64 `json:"step,omitempty"`
	// PerDoc 는 문서마다 다른 값(제목·부제·날짜·문서 번호·판)이다 — 화면이 「문서마다 다른 값」 묶음에 접어 둔다.
	PerDoc bool `json:"per_doc,omitempty"`
}

// FrontmatterSchema 는 아는 frontmatter 키 전부다(최상위 → slides 하위 → code 하위 순서).
var FrontmatterSchema = []MetaKey{
	{Key: "title", Kind: "글자", Default: "없음", Desc: "문서 제목(표지·머리글)", Input: "text", PerDoc: true},
	{Key: "subtitle", Kind: "글자", Default: "없음", Desc: "부제", Input: "text", PerDoc: true},
	{Key: "author", Kind: "글자 또는 글자 목록(`- name: …` 꼴도)", Default: "없음", Desc: "쓴 사람"},
	{Key: "date", Kind: "글자", Default: "없음", Desc: "날짜 — 적은 글자 그대로 보인다", Input: "text", PerDoc: true},
	{Key: "lang", Kind: "글자", Default: "없음(한국어)", Desc: "알림 제목의 언어(ko·en …)", Input: "text"},
	{Key: "toc", Kind: "참·거짓", Default: "false", Desc: "목차를 넣는다", Input: "bool"},
	{Key: "toc-depth", Kind: "정수", Range: "1~6", Default: "3", Desc: "목차에 넣는 제목 단계", Input: "int", Min: 1, Max: 6},
	{Key: "number-sections", Kind: "참·거짓", Default: "false", Desc: "제목 번호 1. / 1.1 / 1.1.1", Input: "bool"},
	{Key: "papersize", Kind: "열거", Range: "a4·a4-landscape", Default: "a4", Desc: "종이 크기",
		Input: "enum", Values: []string{"a4", "a4-landscape"}},
	{Key: "theme", Kind: "테마 이름 또는 별칭", Default: "classic", Desc: "테마", Input: "theme"},
	{Key: "cover", Kind: "참·거짓", Default: "테마가 정함", Desc: "표지 쪽", Input: "bool"},
	{Key: "doc-no", Kind: "글자", Default: "없음", Desc: "문서 번호(표지·머리글)", Input: "text", PerDoc: true},
	{Key: "version", Kind: "글자(따옴표 권장)", Default: "없음", Desc: "판", Input: "text", PerDoc: true},
	{Key: "classification", Kind: "글자", Default: "없음", Desc: "보안 등급(예: 대외비)", Input: "text"},
	{Key: "organization", Kind: "글자", Default: "테마가 정함", Desc: "조직 이름", Input: "text"},
	{Key: "list-style", Kind: "열거 또는 목록", Range: "gaejosik·gongmun·plain·[gaejosik, gongmun]", Default: "테마가 정함", Desc: "목록 기호 사다리",
		Input: "list-style", Values: []string{ListGaejosik, ListGongmun, ListGaejosik + " " + ListGongmun, ListPlain}},
	{Key: "slides", Kind: "매핑 또는 참·거짓", Default: "없음", Desc: "발표 자료 — 슬라이드로 여는지는 문서의 이 키만 정한다"},
	{Key: "code", Kind: "매핑 또는 참·거짓", Default: "줄 번호 켬", Desc: "코드 블록 표시"},

	{Key: "ratio", Group: "slides", Kind: "열거", Range: `"16:9"·"4:3"(16/9 꼴도 받음)`, Default: "16:9",
		Desc: "화면 비율", Aliases: []string{"aspect", "aspectratio"},
		Input: "enum", Values: []string{"16:9", "4:3"}},
	{Key: "level", Group: "slides", Kind: "정수", Range: "1~6", Default: "문서 구조에서 정함",
		Desc: "슬라이드를 여는 제목 단계", Aliases: []string{"slide-level"}, Input: "int", Min: 1, Max: 6},
	{Key: "transition", Group: "slides", Kind: "열거", Range: "none·fade·slide", Default: "none",
		Desc: "장면 전환", Input: "enum", Values: []string{"none", "fade", "slide"}},
	{Key: "incremental", Group: "slides", Kind: "참·거짓", Default: "false",
		Desc: "모든 목록을 차례로 나타낸다", Input: "bool"},
	{Key: "font-scale", Group: "slides", Kind: "숫자", Range: "0.6~1.5", Default: "0.9",
		Desc: "슬라이드 글자 배율", Aliases: []string{"fontscale", "font_scale"},
		Input: "number", Min: 0.6, Max: 1.5, Step: 0.05},

	{Key: "line-numbers", Group: "code", Kind: "참·거짓", Default: "true(켬)",
		Desc: "코드 블록 줄 번호", Aliases: []string{"linenumbers", "line_numbers"}, Input: "bool"},
}

// SchemaKeys 는 그룹의 키·별칭 전부다(정렬 아님 — 표 순서).
func schemaNames(group string) []string {
	var out []string
	for _, k := range FrontmatterSchema {
		if k.Group != group {
			continue
		}
		out = append(out, k.Key)
		out = append(out, k.Aliases...)
	}
	return out
}

// metaAlias 는 그룹별 별칭 → 정규 키다(slides·code 하위 — 최상위 키에는 별칭이 없다). 폴더 상속의
// 합치기·출처 등록은 이 표로 키를 정규 이름으로 바꾸고 본다(#474 리뷰 R1 — 별칭끼리 다른 키로
// 어긋나 폴더 값이 문서를 이기는 일이 없게).
var metaAlias = func() map[string]string {
	out := map[string]string{}
	for _, k := range FrontmatterSchema {
		for _, a := range k.Aliases {
			out[k.Group+"."+a] = k.Group + "." + k.Key
		}
	}
	return out
}()

// normalizeAlias 는 키를 묶음 경로(group "slides"|"code"|"") 의 정규 이름으로 바꾼다. 별칭이 아니면 그대로.
func normalizeAlias(group, key string) string {
	if canon, ok := metaAlias[group+"."+key]; ok {
		return canon[len(group)+1:]
	}
	return key
}
