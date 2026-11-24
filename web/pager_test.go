package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 쪽 나눔의 pre 줄 번호(itda-hyve#126 리뷰 R4): 코드 블록·mdoc 안전 모드의 원문 블록(.raw-source)을 거듭 나눠도 조각마다 data-line 이 그 조각의
// 첫 원문 줄이고(나눌 때마다 펜스 몫 1 이 쌓였다), 원문 블록은 조각에서도 줄마다 span[data-line] 이다. pager.js 의 splitPre 를 그대로 떼어
// node 로 돌린다(testdata/pager_split.cjs). 실제 레이아웃은 selftest 가 본다.
func TestPagerSplitPreLines(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("pager.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	a := strings.Index(src, "  function splitPre(")
	z := strings.Index(src[max(a, 0):], "\n  // ---- 문단")
	if a < 0 || z < 0 {
		t.Fatal("pager.js 에서 splitPre 를 찾지 못함")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" { // just test·CI
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	part := filepath.Join(t.TempDir(), "split.js")
	if err := os.WriteFile(part, []byte(src[a:a+z]), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_split.cjs"))
	out, err := exec.Command(node, runner, part).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`PAGER DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

func TestPagerPageBands(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 경계 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 경계 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_bands.cjs"))
	out, err := exec.Command(node, runner).CombinedOutput()
	if err != nil {
		t.Fatalf("쪽 경계 검사 실패: %v\n%s", err, out)
	}
}

// 쪽 넘김 키와 복사 단추(itda-hyve#413 리뷰 [2]): 단추(button — 코드 복사 단추 등)에 포커스가 있으면 Space 는 단추 누르기의 키다 —
// navKey 는 넘기지 않고, hostNavKey(문서 창 보기 모드)도 단추 위에서는 넘기지 않는다(HOST_BUSY 제외). pager.js 를 그대로
// node 로 돌린다(testdata/pager_nav.cjs). 공유 보기 화면의 keyHandler 는 share_client_test.go(share_conn.cjs)가 본다.
func TestPagerNavKeys(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	src, _ := filepath.Abs(filepath.Join("pager.js"))
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_nav.cjs"))
	out, err := exec.Command(node, runner, src).CombinedOutput()
	if err != nil {
		t.Fatalf("네김 키 검사 실패: %v\n%s", err, out)
	}
	if m := regexp.MustCompile(`PAGERNAV DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out)); m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

// 번호 목록 쪽 나눔(itda-hyve#584): 쪽을 넘어 나뉜 OL 의 뒷조각은 앞 조각에 놓인 항목 수만큼 start 를 이어 간다 — start="0" 도
// 0 으로 읽는다(예전 판정 `|| 1` 은 0 을 1 로 읽어 뒷조각 번호가 하나씩 밀렸다). 숫자가 아닌 값만 1 로 둔다. 반쯤 들어간
// 항목(li.cont)·start 없음·start="3"·음수도 함께 본다. pager.js 의 splitChildren 을 그대로 떼어 node 로 돌린다(testdata/pager_list.cjs).
func TestPagerSplitListStart(t *testing.T) {
	src := pagerSource(t)
	a := strings.Index(src, "  function splitChildren(")
	z := strings.Index(src[max(a, 0):], "\n  // ---- 표:")
	if a < 0 || z < 0 {
		t.Fatal("pager.js 에서 splitChildren 을 찾지 못함")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	part := filepath.Join(t.TempDir(), "list.js")
	if err := os.WriteFile(part, []byte(src[a:a+z]), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_list.cjs"))
	out, err := exec.Command(node, runner, part).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if m := regexp.MustCompile(`PAGERLIST DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out)); m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

// 한 쪽보다 높은 표 행(itda-hyve#795): paginate 옵션 splitTallRows(공유 PDF 의 노트·메모 쪽만 켠다)가 켜지면 splitTable 이 그 행을 칸의 글 단위로 나눠
// 앞 조각을 지금 쪽에, 뒤 조각(id 없음, .cont)을 다음 쪽 표의 첫 행으로 보낸다 — 칸 글을 이으면 원래 글이고(공백에서 끊음 — 공백 없는 글자만 그 자리),
// 이어지는 쪽마다 thead 가 있고, 잘린 것이 없다. 한 쪽에 들어가는 행만 있는 표는 행 단위 그대로(앞 행과 합쳐서만 한 쪽을 넘는 행도 나누지 않는다),
// rowspan 이 걸린 행·글 단위로 못 자르는 칸(큰 수식 하나·블록)은 잘린 채(overflow-clipped)다. 옵션이 없으면 높은 행을 나누지 않고 잘린 채 담는다
// (변경 전과 같다). pager.js 의 쪽 나눔 부분을 떼어 레이아웃을 흉내 낸 DOM 대역으로 node 에서 돌린다(testdata/pager_rowsplit.cjs).
// 실제 레이아웃은 itda-hyve 의 공유 PDF 시험이 본다.
func TestPagerSplitTallRow(t *testing.T) {
	src := pagerSource(t)
	a := strings.Index(src, "  const BLOCK_TAGS")
	z := strings.Index(src[max(a, 0):], "\n  // ---- .fit 그림")
	if a < 0 || z < 0 {
		t.Fatal("pager.js 에서 쪽 나눔 부분을 찾지 못함")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	part := filepath.Join(t.TempDir(), "rowsplit.js")
	if err := os.WriteFile(part, []byte(src[a:a+z]), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_rowsplit.cjs"))
	out, err := exec.Command(node, runner, part).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if m := regexp.MustCompile(`PAGERROW DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out)); m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

// pagerSource 는 pager.js 원문이다. ITDA_DOC_TEST_OVERLAY 가 가리키는 overlay JSON 의 Replace 에 pager.js 가 있으면 그 파일을 읽는다
// (뮤테이션 시험이 디스크의 원문 대신 바꿔 끼운 소스를 보게 — mdoc/guidemd_test.go 의 overlayReplace 와 같은 약속).
func pagerSource(t *testing.T) string {
	t.Helper()
	name := "pager.js"
	if p := os.Getenv("ITDA_DOC_TEST_OVERLAY"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ITDA_DOC_TEST_OVERLAY=%s 를 읽지 못함: %v", p, err)
		}
		var ov struct {
			Replace map[string]string `json:"Replace"`
		}
		if json.Unmarshal(b, &ov) != nil {
			t.Fatalf("ITDA_DOC_TEST_OVERLAY=%s 가 overlay JSON 이 아님", p)
		}
		abs, _ := filepath.Abs(name)
		if r, ok := ov.Replace[abs]; ok {
			name = r
		} else if r, ok := ov.Replace[name]; ok {
			name = r
		}
	}
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 이어 만들기 공통 부품(incr.js, itda-hyve#658): 서명은 위치 속성을 묶음 첫 값에 대한 상대값으로 넣는다(통째로 밀린 묶음은 같은 서명, 묶음 안
// 줄 간격·내용·종류가 바뀌면 다른 서명), 밀기는 줄·블록 번호를 차이만큼, 끼우기는 그대로 쓰는 노드를 옮기지 않는다, 비우기는 기억을 놓는다.
// 작은 가짜 DOM 으로 node 에서 돌린다(testdata/incr.cjs). 이어 만든 슬라이드 = 처음부터 만든 것은 레이아웃이 들어 itda-hyve selftest 가 본다.
func TestIncrParts(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "incr.cjs"))
	out, err := exec.Command(node, runner).CombinedOutput()
	if err != nil {
		t.Fatalf("이어 만들기 부품 검사 실패: %v\n%s", err, out)
	}
	if !regexp.MustCompile(`INCR DONE cases=\d+ failed=0\s*$`).Match(out) {
		t.Fatalf("끝나지 않음\n%s", out)
	}
}

// 넓은 수식 맞추기(itda-hyve#752): fitMath 의 동작은 testdata/pager_fitmath.cjs 가 pager.js 를 그대로 돌려 보고, 쪽 나눔이 줄어든 높이로 재는 차례
// (adoptWidths 가 인라인 style 을 지운 뒤, 나누기 전)와 굴림 상자가 쪽 상자 안에 남지 않는 CSS 를 글자로 본다. 실제 레이아웃은 selftest 가 본다.
func TestPagerFitMath(t *testing.T) {
	b, err := os.ReadFile("pager.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	a := strings.Index(src, "  function paginate(")
	if a < 0 {
		t.Fatal("pager.js 에서 paginate 를 찾지 못함")
	}
	p := src[a:]
	wi, fi, li := strings.Index(p, "adoptWidths(flow)"), strings.Index(p, "fitQueueMath(queue)"), strings.Index(p, "while (queue.length")
	if wi < 0 || fi < wi || li < fi {
		t.Errorf("paginate 의 차례가 adoptWidths → fitQueueMath → 나누기가 아님: %d %d %d", wi, fi, li)
	}
	dash, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatal(err)
	}
	if fm, ft := strings.Index(string(dash), "ItdaPager.fitMath(pg)"), strings.Index(string(dash), "if (fitTile(t))"); fm < 0 || ft < fm {
		t.Errorf("dashboard.js build 가 타일 넘침을 재기(fitTile) 전에 수식을 맞추지 않음: %d %d", fm, ft)
	}
	css, err := os.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), ".page:not(.page-flow) .math-display { overflow: hidden; overflow: clip; }") {
		t.Error("base.css 의 쪽 상자 안 .math-display 가 두 축 모두 clip 이 아님")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js, _ := filepath.Abs("pager.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_fitmath.cjs"))
	out, err := exec.Command(node, runner, js).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`PAGER DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

// 그림 테두리(itda-doc#97): 문장 속이 아닌 그림 — figure·링크 단 그림 줄(mdoc 의 p.img-link)·슬라이드 가로 배치·분할 패널(cover 인 img-fill 과
// YouTube 대표 그림 빼고) — 이 --img-border 를 두르는 것을 base.css 글자로 묶고, 자리를 채우는 그림의 상자를 그림 크기로 줄이는 fitImages 를
// testdata/pager_fitimages.cjs 가 pager.js 를 그대로 돌려 본다(세로·가로 긴 그림 × 패널 1:1·2:1·.fit). slides.js 가 장을 다 꾸민 뒤 부르는 것도 글자로 본다.
// 실제 레이아웃(flex·object-fit)과 테마별 모양은 화면 판정이 본다.
func TestImageBorders(t *testing.T) {
	css, err := os.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	const rule = ".page-body :where(figure > img, p.img-link > img, p.img-link > a > img, .page.slide .page-body > p.fit-row > img, .page.slide .page-body > p.fit-row > a > img),\n" +
		":where(.page.slide > .slide-panel) img:where(:not(.img-fill, a.yt *)) { border: var(--img-border); border-radius: var(--img-radius); }"
	if !strings.Contains(string(css), rule) {
		t.Errorf("base.css 의 그림 테두리 규칙이 figure·링크 단 그림·가로 배치·패널(img-fill·a.yt 제외)을 특이도 낮게 고르지 않음")
	}
	// 테두리를 두르는 다른 img 규칙이 없다 — 로고·배경·글 줄 속 그림은 위 선택자에만 걸리지 않으면 테두리가 없다
	if n := strings.Count(string(css), "border: var(--img-border)"); n != 1 {
		t.Errorf("base.css 에 --img-border 를 두르는 규칙이 %d개 — 하나여야 대상·제외 목록이 한곳에 있다", n)
	}
	sl, err := os.ReadFile("slides.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(sl)
	if d, f := strings.Index(s, "decorate(pg, o);\n"), strings.Index(s, "window.ItdaPager.fitImages(pg);"); d < 0 || f < d {
		t.Errorf("slides.js makeGroup 이 장을 꾸민(decorate) 뒤 fitImages 를 부르지 않음: %d %d", d, f)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js, _ := filepath.Abs("pager.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_fitimages.cjs"))
	out, err := exec.Command(node, runner, js).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`PAGER DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

// 그림 자리 ratio(itda-hyve#475·#1074): sizeDiagrams 가 mdoc 이 낸 data-diagram-ratio(정수·소수 W:H)를 mdoc 과 같은 규칙으로 다시 검사해
// svg 의 aspect-ratio·width 계산식에 비율을 그대로 거는지, 규칙 밖 값(수 하나 꼴·0·비율 밖·셋째 자리 소수)은 상자를 걸지 않는지
// testdata/pager_diagrams.cjs 가 pager.js 를 그대로 돌려 본다(adoptWidths 로 부른다). 값 규칙의 정본 검사는 mdoc TestDiagramSizeParse.
func TestPagerSizeDiagrams(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js, _ := filepath.Abs("pager.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_diagrams.cjs"))
	out, err := exec.Command(node, runner, js).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`PAGER DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}
