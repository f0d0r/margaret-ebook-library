package converter

import (
	"testing"
)

func TestHtmlToText_EmptyString(t *testing.T) {
	result := HtmlToText("")
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestHtmlToText_PlainText(t *testing.T) {
	input := "Hello, World!"
	result := HtmlToText(input)
	if result != input {
		t.Fatalf("expected %q, got %q", input, result)
	}
}

func TestHtmlToText_SimpleTag(t *testing.T) {
	input := "<p>Hello</p>"
	expected := "Hello"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_NestedTags(t *testing.T) {
	input := "<div><p><span>Nested</span></p></div>"
	expected := "Nested"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_MultipleElements(t *testing.T) {
	input := "<p>First</p><p>Second</p>"
	expected := "FirstSecond"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_ElementsWithWhitespace(t *testing.T) {
	input := "<p>First</p> <p>Second</p>"
	expected := "First Second"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_WhitespaceInContent(t *testing.T) {
	input := "<p>  Hello  World  </p>"
	expected := "  Hello  World  "
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_SelfClosingTags(t *testing.T) {
	input := "Text<br/>More"
	expected := "TextMore"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_SelfClosingTagsWithAttributes(t *testing.T) {
	input := "Line1<hr class='separator' />Line2"
	expected := "Line1Line2"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_TagsWithAttributes(t *testing.T) {
	input := `<p class="intro" id="main">Content</p>`
	expected := "Content"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_ScriptTag(t *testing.T) {
	input := "<p>Visible</p><script>var x = 1;</script><p>Also visible</p>"
	// Script content is text and will be extracted
	expected := "Visiblevar x = 1;Also visible"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_StyleTag(t *testing.T) {
	input := "<p>Visible</p><style>.class { color: red; }</style><p>Text</p>"
	// Style content is text and will be extracted
	expected := "Visible.class { color: red; }Text"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_HtmlComments(t *testing.T) {
	input := "<p>Before</p><!-- This is a comment --><p>After</p>"
	// Comments are not text tokens, so they should be ignored
	expected := "BeforeAfter"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_MultilineContent(t *testing.T) {
	input := `<p>Line 1
Line 2
Line 3</p>`
	expected := "Line 1\nLine 2\nLine 3"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_MixedContent(t *testing.T) {
	input := `<div>
		<p>Paragraph 1</p>
		<p>Paragraph 2</p>
		<span>Span text</span>
	</div>`
	expected := "\n\t\tParagraph 1\n\t\tParagraph 2\n\t\tSpan text\n\t"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_SpecialCharacters(t *testing.T) {
	input := "<p>Text with & < > characters</p>"
	expected := "Text with & < > characters"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_UnicodeContent(t *testing.T) {
	input := "<p>Hello 世界 🌍</p>"
	expected := "Hello 世界 🌍"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_MalformedHtml_UnclosedTag(t *testing.T) {
	input := "<p>Text without closing tag"
	expected := "Text without closing tag"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_MalformedHtml_WrongClosingOrder(t *testing.T) {
	input := "<div><p>Text</div></p>"
	expected := "Text"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_Doctype(t *testing.T) {
	input := "<!DOCTYPE html><p>Content</p>"
	expected := "Content"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_EmptyTags(t *testing.T) {
	input := "<p></p><div></div>Text<span></span>"
	expected := "Text"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_Entities(t *testing.T) {
	// HTML entities are decoded by the tokenizer
	input := "<p>&lt;tag&gt;</p>"
	expected := "<tag>"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_ConsecutiveText(t *testing.T) {
	input := "Text1Text2Text3"
	expected := "Text1Text2Text3"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_ImgTag(t *testing.T) {
	input := "<p>Before</p><img src='image.jpg' alt='Image' /><p>After</p>"
	expected := "BeforeAfter"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_AnchorTag(t *testing.T) {
	input := "<p>Check <a href='https://example.com'>this link</a> out</p>"
	expected := "Check this link out"
	result := HtmlToText(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestHtmlToText_LongContent(t *testing.T) {
	input := "<p>" + string([]byte{'a' + byte(0)}) + "</p>"
	// Just verify it doesn't crash
	_ = HtmlToText(input)
}
