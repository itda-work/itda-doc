package mdoc

import (
	"strings"
	"testing"
)

// YouTube 블록: 받는 주소 꼴만 ID·시작 초를 뽑고, 그 밖은 보통 그림 규칙 그대로다.
func TestParseYouTube(t *testing.T) {
	const id = "9Luu4itC-Zs"
	ok := []struct {
		dest  string
		start int
	}{
		{"https://www.youtube.com/watch?v=" + id, 0},
		{"https://youtube.com/watch?v=" + id + "&list=PL1&index=2", 0},
		{"http://m.youtube.com/watch?v=" + id, 0},
		{"https://WWW.YouTube.com/watch?v=" + id, 0},
		{"https://youtu.be/" + id, 0},
		{"https://youtu.be/" + id + "?si=abc&t=90", 90},
		{"https://www.youtube.com/shorts/" + id, 0},
		{"https://www.youtube.com/embed/" + id + "?start=75", 75},
		{"https://www.youtube.com/watch?v=" + id + "&t=1m30s", 90},
		{"https://www.youtube.com/watch?v=" + id + "&t=1h2m3s", 3723},
		{"https://www.youtube.com/watch?v=" + id + "&t=45s", 45},
		{"https://www.youtube.com/watch?v=" + id + "&t=abc", 0},     // 시작 시각만 버린다
		{"https://www.youtube.com/watch?v=" + id + "&t=999999", 0},  // 하루 넘음
		{"https://www.youtube.com/watch?v=" + id + "&amp;t=10", 10}, // 엔티티는 푼다(schemeOf 와 같게)
		{"https://www.youtube.com/watch/?v=" + id, 0},               // 끝 / 는 같은 경로
	}
	for _, c := range ok {
		v, got := parseYouTube(c.dest)
		if !got || v.id != id || v.start != c.start {
			t.Errorf("%s: %+v %v (기대 start %d)", c.dest, v, got, c.start)
		}
	}
	bad := []string{
		"https://youtube.com.evil.example/watch?v=" + id,
		"https://notyoutube.com/watch?v=" + id,
		"https://evil.example/youtube.com/watch?v=" + id,
		"https://youtube.com@evil.example/watch?v=" + id,
		"https://user@www.youtube.com/watch?v=" + id,
		"https://www.youtube.com:8443/watch?v=" + id,
		"javascript:alert(1)//youtube.com/watch?v=" + id,
		"javascript://www.youtube.com/watch?v=" + id,
		"ftp://www.youtube.com/watch?v=" + id,
		"//www.youtube.com/watch?v=" + id,
		"https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/watch?v=" + id + "x",
		"https://www.youtube.com/watch?v=9Luu4itC%22Zs",
		"https://www.youtube.com/watch?v=9Luu4itC<Zs",
		"https://www.youtube.com/watch",
		"https://www.youtube.com/channel/" + id,
		"https://www.youtube.com/shorts/" + id + "/x",
		"https://youtu.be/",
		"https://youtu.be/" + id + "/more",
		"https://www.youtube-nocookie.com/embed/" + id,
		"https://i.ytimg.com/vi/" + id + "/hqdefault.jpg",
		"youtube.com/watch?v=" + id,
	}
	for _, d := range bad {
		if v, got := parseYouTube(d); got {
			t.Errorf("%s: YouTube 로 읽음 %+v", d, v)
		}
	}
}

// YouTube 블록의 HTML: 링크(class yt, 정규화한 watch 주소, data-yt-id·data-yt-start) 안에 대표 그림. 배치 class·data-split·width 는
// 보통 그림과 같은 자리(img)에 같은 값으로 나온다.
func TestYouTubeBlock(t *testing.T) {
	const u = "https://www.youtube.com/watch?v=9Luu4itC-Zs"
	const open = `<a class="yt" href="https://www.youtube.com/watch?v=9Luu4itC-Zs" data-yt-id="9Luu4itC-Zs">`
	const openPH = `<a class="yt yt-ph" href="https://www.youtube.com/watch?v=9Luu4itC-Zs" data-yt-id="9Luu4itC-Zs">`
	const thumb = `https://i.ytimg.com/vi/9Luu4itC-Zs/hqdefault.jpg`
	remote := Options{AllowRemoteImages: true}
	cases := []struct {
		name, src string
		opts      Options
		want      []string
		not       []string
		codes     []string // 경고 코드(차례대로)
	}{
		{"원격 끔 — 자리표시", "![](" + u + ")\n", Options{},
			[]string{"<figure data-line=\"1\">\n" + openPH + `<img data-remote-src="` + thumb + `" alt=""></a>`}, []string{" src="}, []string{WarnRemoteImage}},
		{"원격 켬 — i.ytimg.com", "![영상](" + u + ")\n", remote,
			[]string{open + `<img src="` + thumb + `" alt="영상"></a>`}, []string{"yt-ph"}, nil},
		{"right split", "![](" + u + "){.right split=1:2}\n", remote,
			[]string{open + `<img src="` + thumb + `" alt="" class="img-right" data-split="1:2"></a>`}, nil, nil},
		{"left fill", "![](" + u + "){.left .fill}\n", remote,
			[]string{`alt="" class="img-left img-fill"></a>`}, nil, nil},
		{"fill 만은 버림", "![](" + u + "){.fill}\n", remote,
			[]string{open + `<img src="` + thumb + `" alt=""></a>`}, []string{"img-fill"}, []string{WarnBadAttribute}},
		{"fit", "![](" + u + "){.fit}\n", remote, []string{`alt="" class="img-fit"></a>`}, nil, nil},
		{"width", "![](" + u + "){width=50%}\n", remote, []string{`alt="" style="width:50%"></a>`}, nil, nil},
		// 자리표시는 링크가 상자를 그리므로 width 를 링크에도(배치 class 가 없을 때만)
		{"자리표시 width", "![](" + u + "){width=50%}\n", Options{},
			[]string{`<a class="yt yt-ph" href="https://www.youtube.com/watch?v=9Luu4itC-Zs" data-yt-id="9Luu4itC-Zs" style="width:50%"><img data-remote-src="` + thumb + `" alt="" style="width:50%"></a>`}, nil, []string{WarnRemoteImage}},
		{"자리표시 fit width 는 링크에 없음", "![](" + u + "){.fit width=50%}\n", Options{},
			[]string{openPH + `<img data-remote-src="` + thumb + `" alt="" class="img-fit" style="width:50%"></a>`}, nil, []string{WarnRemoteImage}},
		{"Deckset right fit", "![right fit](" + u + ")\n", remote, []string{`alt="" class="img-right"></a>`}, []string{"img-fill"}, nil},
		{"Deckset left", "![left](" + u + ")\n", remote, []string{`alt="" class="img-left img-fill"></a>`}, nil, nil},
		{"poster — 원격 끔에도 로컬 그림, 경고 없음", "![](" + u + "){poster=./assets/x.jpg .right}\n", Options{LocalImagePrefix: "/_doc/t1/"},
			[]string{open + `<img src="/_doc/t1/./assets/x.jpg" alt="" class="img-right"></a>`}, []string{"yt-ph", "ytimg"}, nil},
		{"poster data:", "![](" + u + "){poster=\"data:image/png;base64,AAAA\"}\n", Options{LocalImagePrefix: "/_doc/"},
			[]string{`<img src="data:image/png;base64,AAAA" alt=""></a>`}, nil, nil},
		{"poster 원격은 버림", "![](" + u + "){poster=https://x.example/p.jpg}\n", Options{},
			[]string{openPH, `data-remote-src="` + thumb + `"`}, []string{"x.example"}, []string{WarnBadAttribute, WarnRemoteImage}},
		{"poster javascript: 는 버림", "![](" + u + "){poster=javascript:alert(1)}\n", remote,
			[]string{open + `<img src="` + thumb + `"`}, []string{"javascript"}, []string{WarnBadAttribute}},
		{"시작 초", "![](https://youtu.be/9Luu4itC-Zs?t=1m30s&si=x)\n", remote,
			[]string{`<a class="yt" href="https://www.youtube.com/watch?v=9Luu4itC-Zs&amp;t=90s" data-yt-id="9Luu4itC-Zs" data-yt-start="90">`}, []string{"si=x"}, nil},
		{"shorts", "![](https://youtube.com/shorts/9Luu4itC-Zs)\n", remote, []string{open}, nil, nil},
		{"글 사이 그림", "앞 글 ![](" + u + ") 뒤 글\n", remote, []string{"앞 글 " + open}, []string{"<figure>"}, nil},
		{"캡션", "![](" + u + " \"설명\")\n", remote, []string{open, "<figcaption>설명</figcaption>"}, nil, nil},
		// 링크 안의 YouTube 주소 그림은 사용자 링크가 이긴다 — 보통 원격 그림
		{"링크 안", "[![](" + u + ")](https://example.com)\n", Options{},
			[]string{`<a href="https://example.com"><img data-remote-src="` + u + `" alt=""></a>`}, []string{"yt", "data-yt"}, []string{WarnRemoteImage}},
		// 다른 호스트·틀린 ID 는 보통 원격 그림(또는 막은 주소)
		{"다른 호스트", "![](https://youtube.com.evil.example/watch?v=9Luu4itC-Zs)\n", Options{},
			[]string{`<img data-remote-src="https://youtube.com.evil.example/watch?v=9Luu4itC-Zs" alt="">`}, []string{"data-yt", "<a "}, []string{WarnRemoteImage}},
		{"notyoutube", "![](https://notyoutube.com/watch?v=9Luu4itC-Zs)\n", remote,
			[]string{`<img src="https://notyoutube.com/watch?v=9Luu4itC-Zs" alt="">`}, []string{"data-yt", "<a "}, nil},
		{"틀린 ID", "![](https://www.youtube.com/watch?v=abc)\n", Options{},
			[]string{`<img data-remote-src="https://www.youtube.com/watch?v=abc" alt="">`}, []string{"data-yt"}, []string{WarnRemoteImage}},
		{"javascript:", "![](javascript://www.youtube.com/watch?v=9Luu4itC-Zs)\n", remote,
			[]string{`<img alt="">`}, []string{"data-yt", "javascript"}, []string{WarnUnsafeURL}},
		{"보통 그림의 poster 는 버림", "![](a.png){poster=b.png}\n", Options{},
			[]string{`<img src="a.png" alt="">`}, []string{"b.png"}, []string{WarnBadAttribute}},
	}
	for _, c := range cases {
		r := render(t, c.src, c.opts)
		if !c.opts.AllowRemoteImages {
			assertSafe(t, r.HTML) // 원격 그림을 켠 결과는 원격 src 가 뜻한 것이다
		}
		for _, w := range c.want {
			if !strings.Contains(r.HTML, w) {
				t.Errorf("%s: %q 없음\n%s", c.name, w, r.HTML)
			}
		}
		for _, w := range c.not {
			if strings.Contains(r.HTML, w) {
				t.Errorf("%s: %q 있음\n%s", c.name, w, r.HTML)
			}
		}
		var codes []string
		for _, w := range r.Warnings {
			codes = append(codes, w.Code)
		}
		if strings.Join(codes, ",") != strings.Join(c.codes, ",") {
			t.Errorf("%s: 경고 %v, 기대 %v: %+v", c.name, codes, c.codes, r.Warnings)
		}
		for _, bad := range []string{"<script", "<iframe", " on"} {
			if strings.Contains(strings.ToLower(r.HTML), bad) {
				t.Errorf("%s: 결과에 %q\n%s", c.name, bad, r.HTML)
			}
		}
	}
}

// YouTube 블록도 분할 그림 패널(itda-hyve#550) 후보다 — 판정은 AST(그림 문단)라 보통 그림과 같다.
func TestYouTubePanel(t *testing.T) {
	src := "## 가\n\n![](https://youtu.be/9Luu4itC-Zs){.left split=2:1}\n\n- 본문\n\n## 나\n\n" +
		"![](https://youtu.be/9Luu4itC-Zs){.right}\n\n![](https://youtu.be/9Luu4itC-Zs){.left}\n"
	r := render(t, src, Options{AllowRemoteImages: true})
	if n := strings.Count(r.HTML, "data-slide-panel"); n != 2 {
		t.Errorf("패널 %d개, 기대 2\n%s", n, r.HTML)
	}
	if !strings.Contains(r.HTML, `<figure data-slide-panel="1"`) || !strings.Contains(r.HTML, `class="img-left" data-split="2:1"></a>`) {
		t.Errorf("패널 블록\n%s", r.HTML)
	}
	// 둘째 슬라이드의 두 번째 분할 그림은 제자리 .fit
	if !strings.Contains(r.HTML, `class="img-fit"></a>`) || len(r.Warnings) != 1 {
		t.Errorf(".fit 으로 바뀐 것: %+v\n%s", r.Warnings, r.HTML)
	}
}

// YouTube 블록 × 슬라이드 배경·가로 배치(itda-hyve#569): YouTube 블록은 배경(.full·.full .fill·Deckset ![fit])이 되지 않고(경고 + 제자리 .fit),
// 가로 배치(fit-row)에도 들지 않는다(.fit 그림 2장 이상인 문단에 섞이면 경고 + 각 그림 제자리 .fit). 분할 패널과 배경은 한 슬라이드에
// 그림 자리 하나라는 itda-hyve#569 규칙을 YouTube 블록도 따른다.
func TestYouTubeSlideBG(t *testing.T) {
	const u = "https://youtu.be/9Luu4itC-Zs"
	cases := []struct {
		name, src string
		want      []string
		not       []string
		codes     []string
	}{
		{"full → 제자리 fit", "## 가\n\n![](" + u + "){.full}\n\n- 본문\n",
			[]string{"<figure data-line=\"3\">\n<a class=\"yt\"", `alt="" class="img-fit"></a>`}, []string{"data-slide-bg", "img-full"}, []string{WarnBadAttribute}},
		{"full fill → 제자리 fit(fill 도 버림)", "## 가\n\n![](" + u + "){.full .fill}\n",
			[]string{`alt="" class="img-fit"></a>`}, []string{"data-slide-bg", "img-full", "img-fill"}, []string{WarnBadAttribute}},
		{"Deckset fit → 제자리 fit", "## 가\n\n![fit](" + u + ")\n",
			[]string{`alt="" class="img-fit"></a>`}, []string{"data-slide-bg", "img-full"}, []string{WarnBadAttribute}},
		{"가로 배치에 섞임 → 줄 아님", "## 가\n\n![](a.png){.fit} ![](" + u + "){.fit}\n",
			[]string{`<img src="a.png" alt="" class="img-fit">`, `alt="" class="img-fit"></a>`}, []string{"fit-row"}, []string{WarnBadAttribute}},
		{"YouTube 둘만 → 줄 아님", "## 가\n\n![](" + u + "){.fit}\n![](" + u + "){.fit}\n",
			[]string{`alt="" class="img-fit"></a>`}, []string{"fit-row"}, []string{WarnBadAttribute}},
		{"보통 그림 둘은 그대로 줄", "## 가\n\n![](a.png){.fit} ![](b.png){.fit}\n\n![](" + u + "){.fit}\n",
			[]string{`<p class="fit-row"`, `alt="" class="img-fit"></a>`}, nil, nil},
		{"배경이 먼저면 YouTube 분할은 제자리 fit", "## 가\n\n![](bg.png){.full}\n\n![](" + u + "){.right}\n",
			[]string{`data-slide-bg`, `alt="" class="img-fit"></a>`}, []string{"data-slide-panel", "img-right"}, []string{WarnBadAttribute}},
		{"YouTube 분할이 먼저면 배경은 제자리 fit", "## 가\n\n![](" + u + "){.right}\n\n![](bg.png){.full}\n",
			[]string{`data-slide-panel`, `class="img-right"></a>`, `<img src="bg.png" alt="" class="img-fit">`}, []string{"data-slide-bg"}, []string{WarnBadAttribute}},
	}
	for _, c := range cases {
		r := render(t, c.src, Options{AllowRemoteImages: true})
		for _, w := range c.want {
			if !strings.Contains(r.HTML, w) {
				t.Errorf("%s: %q 없음\n%s", c.name, w, r.HTML)
			}
		}
		for _, w := range c.not {
			if strings.Contains(r.HTML, w) {
				t.Errorf("%s: %q 있음\n%s", c.name, w, r.HTML)
			}
		}
		var codes []string
		for _, w := range r.Warnings {
			codes = append(codes, w.Code)
		}
		if strings.Join(codes, ",") != strings.Join(c.codes, ",") {
			t.Errorf("%s: 경고 %v, 기대 %v: %+v", c.name, codes, c.codes, r.Warnings)
		}
	}
}
