package converter

import (
	"bytes"
	"golang.org/x/net/html"
)

func HtmlToText(htmlStr string) string {
	tokenizer := html.NewTokenizer(bytes.NewBufferString(htmlStr))
	var buffer bytes.Buffer
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.TextToken {
			buffer.Write(tokenizer.Text())
		}
	}
	return buffer.String()
}
