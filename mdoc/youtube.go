package mdoc

import (
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// YouTube 블록 — 그림 문법의 목적지가 YouTube 동영상 주소이면 「대표 그림 + ▶」 블록이 된다.
// mdoc 결과는 링크(<a class="yt">)와 그 안의 대표 그림(<img>)뿐이다 — 스크립트·이벤트 속성·iframe 은 내지 않는다.
// 누르면 그 자리에서 플레이어로 바꾸는 것은 화면 JS(web/share.js 가 찾는 호스트 부품)가 하고, 부품이 없는 화면·PDF 에서는 그림과 링크로만 남는다.

// ytIDRe 는 YouTube 동영상 ID(11자)다.
var ytIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ytTimeRe 는 시작 시각 t=·start= 의 꼴이다: 초(90, 90s) 또는 1h2m3s 꼴.
var ytTimeRe = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s?)?$`)

// ytHosts 는 받는 YouTube 호스트다(소문자, 포트 없음). youtu.be 는 경로가 곧 ID 다.
var ytHosts = map[string]bool{"youtube.com": true, "www.youtube.com": true, "m.youtube.com": true, "youtu.be": true}

// ytVideo 는 YouTube 블록 하나의 값이다.
type ytVideo struct {
	id     string
	start  int    // 시작 초(0 이면 없음)
	poster string // poster= 로 받은 로컬 대표 그림(classifyImage 가 urlOK) — 없으면 ""
}

// watchURL 은 링크로 내는 정규화한 주소다(ID 와 시작 초만 — 원래 주소의 다른 매개변수는 버린다).
func (v *ytVideo) watchURL() string {
	u := "https://www.youtube.com/watch?v=" + v.id
	if v.start > 0 {
		u += "&t=" + strconv.Itoa(v.start) + "s"
	}
	return u
}

// thumbURL 은 YouTube 가 주는 대표 그림 주소다(원격 그림 — 원격 그림을 켠 문서에서만 불러온다).
func (v *ytVideo) thumbURL() string { return "https://i.ytimg.com/vi/" + v.id + "/hqdefault.jpg" }

// parseYouTube 는 이미지 목적지가 YouTube 동영상 주소이면 그 값을 돌려준다. 받는 꼴:
// http(s)://(www.|m.)youtube.com/watch?v=ID, /shorts/ID, /embed/ID, http(s)://youtu.be/ID — 시작 시각은 t= 또는 start=(초, 90s, 1m30s).
// 사용자 정보(user@)·포트가 있는 주소, 받는 호스트가 아닌 주소(youtube.com.evil.example·notyoutube.com), ID 가 11자 [A-Za-z0-9_-] 가
// 아닌 주소는 YouTube 블록이 아니다(보통 원격 그림·막은 주소 규칙 그대로). 시작 시각을 읽지 못하면 시작 시각만 버린다.
func parseYouTube(dest string) (*ytVideo, bool) {
	s := strings.TrimSpace(html.UnescapeString(inlineValue([]byte(dest))))
	u, err := url.Parse(s)
	if err != nil || u.User != nil || u.Port() != "" || u.Opaque != "" {
		return nil, false
	}
	if sc := strings.ToLower(u.Scheme); sc != "http" && sc != "https" {
		return nil, false
	}
	host := strings.ToLower(u.Hostname())
	if !ytHosts[host] {
		return nil, false
	}
	var id string
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case host == "youtu.be":
		if len(parts) == 1 {
			id = parts[0]
		}
	case len(parts) == 1 && parts[0] == "watch":
		id = u.Query().Get("v")
	case len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed"):
		id = parts[1]
	}
	if !ytIDRe.MatchString(id) {
		return nil, false
	}
	v := &ytVideo{id: id}
	q := u.Query()
	t := q.Get("t")
	if t == "" {
		t = q.Get("start")
	}
	v.start = ytSeconds(t)
	return v, true
}

// ytSeconds 는 시작 시각 글자를 초로 읽는다. 비었거나 꼴이 틀리면 0 이다. 하루(86400초)를 넘으면 0 이다.
func ytSeconds(t string) int {
	m := ytTimeRe.FindStringSubmatch(strings.ToLower(t))
	if t == "" || m == nil {
		return 0
	}
	n := 0
	for i, mul := range []int{3600, 60, 1} {
		if m[i+1] == "" {
			continue
		}
		x, err := strconv.Atoi(m[i+1])
		if err != nil || x > 86400 {
			return 0
		}
		n += x * mul
	}
	if n > 86400 {
		return 0
	}
	return n
}
