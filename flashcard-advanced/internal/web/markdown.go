package web

import (
	"bytes"
	"html/template"
	"log"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// tag::markdown-engine[]

// 덱 스토리의 마크다운을 서버에서 HTML로 바꾼다. 브라우저에 마크다운
// 라이브러리를 들이지 않기 위한 것으로, 미리 보기도 같은 함수를 htmx로 부른다.
//
// html.WithUnsafe를 켜지 않으므로 원문에 섞인 생 HTML(<script> 등)은 goldmark가
// 지워 버린다. 사용자 입력을 그대로 받아도 스크립트가 화면에 실리지 않는다.
var markdownEngine = goldmark.New(
	// GFM: 표·취소선·자동 링크. WithHardWraps: 줄바꿈 한 번을 <br>로 살린다.
	// 스토리의 주 용도가 대화문이라 줄마다 빈 줄을 강요하지 않기 위해서다.
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// end::markdown-engine[]

// markdownHTML은 마크다운 원문을 신뢰할 수 있는 HTML 조각으로 바꾼다.
// 비어 있으면 빈 조각이다.
func markdownHTML(src string) template.HTML {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(src), &buf); err != nil {
		// Convert는 버퍼에 쓰므로 사실상 실패하지 않지만, 만약을 위해 원문을
		// 이스케이프한 문단으로 대신한다.
		log.Printf("markdown convert: %v", err)
		return template.HTML("<p>" + template.HTMLEscapeString(src) + "</p>")
	}
	return template.HTML(buf.String())
}
