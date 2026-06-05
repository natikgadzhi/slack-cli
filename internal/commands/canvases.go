package commands

import (
	"fmt"
	"os"
	"strings"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// canvasesCmd is the parent command for canvas-related subcommands.
var canvasesCmd = &cobra.Command{
	Use:   "canvases",
	Short: "View Slack canvases",
}

// canvasesReadCmd fetches and displays Slack canvas content by ID.
var canvasesReadCmd = &cobra.Command{
	Use:   "read <canvas-id>",
	Short: "Fetch and display a Slack canvas",
	Args:  cobra.ExactArgs(1),
	Example: `  slack-cli canvases read F12345678
  slack-cli canvases read F12345678 -o json`,
	RunE: runCanvasesRead,
}

func init() {
	canvasesCmd.AddCommand(canvasesReadCmd)
	rootCmd.AddCommand(canvasesCmd)
}

// runCanvasesRead fetches a canvas document by ID and renders it as plain text
// (table output) or the full document structure (JSON output).
func runCanvasesRead(cmd *cobra.Command, args []string) error {
	canvasID := args[0]

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	spinner := progress.NewSpinner("Fetching canvas", format)

	result, err := client.Call("canvases.sections.lookup", map[string]string{
		"canvas_id": canvasID,
	})

	spinner.Finish()

	if err != nil {
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching canvas: %w", err)
	}

	if output.IsJSON(format) {
		return output.PrintJSON(result)
	}

	// Render canvas sections as plain text.
	sections := extractCanvasSections(result)
	if len(sections) == 0 {
		fmt.Fprintln(os.Stderr, "no content found in canvas")
		return nil
	}

	text := renderCanvasSections(sections)
	fmt.Print(text)
	return nil
}

// canvasSection represents a parsed section from the canvas response.
type canvasSection struct {
	ID       string
	Elements []canvasElement
}

// canvasElement represents a single element within a canvas section.
type canvasElement struct {
	Type     string
	Children []canvasElement
	Text     string
	Style    map[string]any
	URL      string
	Level    int
}

// extractCanvasSections parses the "sections" array from the API response.
func extractCanvasSections(result map[string]any) []canvasSection {
	rawSections, ok := result["sections"].([]any)
	if !ok {
		return nil
	}

	sections := make([]canvasSection, 0, len(rawSections))
	for _, raw := range rawSections {
		s, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		section := canvasSection{
			ID: getString(s, "id"),
		}

		rawElements, ok := s["elements"].([]any)
		if !ok {
			// Some responses nest content under "document_content.nodes" instead.
			if docContent, ok := s["document_content"].(map[string]any); ok {
				rawElements, _ = docContent["nodes"].([]any)
			}
		}

		for _, rawElem := range rawElements {
			if elem, ok := rawElem.(map[string]any); ok {
				section.Elements = append(section.Elements, parseCanvasElement(elem))
			}
		}

		sections = append(sections, section)
	}

	return sections
}

// parseCanvasElement recursively parses a canvas document node into a
// canvasElement.
func parseCanvasElement(raw map[string]any) canvasElement {
	elem := canvasElement{
		Type: getString(raw, "type"),
		Text: getString(raw, "text"),
		URL:  getString(raw, "url"),
	}

	if level, ok := raw["indent"].(float64); ok {
		elem.Level = int(level)
	}
	if level, ok := raw["level"].(float64); ok {
		elem.Level = int(level)
	}

	if style, ok := raw["style"].(map[string]any); ok {
		elem.Style = style
	}

	if children, ok := raw["children"].([]any); ok {
		for _, child := range children {
			if c, ok := child.(map[string]any); ok {
				elem.Children = append(elem.Children, parseCanvasElement(c))
			}
		}
	}

	// Some nodes nest content under "elements" rather than "children".
	if elements, ok := raw["elements"].([]any); ok && len(elem.Children) == 0 {
		for _, el := range elements {
			if e, ok := el.(map[string]any); ok {
				elem.Children = append(elem.Children, parseCanvasElement(e))
			}
		}
	}

	return elem
}

// renderCanvasSections converts parsed canvas sections into a plain-text string.
func renderCanvasSections(sections []canvasSection) string {
	var sb strings.Builder
	for _, section := range sections {
		for _, elem := range section.Elements {
			renderElement(&sb, elem, 0)
		}
	}
	return sb.String()
}

// renderElement writes a single element (and its children) to the string
// builder, producing a readable plain-text rendition of the canvas content.
func renderElement(sb *strings.Builder, elem canvasElement, depth int) {
	indent := strings.Repeat("  ", depth+elem.Level)

	switch elem.Type {
	case "heading", "header":
		text := collectText(elem)
		if text != "" {
			sb.WriteString("# ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	case "heading_1", "h1":
		text := collectText(elem)
		if text != "" {
			sb.WriteString("# ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	case "heading_2", "h2":
		text := collectText(elem)
		if text != "" {
			sb.WriteString("## ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	case "heading_3", "h3":
		text := collectText(elem)
		if text != "" {
			sb.WriteString("### ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	case "paragraph", "p":
		text := collectText(elem)
		if text != "" {
			sb.WriteString(indent)
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	case "bulleted_list", "unordered_list":
		for _, child := range elem.Children {
			renderElement(sb, child, depth)
		}
	case "ordered_list", "numbered_list":
		for i, child := range elem.Children {
			text := collectText(child)
			if text != "" {
				sb.WriteString(indent)
				fmt.Fprintf(sb, "%d. %s\n", i+1, text)
			}
		}
		sb.WriteString("\n")
	case "list_item":
		text := collectText(elem)
		if text != "" {
			sb.WriteString(indent)
			sb.WriteString("- ")
			sb.WriteString(text)
			sb.WriteString("\n")
		}
		// Render nested lists within the list item.
		for _, child := range elem.Children {
			if isListType(child.Type) {
				renderElement(sb, child, depth+1)
			}
		}
	case "checklist_item", "todo":
		text := collectText(elem)
		checked := false
		if elem.Style != nil {
			if v, ok := elem.Style["checked"].(bool); ok {
				checked = v
			}
		}
		marker := "[ ]"
		if checked {
			marker = "[x]"
		}
		sb.WriteString(indent)
		sb.WriteString("- ")
		sb.WriteString(marker)
		sb.WriteString(" ")
		sb.WriteString(text)
		sb.WriteString("\n")
	case "code_block", "preformatted":
		text := collectText(elem)
		sb.WriteString("```\n")
		sb.WriteString(text)
		sb.WriteString("\n```\n\n")
	case "blockquote", "quote":
		text := collectText(elem)
		for _, line := range strings.Split(text, "\n") {
			sb.WriteString("> ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	case "divider", "horizontal_rule":
		sb.WriteString("---\n\n")
	case "link":
		text := collectText(elem)
		if elem.URL != "" {
			if text != "" && text != elem.URL {
				fmt.Fprintf(sb, "[%s](%s)", text, elem.URL)
			} else {
				sb.WriteString(elem.URL)
			}
		} else if text != "" {
			sb.WriteString(text)
		}
	case "text", "rich_text_section":
		text := collectText(elem)
		sb.WriteString(text)
	default:
		// For unknown types, try to extract and render any text content.
		text := collectText(elem)
		if text != "" {
			sb.WriteString(indent)
			sb.WriteString(text)
			sb.WriteString("\n")
		}
		// Still render children in case there are nested elements.
		for _, child := range elem.Children {
			renderElement(sb, child, depth)
		}
	}
}

// collectText extracts all text content from an element and its children
// recursively.
func collectText(elem canvasElement) string {
	if elem.Text != "" && len(elem.Children) == 0 {
		return elem.Text
	}

	var parts []string
	if elem.Text != "" {
		parts = append(parts, elem.Text)
	}
	for _, child := range elem.Children {
		// Skip list-type children — they're rendered separately.
		if isListType(child.Type) {
			continue
		}
		if t := collectText(child); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "")
}

// isListType returns true if the element type is a list container.
func isListType(t string) bool {
	switch t {
	case "bulleted_list", "unordered_list", "ordered_list", "numbered_list":
		return true
	}
	return false
}
