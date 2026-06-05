package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractCanvasSections_Basic(t *testing.T) {
	result := map[string]any{
		"ok": true,
		"sections": []any{
			map[string]any{
				"id": "section-1",
				"elements": []any{
					map[string]any{
						"type": "heading",
						"children": []any{
							map[string]any{"type": "text", "text": "My Canvas"},
						},
					},
					map[string]any{
						"type": "paragraph",
						"children": []any{
							map[string]any{"type": "text", "text": "Hello world"},
						},
					},
				},
			},
		},
	}

	sections := extractCanvasSections(result)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].ID != "section-1" {
		t.Errorf("section ID = %q, want %q", sections[0].ID, "section-1")
	}
	if len(sections[0].Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(sections[0].Elements))
	}
	if sections[0].Elements[0].Type != "heading" {
		t.Errorf("first element type = %q, want %q", sections[0].Elements[0].Type, "heading")
	}
}

func TestExtractCanvasSections_Empty(t *testing.T) {
	result := map[string]any{"ok": true}
	sections := extractCanvasSections(result)
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestExtractCanvasSections_NoElements(t *testing.T) {
	result := map[string]any{
		"ok": true,
		"sections": []any{
			map[string]any{"id": "empty-section"},
		},
	}

	sections := extractCanvasSections(result)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if len(sections[0].Elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(sections[0].Elements))
	}
}

func TestExtractCanvasSections_DocumentContent(t *testing.T) {
	// Some responses nest content under "document_content.nodes".
	result := map[string]any{
		"ok": true,
		"sections": []any{
			map[string]any{
				"id": "section-doc",
				"document_content": map[string]any{
					"nodes": []any{
						map[string]any{
							"type": "paragraph",
							"children": []any{
								map[string]any{"type": "text", "text": "From document_content"},
							},
						},
					},
				},
			},
		},
	}

	sections := extractCanvasSections(result)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if len(sections[0].Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(sections[0].Elements))
	}
	text := collectText(sections[0].Elements[0])
	if text != "From document_content" {
		t.Errorf("text = %q, want %q", text, "From document_content")
	}
}

func TestParseCanvasElement_TextNode(t *testing.T) {
	raw := map[string]any{
		"type": "text",
		"text": "Hello",
	}

	elem := parseCanvasElement(raw)
	if elem.Type != "text" {
		t.Errorf("type = %q, want %q", elem.Type, "text")
	}
	if elem.Text != "Hello" {
		t.Errorf("text = %q, want %q", elem.Text, "Hello")
	}
}

func TestParseCanvasElement_WithChildren(t *testing.T) {
	raw := map[string]any{
		"type": "paragraph",
		"children": []any{
			map[string]any{"type": "text", "text": "part1"},
			map[string]any{"type": "text", "text": "part2"},
		},
	}

	elem := parseCanvasElement(raw)
	if len(elem.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(elem.Children))
	}
	if elem.Children[0].Text != "part1" {
		t.Errorf("child 0 text = %q, want %q", elem.Children[0].Text, "part1")
	}
}

func TestParseCanvasElement_WithLevel(t *testing.T) {
	raw := map[string]any{
		"type":   "list_item",
		"text":   "nested",
		"indent": float64(2),
	}

	elem := parseCanvasElement(raw)
	if elem.Level != 2 {
		t.Errorf("level = %d, want 2", elem.Level)
	}
}

func TestParseCanvasElement_WithStyle(t *testing.T) {
	raw := map[string]any{
		"type": "checklist_item",
		"text": "done task",
		"style": map[string]any{
			"checked": true,
		},
	}

	elem := parseCanvasElement(raw)
	if elem.Style == nil {
		t.Fatal("expected style to be set")
	}
	if checked, ok := elem.Style["checked"].(bool); !ok || !checked {
		t.Error("expected checked to be true")
	}
}

func TestParseCanvasElement_WithURL(t *testing.T) {
	raw := map[string]any{
		"type": "link",
		"text": "Click here",
		"url":  "https://example.com",
	}

	elem := parseCanvasElement(raw)
	if elem.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", elem.URL, "https://example.com")
	}
}

func TestCollectText_LeafNode(t *testing.T) {
	elem := canvasElement{Text: "hello"}
	if got := collectText(elem); got != "hello" {
		t.Errorf("collectText = %q, want %q", got, "hello")
	}
}

func TestCollectText_WithChildren(t *testing.T) {
	elem := canvasElement{
		Children: []canvasElement{
			{Text: "Hello "},
			{Text: "world"},
		},
	}
	if got := collectText(elem); got != "Hello world" {
		t.Errorf("collectText = %q, want %q", got, "Hello world")
	}
}

func TestCollectText_SkipsListChildren(t *testing.T) {
	elem := canvasElement{
		Text: "item text",
		Children: []canvasElement{
			{Type: "bulleted_list", Children: []canvasElement{
				{Type: "list_item", Text: "nested"},
			}},
		},
	}
	if got := collectText(elem); got != "item text" {
		t.Errorf("collectText = %q, want %q (should skip list children)", got, "item text")
	}
}

func TestCollectText_Nested(t *testing.T) {
	elem := canvasElement{
		Children: []canvasElement{
			{
				Children: []canvasElement{
					{Text: "deep"},
				},
			},
		},
	}
	if got := collectText(elem); got != "deep" {
		t.Errorf("collectText = %q, want %q", got, "deep")
	}
}

func TestRenderCanvasSections_Heading(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "heading", Children: []canvasElement{{Text: "Title"}}},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "# Title") {
		t.Errorf("expected heading '# Title' in output, got %q", got)
	}
}

func TestRenderCanvasSections_HeadingLevels(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "heading_1", Children: []canvasElement{{Text: "H1"}}},
			{Type: "heading_2", Children: []canvasElement{{Text: "H2"}}},
			{Type: "heading_3", Children: []canvasElement{{Text: "H3"}}},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "# H1") {
		t.Errorf("expected '# H1' in output, got %q", got)
	}
	if !strings.Contains(got, "## H2") {
		t.Errorf("expected '## H2' in output, got %q", got)
	}
	if !strings.Contains(got, "### H3") {
		t.Errorf("expected '### H3' in output, got %q", got)
	}
}

func TestRenderCanvasSections_Paragraph(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "paragraph", Children: []canvasElement{{Text: "Hello world"}}},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "Hello world") {
		t.Errorf("expected paragraph text, got %q", got)
	}
}

func TestRenderCanvasSections_BulletedList(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "bulleted_list", Children: []canvasElement{
				{Type: "list_item", Text: "item one"},
				{Type: "list_item", Text: "item two"},
			}},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "- item one") {
		t.Errorf("expected '- item one', got %q", got)
	}
	if !strings.Contains(got, "- item two") {
		t.Errorf("expected '- item two', got %q", got)
	}
}

func TestRenderCanvasSections_OrderedList(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "ordered_list", Children: []canvasElement{
				{Type: "list_item", Text: "first"},
				{Type: "list_item", Text: "second"},
			}},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "1. first") {
		t.Errorf("expected '1. first', got %q", got)
	}
	if !strings.Contains(got, "2. second") {
		t.Errorf("expected '2. second', got %q", got)
	}
}

func TestRenderCanvasSections_ChecklistItem(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "checklist_item", Text: "done", Style: map[string]any{"checked": true}},
			{Type: "checklist_item", Text: "todo", Style: map[string]any{"checked": false}},
			{Type: "checklist_item", Text: "no style"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "- [x] done") {
		t.Errorf("expected checked item, got %q", got)
	}
	if !strings.Contains(got, "- [ ] todo") {
		t.Errorf("expected unchecked item, got %q", got)
	}
	if !strings.Contains(got, "- [ ] no style") {
		t.Errorf("expected unchecked item for missing style, got %q", got)
	}
}

func TestRenderCanvasSections_CodeBlock(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "code_block", Text: "fmt.Println(\"hello\")"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "```\nfmt.Println(\"hello\")\n```") {
		t.Errorf("expected code block, got %q", got)
	}
}

func TestRenderCanvasSections_Blockquote(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "blockquote", Text: "wise words"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "> wise words") {
		t.Errorf("expected blockquote, got %q", got)
	}
}

func TestRenderCanvasSections_Divider(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "divider"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "---") {
		t.Errorf("expected divider, got %q", got)
	}
}

func TestRenderCanvasSections_Link(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "link", Text: "Example", URL: "https://example.com"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "[Example](https://example.com)") {
		t.Errorf("expected markdown link, got %q", got)
	}
}

func TestRenderCanvasSections_LinkURLOnly(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "link", URL: "https://example.com"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("expected bare URL, got %q", got)
	}
	// Should NOT be wrapped in markdown link syntax when text == url.
	if strings.Contains(got, "[https://example.com]") {
		t.Errorf("expected bare URL without markdown link, got %q", got)
	}
}

func TestRenderCanvasSections_MultipleSections(t *testing.T) {
	sections := []canvasSection{
		{Elements: []canvasElement{{Type: "paragraph", Text: "Section 1"}}},
		{Elements: []canvasElement{{Type: "paragraph", Text: "Section 2"}}},
	}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "Section 1") || !strings.Contains(got, "Section 2") {
		t.Errorf("expected both sections, got %q", got)
	}
}

func TestRenderCanvasSections_UnknownType(t *testing.T) {
	sections := []canvasSection{{
		Elements: []canvasElement{
			{Type: "unknown_widget", Text: "fallback text"},
		},
	}}
	got := renderCanvasSections(sections)
	if !strings.Contains(got, "fallback text") {
		t.Errorf("expected unknown type text to appear, got %q", got)
	}
}

func TestIsListType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"bulleted_list", true},
		{"unordered_list", true},
		{"ordered_list", true},
		{"numbered_list", true},
		{"paragraph", false},
		{"text", false},
		{"list_item", false},
		{"", false},
	}

	for _, tc := range tests {
		if got := isListType(tc.input); got != tc.want {
			t.Errorf("isListType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestRunCanvasesRead_JSON verifies the full command flow with a stubbed HTTP
// server returning canvas sections, rendered as JSON.
func TestRunCanvasesRead_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/canvases.sections.lookup") {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm error: %v", err)
		}
		if got := r.FormValue("canvas_id"); got != "F12345678" {
			t.Errorf("canvas_id = %q, want %q", got, "F12345678")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"sections": []any{
				map[string]any{
					"id": "s1",
					"elements": []any{
						map[string]any{
							"type": "paragraph",
							"children": []any{
								map[string]any{"type": "text", "text": "Hello from canvas"},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	// Override env to point at the test server and skip keychain.
	t.Setenv("SLACK_XOXC", "xoxc-test")
	t.Setenv("SLACK_XOXD", "xoxd-test")
	t.Setenv("SLACK_BASE_URL", srv.URL)

	// Run the command in JSON mode.
	cmd := canvasesReadCmd
	cmd.SetArgs([]string{"F12345678"})
	// Use the root command to properly wire flags.
	rootCmd.SetArgs([]string{"canvases", "read", "F12345678", "-o", "json"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command returned error: %v", err)
	}
}

// TestRunCanvasesRead_NoContent verifies the command handles an empty sections
// response gracefully.
func TestRunCanvasesRead_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"sections": []any{},
		})
	}))
	defer srv.Close()

	t.Setenv("SLACK_XOXC", "xoxc-test")
	t.Setenv("SLACK_XOXD", "xoxd-test")
	t.Setenv("SLACK_BASE_URL", srv.URL)

	// Table mode — should print "no content found" to stderr and return nil.
	rootCmd.SetArgs([]string{"canvases", "read", "F12345678", "-o", "table"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for empty canvas, got: %v", err)
	}
}
