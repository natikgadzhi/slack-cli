package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/natikgadzhi/cli-kit/derived"
	"github.com/natikgadzhi/slack-cli/internal/config"
)

// Cache provides read/write access to Markdown files with YAML frontmatter
// stored in the local data directory.
type Cache struct {
	baseDir string
}

// NewCache creates a Cache rooted at config.DataDir().
// The base directory is created if it does not exist.
func NewCache() (*Cache, error) {
	dir, err := config.DataDir()
	if err != nil {
		return nil, fmt.Errorf("cache: %w", err)
	}
	if err := derived.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("cache: create base dir: %w", err)
	}
	return &Cache{baseDir: dir}, nil
}

// NewCacheWithDir creates a Cache rooted at the given directory.
// Useful for testing and the --derived flag.
func NewCacheWithDir(dir string) (*Cache, error) {
	if err := derived.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("cache: create base dir: %w", err)
	}
	return &Cache{baseDir: dir}, nil
}

// Get reads a cached object. If the file does not exist, found is false
// and err is nil. The returned content is the Markdown body (without
// frontmatter).
func (c *Cache) Get(objectType, slug string) (content []byte, meta Metadata, found bool, err error) {
	p, pathErr := c.path(objectType, slug)
	if pathErr != nil {
		return nil, Metadata{}, false, pathErr
	}
	data, readErr := os.ReadFile(p)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, Metadata{}, false, nil
		}
		return nil, Metadata{}, false, fmt.Errorf("cache get: %w", readErr)
	}

	meta, body, parseErr := UnmarshalFrontmatter(data)
	if parseErr != nil {
		return nil, Metadata{}, false, fmt.Errorf("cache get: %w", parseErr)
	}

	return body, meta, true, nil
}

// Put writes a cached object atomically. Intermediate directories are
// created as needed. The write uses a temp file + rename to avoid
// partial reads.
func (c *Cache) Put(objectType, slug string, content []byte, meta Metadata) error {
	p, pathErr := c.path(objectType, slug)
	if pathErr != nil {
		return pathErr
	}
	dir := filepath.Dir(p)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cache put: mkdir: %w", err)
	}

	// Set timestamps if not provided.
	now := time.Now().UTC()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = now
	}
	if meta.Tool == "" {
		meta.Tool = "slack-cli"
	}
	if meta.ObjectType == "" {
		meta.ObjectType = objectType
	}
	if meta.Slug == "" {
		meta.Slug = slug
	}

	data := MarshalFrontmatter(meta, content)

	// Atomic write: write to temp file in the same directory, then rename.
	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("cache put: create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache put: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache put: close temp: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache put: rename: %w", err)
	}

	return nil
}

// PutItem writes a single item file with pre-rendered markdown body and
// frontmatter. It reuses the same atomic-write and path-safety logic as Put.
// Unlike Put (which stores aggregate/JSON data), PutItem is designed for
// the --derived feature where each item gets its own rendered markdown file.
func (c *Cache) PutItem(objectType, slug string, body []byte, meta Metadata) error {
	return c.Put(objectType, slug, body, meta)
}

// path returns the filesystem path for a cached object.
// It returns an error if the resolved path escapes the cache base directory
// (e.g. via a slug containing "..").
func (c *Cache) path(objectType, slug string) (string, error) {
	// Slug may contain slashes (e.g. "C12345678/1741234567.123456"),
	// which naturally maps to subdirectories.
	p := filepath.Clean(filepath.Join(c.baseDir, objectType, slug+".md"))
	baseDir := filepath.Clean(c.baseDir)
	if !strings.HasPrefix(p, baseDir+string(filepath.Separator)) {
		return "", fmt.Errorf("cache: path %q escapes base directory", slug)
	}
	return p, nil
}

// MessageSlug returns the cache slug for a single message.
func MessageSlug(channelID, ts string) string {
	return filepath.Join(channelID, ts)
}

// ChannelHistorySlug returns the cache slug for a channel history range.
func ChannelHistorySlug(channelID, since, until string) string {
	return filepath.Join(channelID, since+"_"+until)
}

// SearchSlug returns the cache slug for a search query, using a truncated
// SHA-256 hash of the query string.
func SearchSlug(query string) string {
	h := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%x", h[:6]) // 12 hex chars
}
