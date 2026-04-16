package commands

import (
	"testing"

	"github.com/natikgadzhi/slack-cli/internal/formatting"
)

func TestExtractSavedMessageItems_FiltersToMessages(t *testing.T) {
	items := []map[string]any{
		{
			"type":    "message",
			"channel": "C123",
			"message": map[string]any{
				"ts":   "1741234567.123456",
				"user": "U123",
				"text": "saved message",
			},
		},
		{
			"type": "file",
			"file": map[string]any{"id": "F123"},
		},
		{
			"type":    "message",
			"channel": "C456",
		},
	}

	got := extractSavedMessageItems(items)
	if len(got) != 1 {
		t.Fatalf("len(extractSavedMessageItems) = %d, want 1", len(got))
	}
	if got[0].channelID != "C123" {
		t.Errorf("channelID = %q, want %q", got[0].channelID, "C123")
	}
	if getString(got[0].message, "text") != "saved message" {
		t.Errorf("text = %q, want %q", getString(got[0].message, "text"), "saved message")
	}
}

func TestBuildSavedResults_UsesExistingPermalink(t *testing.T) {
	items := []savedMessageItem{
		{
			channelID: "C123",
			message: map[string]any{
				"ts":        "1741234567.123456",
				"user":      "alice",
				"text":      "saved message",
				"permalink": "https://team.slack.com/archives/C123/p1741234567123456",
			},
		},
	}

	got := buildSavedResults(items, "https://other.slack.com", true)
	if len(got) != 1 {
		t.Fatalf("len(buildSavedResults) = %d, want 1", len(got))
	}
	if got[0]["permalink"] != "https://team.slack.com/archives/C123/p1741234567123456" {
		t.Errorf("permalink = %v", got[0]["permalink"])
	}
	if got[0]["channel"] != "C123" {
		t.Errorf("channel = %v, want C123", got[0]["channel"])
	}
}

func TestBuildSavedResults_BuildsPermalinkFromTeamURL(t *testing.T) {
	items := []savedMessageItem{
		{
			channelID: "C12345678",
			message: map[string]any{
				"ts":   "1741234567.123456",
				"user": "alice",
				"text": "saved message",
			},
		},
	}

	got := buildSavedResults(items, "https://team.slack.com", true)
	if len(got) != 1 {
		t.Fatalf("len(buildSavedResults) = %d, want 1", len(got))
	}
	want := "https://team.slack.com/archives/C12345678/p1741234567123456"
	if got[0]["permalink"] != want {
		t.Errorf("permalink = %v, want %v", got[0]["permalink"], want)
	}
}

func TestSavedMessageText_FallsBackToAttachment(t *testing.T) {
	message := savedMessageText(buildTestFormattedAttachmentMessage())
	want := "Build failed - Service deploy is red"
	if message != want {
		t.Errorf("savedMessageText() = %q, want %q", message, want)
	}
}

func buildTestFormattedAttachmentMessage() formatting.Message {
	return formatting.Message{
		Attachment: &formatting.Attachment{
			Title: "Build failed",
			Text:  "Service deploy is red",
		},
	}
}
