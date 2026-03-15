# Task 17: Homebrew tap

**Phase**: 4 — Polish
**Blocked by**: #16

## Objective

Set up a Homebrew tap so users can install slack-cli with `brew install` (PROJECT_PROMPT item 2). No Go toolchain required on the user's machine.

## Design

- Tap repo: `../taps/` locally → `github.com/natikgadzhi/taps.git` remote
- This is a shared tap repo (user mentions "multiple tools like this"), so structure accordingly
- Formula: `Formula/slack-cli.rb`

## Acceptance criteria

- [ ] `../taps/` directory created (or exists)
- [ ] `../taps/Formula/slack-cli.rb` — Homebrew formula that:
  - Downloads the pre-built binary from GitHub Releases
  - Supports macOS (Intel + Apple Silicon) and Linux
  - Uses SHA256 checksums from the release
  - Installs binary to `bin/slack-cli`
  - Has a test block that runs `slack-cli --help`
- [ ] GoReleaser config (from Task #16) auto-updates this formula on release via `brews:` section
- [ ] Installation works: `brew tap natikgadzhi/taps && brew install slack-cli`
- [ ] `which slack-cli` → Homebrew path
- [ ] `slack-cli --help` → works

## Formula template

```ruby
class SlackCli < Formula
  desc "Read-only Slack CLI for fetching messages, threads, and history"
  homepage "https://github.com/natikgadzhi/slack-cli"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/natikgadzhi/slack-cli/releases/download/v#{version}/slack-cli_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/natikgadzhi/slack-cli/releases/download/v#{version}/slack-cli_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    url "https://github.com/natikgadzhi/slack-cli/releases/download/v#{version}/slack-cli_#{version}_linux_amd64.tar.gz"
    sha256 "PLACEHOLDER"
  end

  def install
    bin.install "slack-cli"
  end

  test do
    assert_match "Slack read-only CLI", shell_output("#{bin}/slack-cli --help")
  end
end
```

## Notes

- The formula template above is a starting point — GoReleaser will auto-generate and push the real one
- GoReleaser `brews:` config needs the tap repo URL and a GitHub token with push access
- First release will need to be triggered manually (push a `v0.1.0` tag)
