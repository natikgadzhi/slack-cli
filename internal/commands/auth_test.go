package commands

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cliauth "github.com/natikgadzhi/cli-kit/auth"
	clierrors "github.com/natikgadzhi/cli-kit/errors"

	"github.com/natikgadzhi/slack-cli/internal/auth"
)

func TestSlackErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "extracts code from CLIError message",
			err:  clierrors.NewCLIError(clierrors.ExitError, "slack api: invalid_auth"),
			want: "invalid_auth",
		},
		{
			name: "plain error returned verbatim",
			err:  fmt.Errorf("network unreachable"),
			want: "network unreachable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := slackErrorCode(tc.err); got != tc.want {
				t.Errorf("slackErrorCode = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExplainAuthError(t *testing.T) {
	// Known codes get a non-empty explanation.
	for _, code := range []string{"invalid_auth", "not_authed", "token_revoked", "token_expired", "account_inactive"} {
		if explainAuthError(code) == "" {
			t.Errorf("explainAuthError(%q) = empty, want an explanation", code)
		}
	}
	// Unknown codes return empty so the caller can fall back gracefully.
	if got := explainAuthError("some_future_code"); got != "" {
		t.Errorf("explainAuthError(unknown) = %q, want empty", got)
	}
}

func TestAuthChecklist_FlagsSourceMismatch(t *testing.T) {
	xoxc := auth.Credential{Source: cliauth.SourceEnvironment}
	xoxd := auth.Credential{Source: cliauth.SourceKeychain}

	items := authChecklist(xoxc, xoxd)

	var mentionsMix bool
	for _, item := range items {
		if strings.Contains(item, "stale mix") {
			mentionsMix = true
		}
	}
	if !mentionsMix {
		t.Errorf("authChecklist with mismatched sources should warn about a stale mix; got %v", items)
	}
}

func TestAuthChecklist_SameSourceNoMixWarning(t *testing.T) {
	cred := auth.Credential{Source: cliauth.SourceKeychain}

	items := authChecklist(cred, cred)

	for _, item := range items {
		if strings.Contains(item, "stale mix") {
			t.Errorf("authChecklist with matching sources should not warn about a stale mix; got %v", items)
		}
	}
}

func TestXoxdCandidates(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "encoded value also offers decoded form",
			input: "xoxd-a%2Fb%3D",
			want:  []string{"xoxd-a%2Fb%3D", "xoxd-a/b="},
		},
		{
			name:  "decoded value also offers encoded form",
			input: "xoxd-a/b=",
			want:  []string{"xoxd-a/b=", "xoxd-a%2Fb%3D"},
		},
		{
			name:  "decoded value with plus is encoded",
			input: "xoxd-a+b",
			want:  []string{"xoxd-a+b", "xoxd-a%2Bb"},
		},
		{
			name:  "plain alphanumeric has no alternate",
			input: "xoxd-abc123",
			want:  []string{"xoxd-abc123"},
		},
		{
			name:  "invalid percent escape yields no alternate",
			input: "xoxd-a%zz",
			want:  []string{"xoxd-a%zz"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := xoxdCandidates(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("xoxdCandidates(%q) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("xoxdCandidates(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestEncodingLabel(t *testing.T) {
	if got := encodingLabel("xoxd-a%2Fb"); got != "URL-encoded form" {
		t.Errorf("encodingLabel(encoded) = %q", got)
	}
	if got := encodingLabel("xoxd-a/b"); got != "raw (decoded) form" {
		t.Errorf("encodingLabel(decoded) = %q", got)
	}
}

// TestProbeXoxd_FlipsToWorkingEncoding verifies that when only the encoded form
// authenticates, probing a decoded paste flips to the encoded form.
func TestProbeXoxd_FlipsToWorkingEncoding(t *testing.T) {
	const accepted = "xoxd-a%2Fb" // server accepts only the encoded cookie

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := strings.TrimPrefix(r.Header.Get("Cookie"), "d=")
		w.Header().Set("Content-Type", "application/json")
		if cookie == accepted {
			_, _ = io.WriteString(w, `{"ok":true,"user":"u","team":"t"}`)
		} else {
			_, _ = io.WriteString(w, `{"ok":false,"error":"invalid_auth"}`)
		}
	}))
	defer srv.Close()
	t.Setenv("SLACK_BASE_URL", srv.URL+"/api")

	// Paste the decoded form; the encoded candidate should win.
	working, ok := probeXoxd("xoxc-test", xoxdCandidates("xoxd-a/b"))
	if !ok {
		t.Fatal("probeXoxd returned ok=false, want a working candidate")
	}
	if working != accepted {
		t.Errorf("probeXoxd = %q, want %q", working, accepted)
	}
}

// TestProbeXoxd_NoneWork verifies probeXoxd reports failure when Slack rejects
// every candidate.
func TestProbeXoxd_NoneWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":false,"error":"invalid_auth"}`)
	}))
	defer srv.Close()
	t.Setenv("SLACK_BASE_URL", srv.URL+"/api")

	if _, ok := probeXoxd("xoxc-test", xoxdCandidates("xoxd-a/b")); ok {
		t.Error("probeXoxd returned ok=true, want false when all candidates are rejected")
	}
}
