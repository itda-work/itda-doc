package page

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/itda-work/itda-doc/mdoc"
)

// 폴더 기본 frontmatter `_metadata.yml` 찾기·읽기(itda-hyve#474). 경로를 아는 쪽(호스트 앱)이 파일을 찾아 읽어
// mdoc.Options.FolderMeta 로 넘긴다 — mdoc.Render 는 경로를 모르는 순수 함수다.

const (
	// FolderMetaName 은 폴더 기본 frontmatter 파일 이름이다(Quarto 관례).
	FolderMetaName = "_metadata.yml"
	// MaxFolderMetaBytes 는 폴더 파일 하나의 크기 상한이다. 넘으면 그 파일을 건너뛰고 경고한다.
	MaxFolderMetaBytes = 64 << 10
	// MaxFolderMetaDepth 는 문서 폴더에서 위로 올라가는 단계 상한이다(순환·이상한 트리 대비).
	MaxFolderMetaDepth = 64
)

// FolderMetaStop 은 이 폴더 위로는 더 올라가지 않는 경계인지다: macOS 의 마운트 볼륨 루트
// /Volumes/<디스크>(그 폴더의 파일은 읽고 멈춘다)와 Windows 드라이브 루트(예: C:\ — 어느
// 플랫폼에서 실행해도 문자열로 판정해 시험이 돌게). 그 밖은 경로 루트뿐이다(홈 멈춤은 HomeDirFor 가 따로 본다).
func FolderMetaStop(dir string) bool {
	if len(dir) == 3 && dir[1] == ':' && (dir[2] == '\\' || dir[2] == '/') &&
		(dir[0] >= 'A' && dir[0] <= 'Z' || dir[0] >= 'a' && dir[0] <= 'z') {
		return true // Windows 드라이브 루트(C:\)
	}
	if dir == "/" {
		return true
	}
	rest := strings.TrimPrefix(dir, "/Volumes/")
	return strings.HasPrefix(dir, "/Volumes/") && rest != "" && !strings.Contains(rest, "/")
}

// HomeDirFor 는 탐색을 멈출 홈 폴더다. 문서 폴더가 홈 아래(홈 자체 포함)면 홈을 돌려주고, 홈을 못
// 찾거나 문서가 홈 밖이면 빈글자(그때는 볼륨·경로 루트·깊이 상한까지만 간다 — 홈 밖 문서도 상속은 쓴다,
// itda-hyve#474 리뷰 R1 열린 결정 1 의 조정 정리).
func HomeDirFor(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	if dir == home {
		return home
	}
	rel, err := filepath.Rel(home, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "" // 문서가 홈 밖이다 — 홈은 이 트리의 위쪽이 아니다
	}
	return home
}

// folderMetaDirs 는 문서 폴더부터 위로 폴더를 올라가며 `_metadata.yml` 이 있는 폴더를 가까운 것부터
// 돌려준다. 홈 폴더(그 파일까지 읽고), 볼륨·경로 루트, 깊이 상한에서 멈춘다. 찾기와 감시(stamp)가 같은
// 목록을 쓴다 — 새로 생긴 파일도 감시가 잡는다.
func folderMetaDirs(docDir string) []string {
	var dirs []string
	home := HomeDirFor(docDir)
	for cur, depth := docDir, 0; ; cur, depth = filepath.Dir(cur), depth+1 {
		if st, err := os.Stat(filepath.Join(cur, FolderMetaName)); err == nil && st.Mode().IsRegular() {
			dirs = append(dirs, cur)
		}
		if cur == home || FolderMetaStop(cur) || depth >= MaxFolderMetaDepth {
			break // 홈(파일을 본 뒤)·볼륨 루트·상한에서 멈춘다
		}
		if parent := filepath.Dir(cur); parent == cur {
			break // 경로 루트
		}
	}
	return dirs
}

// findFolderMetas 는 문서에 적용할 폴더 파일을 먼 폴더 → 가까운 폴더 순으로 돌려준다(mdoc 이 가까운
// 것을 나중에 얹는다). docPath 가 비면(붙여넣은 원문 등) nil 이다. 읽지 못한 파일(크기 상한 초과·권한)은
// YAML 을 비우고 Err 에 까닭을 싣는다 — 그 파일만 건너뛰고 경고하는 규칙(mdoc inherit.go)에 맞춘다.
// 렌더마다 다시 읽는다(파일이 작고 몇 개뿐이라 캐시하지 않는다 — 같은 크기·같은 수정 시각으로 덮어쓴
// 변경을 캐시가 숨기지 않게. 감시의 판정 한계는 문서 자신의 감시와 같다).
func findFolderMetas(docPath string) []mdoc.FolderMeta {
	if docPath == "" {
		return nil
	}
	docDir := filepath.Dir(docPath)
	dirs := folderMetaDirs(docDir) // 가까운 것부터
	var out []mdoc.FolderMeta
	for i := len(dirs) - 1; i >= 0; i-- { // 먼 것부터
		p := filepath.Join(dirs[i], FolderMetaName)
		rel, err := filepath.Rel(docDir, p)
		if err != nil {
			rel = p
		}
		out = append(out, ReadFolderMeta(filepath.ToSlash(rel), p))
	}
	return out
}

// ReadFolderMeta 는 파일 하나를 연 다음 상한 안에서 읽는다(ReadMetaFile).
func ReadFolderMeta(rel, p string) mdoc.FolderMeta {
	r := ReadMetaFile(p)
	return mdoc.FolderMeta{Rel: rel, YAML: r.B, Err: r.Err}
}

// MetaFileRead 는 폴더 파일 하나를 연 파일에서 읽은 것이다. 내용(B)과 권한(Mode)은 같은 열린 파일에서 얻는다 —
// 저장이 기존 권한으로 놓을 때 내용과 다른 순간의 권한을 쓰지 않게(itda-hyve#504 리뷰 2회차 [1]).
type MetaFileRead struct {
	B       []byte
	Mode    os.FileMode
	Missing bool   // 열 때 없었다(Err 도 있다)
	Err     string // 읽지 못한 까닭(화면 문구). 비면 B·Mode 가 있다
}

// ReadMetaFile 은 파일을 연 다음 상한 안에서 읽는다(열린 파일이 정규 파일인지 보고 LimitReader 로 — Stat 뒤 커지거나
// 교체된 파일을 무한정 읽지 않게, Doc.Read 와 같은 막이). 열기는 openMetaFile(Unix 는 비차단 — 탐색 뒤 FIFO 로 교체된
// 파일에서 열기가 붙잡히지 않게).
func ReadMetaFile(p string) MetaFileRead {
	file, err := openMetaFile(p)
	if err != nil {
		return MetaFileRead{Missing: os.IsNotExist(err), Err: "읽지 못함"}
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return MetaFileRead{Err: "정규 파일이 아님"}
	}
	b, err := io.ReadAll(io.LimitReader(file, MaxFolderMetaBytes+1))
	if err != nil {
		return MetaFileRead{Err: "읽지 못함"}
	}
	if len(b) > MaxFolderMetaBytes {
		return MetaFileRead{Err: "파일이 너무 큼(64KB 넘음)"}
	}
	return MetaFileRead{B: b, Mode: st.Mode().Perm()}
}

// FolderMetaStamp 는 문서에 적용될 폴더 파일들의 상태(폴더·수정 시각·크기)를 한 글자로 묶는다.
// 감시(app.go watch·share.go sweep)가 「폴더 파일이 바뀌었는가」를 잰다 — 문서 자신의 감시
// (Doc.Changed)와 같은 방식이라 같은 한계가 있다(같은 크기·같은 수정 시각으로 덮어쓴 변경은 못 본다).
// docPath 가 비면 빈글자다.
func FolderMetaStamp(docPath string) string {
	if docPath == "" {
		return ""
	}
	var b strings.Builder
	for _, dir := range folderMetaDirs(filepath.Dir(docPath)) {
		st, err := os.Stat(filepath.Join(dir, FolderMetaName))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s|%d|%d;", filepath.ToSlash(dir), st.ModTime().UnixNano(), st.Size())
	}
	return b.String()
}
