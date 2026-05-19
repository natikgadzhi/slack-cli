package commands

import (
	"fmt"
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
