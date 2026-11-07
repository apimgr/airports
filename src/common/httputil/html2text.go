package httputil

import (
	"bytes"
	"fmt"
	"strings"

	xhtml "golang.org/x/net/html"
)

func HTML2TextConverter(source string, width int) string {
	if width < 20 {
		width = 80
	}
	doc, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return strings.TrimSpace(stripHTMLTags(source)) + "\n"
	}
	var out strings.Builder
	convertNode(&out, doc, width, 0)
	text := strings.TrimSpace(out.String())
	return text + "\n"
}

func RenderNoJSHTML(source string) (string, error) {
	doc, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("parse HTML for no-JS response: %w", err)
	}
	removeExecutableNodes(doc)
	var out bytes.Buffer
	if err := xhtml.Render(&out, doc); err != nil {
		return "", fmt.Errorf("render no-JS HTML: %w", err)
	}
	return out.String(), nil
}

func removeExecutableNodes(parent *xhtml.Node) {
	for node := parent.FirstChild; node != nil; {
		next := node.NextSibling
		if node.Type == xhtml.ElementNode && node.Data == "script" {
			parent.RemoveChild(node)
		} else {
			removeExecutableNodes(node)
		}
		node = next
	}
}

func convertNode(out *strings.Builder, node *xhtml.Node, width, indent int) {
	if node.Type == xhtml.ElementNode {
		switch node.Data {
		case "script", "style", "form", "input", "button", "noscript", "svg":
			return
		case "h1", "h2", "h3", "p", "blockquote", "pre", "li", "hr", "br":
			convertBlock(out, node, width, indent)
			return
		case "ul", "ol":
			convertList(out, node, width, indent)
			return
		case "table":
			convertTable(out, node, width, indent)
			return
		case "a":
			text := strings.TrimSpace(textContent(node))
			href := attribute(node, "href")
			if text != "" {
				out.WriteString(text)
				if href != "" && href != text {
					fmt.Fprintf(out, " [%s]", href)
				}
			}
			return
		case "strong", "b":
			out.WriteByte('*')
			out.WriteString(strings.TrimSpace(textContent(node)))
			out.WriteByte('*')
			return
		case "em", "i":
			out.WriteByte('_')
			out.WriteString(strings.TrimSpace(textContent(node)))
			out.WriteByte('_')
			return
		case "code":
			out.WriteByte('`')
			out.WriteString(strings.TrimSpace(textContent(node)))
			out.WriteByte('`')
			return
		}
	}
	if node.Type == xhtml.TextNode {
		text := strings.Join(strings.Fields(node.Data), " ")
		if text != "" {
			out.WriteString(text)
		}
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		convertNode(out, child, width, indent)
	}
}

func convertBlock(out *strings.Builder, node *xhtml.Node, width, indent int) {
	text := strings.TrimSpace(textContent(node))
	switch node.Data {
	case "br":
		out.WriteByte('\n')
	case "hr":
		writeLine(out, strings.Repeat("─", width))
	case "h1":
		writeLine(out, strings.Repeat("═", width))
		writeWrapped(out, strings.ToUpper(text), width, 0)
		writeLine(out, strings.Repeat("═", width))
	case "h2":
		writeLine(out, "─── "+text+" ───")
	case "h3":
		writeLine(out, "► "+text)
	case "pre":
		for _, line := range strings.Split(textContent(node), "\n") {
			writeLine(out, "    "+line)
		}
	case "blockquote":
		for _, line := range strings.Split(text, "\n") {
			writeLine(out, "│ "+line)
		}
	default:
		if node.Data == "p" {
			text = inlineText(node)
		}
		writeWrapped(out, text, width, indent)
	}
	if node.Data != "br" {
		out.WriteByte('\n')
	}
}

func inlineText(node *xhtml.Node) string {
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch {
		case child.Type == xhtml.TextNode:
			out.WriteString(strings.Join(strings.Fields(child.Data), " "))
		case child.Type == xhtml.ElementNode && (child.Data == "strong" || child.Data == "b"):
			writeInlineSeparator(&out)
			out.WriteByte('*')
			out.WriteString(strings.TrimSpace(textContent(child)))
			out.WriteByte('*')
		case child.Type == xhtml.ElementNode && (child.Data == "em" || child.Data == "i"):
			writeInlineSeparator(&out)
			out.WriteByte('_')
			out.WriteString(strings.TrimSpace(textContent(child)))
			out.WriteByte('_')
		case child.Type == xhtml.ElementNode && child.Data == "a":
			writeInlineSeparator(&out)
			label := strings.TrimSpace(textContent(child))
			href := attribute(child, "href")
			out.WriteString(label)
			if href != "" && href != label {
				fmt.Fprintf(&out, " [%s]", href)
			}
		default:
			out.WriteString(inlineText(child))
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

func writeInlineSeparator(out *strings.Builder) {
	if out.Len() > 0 && !strings.HasSuffix(out.String(), " ") {
		out.WriteByte(' ')
	}
}

func convertList(out *strings.Builder, list *xhtml.Node, width, indent int) {
	index := 0
	for item := list.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != xhtml.ElementNode || item.Data != "li" {
			continue
		}
		index++
		prefix := "  • "
		if list.Data == "ol" {
			prefix = fmt.Sprintf("  %d. ", index)
		}
		writeWrappedPrefix(out, prefix, strings.TrimSpace(textContent(item)), width, indent)
	}
	out.WriteByte('\n')
}

func convertTable(out *strings.Builder, table *xhtml.Node, width, indent int) {
	for row := table.FirstChild; row != nil; row = row.NextSibling {
		if row.Type != xhtml.ElementNode || row.Data != "tbody" && row.Data != "thead" {
			continue
		}
		for tr := row.FirstChild; tr != nil; tr = tr.NextSibling {
			if tr.Type != xhtml.ElementNode || tr.Data != "tr" {
				continue
			}
			var cells []string
			for cell := tr.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type == xhtml.ElementNode && (cell.Data == "td" || cell.Data == "th") {
					cells = append(cells, strings.TrimSpace(textContent(cell)))
				}
			}
			if len(cells) > 0 {
				writeLine(out, strings.Repeat(" ", indent)+strings.Join(cells, " │ "))
			}
		}
	}
	out.WriteByte('\n')
}

func textContent(node *xhtml.Node) string {
	var out strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			out.WriteString(current.Data)
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
			if child.Type == xhtml.ElementNode && (child.Data == "p" || child.Data == "li" || strings.HasPrefix(child.Data, "h")) {
				out.WriteByte(' ')
			}
		}
	}
	walk(node)
	return strings.Join(strings.Fields(out.String()), " ")
}

func writeWrapped(out *strings.Builder, value string, width, indent int) {
	if value == "" {
		out.WriteByte('\n')
		return
	}
	words := strings.Fields(value)
	line := strings.Repeat(" ", indent)
	for _, word := range words {
		if strings.TrimSpace(line) != "" && len(line)+1+len(word) > width {
			out.WriteString(line)
			out.WriteByte('\n')
			line = strings.Repeat(" ", indent)
		}
		if strings.TrimSpace(line) != "" {
			line += " "
		}
		line += word
	}
	if strings.TrimSpace(line) != "" {
		out.WriteString(line)
	}
	out.WriteByte('\n')
}

func writeWrappedPrefix(out *strings.Builder, prefix, value string, width, indent int) {
	words := strings.Fields(value)
	line := strings.Repeat(" ", indent) + prefix
	continuation := strings.Repeat(" ", indent+len(prefix))
	for _, word := range words {
		if len(strings.TrimSpace(line)) > 0 && len(line)+1+len(word) > width {
			out.WriteString(line)
			out.WriteByte('\n')
			line = continuation
		}
		if strings.TrimSpace(line) != "" {
			line += " "
		}
		line += word
	}
	if strings.TrimSpace(line) != "" {
		out.WriteString(line)
	}
	out.WriteByte('\n')
}

func writeLine(out *strings.Builder, line string) {
	out.WriteString(line)
	out.WriteByte('\n')
}

func attribute(node *xhtml.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func stripHTMLTags(source string) string {
	var out strings.Builder
	inside := false
	for _, char := range source {
		switch char {
		case '<':
			inside = true
		case '>':
			inside = false
		case '&':
			if !inside {
				out.WriteRune(char)
			}
		default:
			if !inside {
				out.WriteRune(char)
			}
		}
	}
	return out.String()
}
