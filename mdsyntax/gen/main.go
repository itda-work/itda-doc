// gen 은 안내서(spec/*.md)의 syntax 구간을 모아 mdsyntax/topics/*.md 를, 표지 표에서 뽑은
// 편집기 자동 완성 후보를 mdsyntax/completions.json 으로 다시 쓴다(itda-hyve#434).
// 저장소 맨 위나 mdsyntax 에서 `go generate ./mdsyntax`(또는 just syntax) 로 돈다.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/itda-work/itda-doc/mdsyntax/extract"
)

func main() {
	// go generate 는 패키지 폴더(mdsyntax)에서 돈다
	docs, pkg := filepath.Join("..", "spec"), "."
	if _, err := os.Stat("go.mod"); err == nil {
		docs, pkg = "spec", "mdsyntax"
	}
	topics := filepath.Join(pkg, "topics")
	out, err := extract.Build(docs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdsyntax:", err)
		os.Exit(1)
	}
	for _, t := range extract.Topics {
		if err := os.WriteFile(filepath.Join(topics, t+".md"), out[t], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "mdsyntax:", err)
			os.Exit(1)
		}
	}
	comp, err := extract.CompletionsJSON(docs)
	if err == nil {
		err = os.WriteFile(filepath.Join(pkg, "completions.json"), comp, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdsyntax:", err)
		os.Exit(1)
	}
}
