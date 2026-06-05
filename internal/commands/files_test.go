package commands

import (
	"testing"
)

func TestIsTextMimetype(t *testing.T) {
	tests := []struct {
		mimetype string
		want     bool
	}{
		{"text/plain", true},
		{"text/html", true},
		{"text/csv", true},
		{"text/markdown", true},
		{"text/x-python", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"application/x-yaml", true},
		{"application/yaml", true},
		{"application/x-sh", true},
		{"application/typescript", true},
		{"application/toml", true},
		{"application/x-toml", true},
		{"application/pdf", false},
		{"image/png", false},
		{"image/jpeg", false},
		{"application/zip", false},
		{"application/octet-stream", false},
		{"video/mp4", false},
		{"", false},
	}

	for _, tc := range tests {
		got := isTextMimetype(tc.mimetype)
		if got != tc.want {
			t.Errorf("isTextMimetype(%q) = %v, want %v", tc.mimetype, got, tc.want)
		}
	}
}

func TestExtractFileInfo(t *testing.T) {
	file := map[string]any{
		"id":                   "F12345678",
		"name":                 "report.txt",
		"title":                "Quarterly Report",
		"mimetype":             "text/plain",
		"filetype":             "text",
		"mode":                 "snippet",
		"user":                 "U12345678",
		"size":                 float64(2048),
		"created":              float64(1700000000),
		"url_private":          "https://files.slack.com/files-pri/T123/report.txt",
		"url_private_download": "https://files.slack.com/files-tmb/T123/report.txt",
		"channels":             []any{"C123", "C456"},
		"preview":              "First few lines of the file...",
	}

	info := extractFileInfo(file)

	stringTests := []struct {
		key  string
		want string
	}{
		{"id", "F12345678"},
		{"name", "report.txt"},
		{"title", "Quarterly Report"},
		{"mimetype", "text/plain"},
		{"filetype", "text"},
		{"mode", "snippet"},
		{"user", "U12345678"},
		{"url_private", "https://files.slack.com/files-pri/T123/report.txt"},
		{"url_private_download", "https://files.slack.com/files-tmb/T123/report.txt"},
		{"channels", "C123, C456"},
		{"preview", "First few lines of the file..."},
	}

	for _, tc := range stringTests {
		val, ok := info[tc.key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", tc.key, info[tc.key])
			continue
		}
		if val != tc.want {
			t.Errorf("extractFileInfo[%q] = %q, want %q", tc.key, val, tc.want)
		}
	}

	// Check size as int.
	size, ok := info["size"].(int)
	if !ok {
		t.Errorf("expected size to be int, got %T", info["size"])
	} else if size != 2048 {
		t.Errorf("extractFileInfo[size] = %d, want 2048", size)
	}

	// Check created timestamp.
	created, ok := info["created"].(string)
	if !ok {
		t.Errorf("expected created to be string, got %T", info["created"])
	} else if created != "14 Nov 2023 22:13" {
		t.Errorf("extractFileInfo[created] = %q, want %q", created, "14 Nov 2023 22:13")
	}
}

func TestExtractFileInfo_MinimalFile(t *testing.T) {
	file := map[string]any{
		"id":   "F99999",
		"name": "image.png",
	}

	info := extractFileInfo(file)

	if info["id"] != "F99999" {
		t.Errorf("extractFileInfo[id] = %v, want %q", info["id"], "F99999")
	}
	if info["name"] != "image.png" {
		t.Errorf("extractFileInfo[name] = %v, want %q", info["name"], "image.png")
	}

	// Missing fields should be empty strings or zero.
	for _, key := range []string{"title", "mimetype", "filetype", "mode", "user", "url_private", "url_private_download", "channels", "preview"} {
		val, ok := info[key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", key, info[key])
			continue
		}
		if val != "" {
			t.Errorf("extractFileInfo[%q] = %q, want empty string", key, val)
		}
	}

	size, ok := info["size"].(int)
	if !ok {
		t.Errorf("expected size to be int, got %T", info["size"])
	} else if size != 0 {
		t.Errorf("extractFileInfo[size] = %d, want 0", size)
	}
}

func TestExtractFileInfo_EmptyFile(t *testing.T) {
	file := map[string]any{}

	info := extractFileInfo(file)

	// All string fields should be empty.
	for _, key := range []string{"id", "name", "title", "mimetype", "filetype", "mode", "user", "url_private", "url_private_download", "channels", "preview"} {
		val, ok := info[key].(string)
		if !ok {
			t.Errorf("expected key %q to be a string, got %T", key, info[key])
			continue
		}
		if val != "" {
			t.Errorf("extractFileInfo[%q] = %q, want empty string", key, val)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int
		want string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tc := range tests {
		got := formatSize(tc.size)
		if got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		input any
		want  int
		ok    bool
	}{
		{float64(42), 42, true},
		{float64(0), 0, true},
		{float64(1.9), 1, true},
		{42, 42, true},
		{0, 0, true},
		{"42", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tc := range tests {
		got, ok := toInt(tc.input)
		if ok != tc.ok {
			t.Errorf("toInt(%v) ok = %v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if got != tc.want {
			t.Errorf("toInt(%v) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{float64(42.5), 42.5, true},
		{float64(0), 0, true},
		{42, 42, true},
		{0, 0, true},
		{"42.5", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tc := range tests {
		got, ok := toFloat(tc.input)
		if ok != tc.ok {
			t.Errorf("toFloat(%v) ok = %v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if got != tc.want {
			t.Errorf("toFloat(%v) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestExtractFileInfo_NoChannels(t *testing.T) {
	file := map[string]any{
		"id":   "F123",
		"name": "test.txt",
	}

	info := extractFileInfo(file)
	channels, ok := info["channels"].(string)
	if !ok {
		t.Fatalf("expected channels to be a string, got %T", info["channels"])
	}
	if channels != "" {
		t.Errorf("extractFileInfo[channels] = %q, want empty string", channels)
	}
}

func TestExtractFileInfo_SingleChannel(t *testing.T) {
	file := map[string]any{
		"id":       "F123",
		"name":     "test.txt",
		"channels": []any{"C123"},
	}

	info := extractFileInfo(file)
	channels, ok := info["channels"].(string)
	if !ok {
		t.Fatalf("expected channels to be a string, got %T", info["channels"])
	}
	if channels != "C123" {
		t.Errorf("extractFileInfo[channels] = %q, want %q", channels, "C123")
	}
}
