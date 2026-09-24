package hh

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLToText converts hh.ru vacancy description HTML (typically <p>, <ul>,
// <li>, <strong>, <em>, <br>, and plain text) into readable plain text.
// Paragraphs are separated by a blank line and list items are rendered as
// "- item" lines. Whitespace is collapsed and HTML entities are decoded by
// the underlying tokenizer.
func HTMLToText(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}

	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		// Fall back to a best-effort strip if parsing fails.
		return collapseWhitespace(s)
	}

	var b strings.Builder
	for _, n := range nodes {
		walk(n, &b, false)
	}

	return finalize(b.String())
}

// walk renders node n (and its children) into b. inList indicates whether
// we are currently inside a <li>: block elements there (hh often wraps item
// text in <p>) are rendered inline so the item stays on one "- " line.
func walk(n *html.Node, b *strings.Builder, inList bool) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
	case html.ElementNode:
		switch n.DataAtom {
		case atom.Br:
			if inList {
				b.WriteString(" ")
			} else {
				b.WriteString("\n")
			}
			return
		case atom.P, atom.Div:
			if inList {
				b.WriteString(" ")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, b, inList)
				}
				b.WriteString(" ")
				return
			}
			ensureBlankLineBefore(b)
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, b, inList)
			}
			ensureNewlineBefore(b)
			b.WriteString("\n")
			return
		case atom.Ul, atom.Ol:
			ensureBlankLineBefore(b)
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, b, false)
			}
			b.WriteString("\n")
			return
		case atom.Li:
			ensureNewlineBefore(b)
			b.WriteString("- ")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, b, true)
			}
			ensureNewlineBefore(b)
			return
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, b, inList)
	}
}

func ensureNewlineBefore(b *strings.Builder) {
	s := b.String()
	if s != "" && !strings.HasSuffix(s, "\n") {
		b.WriteString("\n")
	}
}

func ensureBlankLineBefore(b *strings.Builder) {
	s := b.String()
	if s == "" {
		return
	}
	if !strings.HasSuffix(s, "\n") {
		b.WriteString("\n")
	}
	if !strings.HasSuffix(s, "\n\n") {
		b.WriteString("\n")
	}
}

// collapseWhitespace collapses runs of horizontal whitespace while leaving
// newlines intact, then trims each line.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// finalize collapses intra-line whitespace, trims each line, and collapses
// 3+ consecutive blank lines down to a single blank line.
func finalize(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, l := range lines {
		trimmed := strings.Join(strings.Fields(l), " ")
		if trimmed == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, trimmed)
	}
	// Drop blank lines between consecutive list items (hh sometimes wraps
	// every item in its own <ul>).
	compact := out[:0]
	for i, l := range out {
		if l == "" && i > 0 && i+1 < len(out) &&
			strings.HasPrefix(out[i-1], "- ") && strings.HasPrefix(out[i+1], "- ") {
			continue
		}
		compact = append(compact, l)
	}
	out = compact
	// Trim leading/trailing blank lines.
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}
