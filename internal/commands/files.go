package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	clierrors "github.com/natikgadzhi/cli-kit/errors"
	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/spf13/cobra"

	"github.com/natikgadzhi/slack-cli/internal/api"
)

// fileInfoFields defines the standard metadata fields for file info tables.
var fileInfoFields = []kvField{
	{"id", "ID"},
	{"name", "Name"},
	{"title", "Title"},
	{"mimetype", "Type"},
	{"filetype", "File Type"},
	{"mode", "Mode"},
	{"user", "User"},
	{"created", "Created"},
	{"channels", "Channels"},
}

// filesCmd is the parent command for file-related subcommands.
var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage Slack files",
}

// filesReadCmd fetches a Slack file's metadata and content by file ID.
var filesReadCmd = &cobra.Command{
	Use:   "read <file-id>",
	Short: "Read a Slack file's metadata and content",
	Args: exactlyOneArg(
		"a file ID",
		"slack-cli files read <file-id>",
		"slack-cli files read F12345678",
	),
	Example: `  slack-cli files read F12345678
  slack-cli files read F12345678 -o json
  slack-cli files read F12345678 --download
  slack-cli files read F12345678 --download --download-dir ./my-files`,
	RunE: runFilesRead,
}

func init() {
	filesReadCmd.Flags().Bool("download", false, "Download the file to disk")
	filesReadCmd.Flags().String("download-dir", "slack-files", "Directory for downloaded files")

	filesCmd.AddCommand(filesReadCmd)
	rootCmd.AddCommand(filesCmd)
}

// isTextMimetype returns true if the mimetype indicates a text-like file whose
// content can be displayed inline.
var textMimetypes = map[string]bool{
	"application/json":          true,
	"application/xml":           true,
	"application/javascript":    true,
	"application/x-javascript":  true,
	"application/typescript":    true,
	"application/x-yaml":        true,
	"application/yaml":          true,
	"application/x-sh":          true,
	"application/x-shellscript": true,
	"application/toml":          true,
	"application/x-toml":        true,
}

func isTextMimetype(mimetype string) bool {
	if strings.HasPrefix(mimetype, "text/") {
		return true
	}
	return textMimetypes[mimetype]
}

// extractFileInfo extracts displayable fields from a raw Slack file object
// returned by files.info.
func extractFileInfo(file map[string]any) map[string]any {
	info := map[string]any{
		"id":       getString(file, "id"),
		"name":     getString(file, "name"),
		"title":    getString(file, "title"),
		"mimetype": getString(file, "mimetype"),
		"filetype": getString(file, "filetype"),
		"mode":     getString(file, "mode"),
		"user":     getString(file, "user"),
	}

	if size, ok := toInt(file["size"]); ok {
		info["size"] = size
	} else {
		info["size"] = 0
	}

	if created, ok := toFloat(file["created"]); ok {
		t := time.Unix(int64(created), 0).UTC()
		info["created"] = t.Format("02 Jan 2006 15:04")
	} else {
		info["created"] = ""
	}

	if u, ok := file["url_private"].(string); ok {
		info["url_private"] = u
	} else {
		info["url_private"] = ""
	}

	if u, ok := file["url_private_download"].(string); ok {
		info["url_private_download"] = u
	} else {
		info["url_private_download"] = ""
	}

	// Channels the file is shared in.
	if channels, ok := file["channels"].([]any); ok {
		chStrs := make([]string, 0, len(channels))
		for _, ch := range channels {
			if s, ok := ch.(string); ok {
				chStrs = append(chStrs, s)
			}
		}
		info["channels"] = strings.Join(chStrs, ", ")
	} else {
		info["channels"] = ""
	}

	// Preview content for text files (provided by Slack in the response).
	if preview, ok := file["preview"].(string); ok {
		info["preview"] = preview
	} else {
		info["preview"] = ""
	}

	return info
}

// formatSize formats a file size in bytes to a human-readable string.
func formatSize(size int) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}

// runFilesRead fetches a Slack file by ID and displays its metadata and content.
func runFilesRead(cmd *cobra.Command, args []string) error {
	fileID := args[0]

	format := output.Resolve(cmd)

	client, err := setupClientOnly()
	if err != nil {
		return err
	}

	prog := progress.NewSpinner("Fetching file info", format)

	result, err := client.Call("files.info", map[string]string{
		"file": fileID,
	})
	if err != nil {
		prog.Finish()
		if cliErr, ok := api.AsCLIError(err); ok {
			clierrors.PrintError(cliErr, output.IsJSON(format))
			os.Exit(cliErr.ExitCode)
		}
		return fmt.Errorf("fetching file info: %w", err)
	}

	prog.Finish()

	rawFile, ok := result["file"].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected response: missing file object")
	}

	fileInfo := extractFileInfo(rawFile)
	mimetype := fileInfo["mimetype"].(string)
	isText := isTextMimetype(mimetype)

	// Fetch text content inline if the file is text-like.
	var textContent string
	if isText {
		urlPrivate := fileInfo["url_private"].(string)
		if urlPrivate != "" {
			contentBytes, fetchErr := client.FetchFileContent(urlPrivate)
			if fetchErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not fetch file content: %v\n", fetchErr)
			} else {
				textContent = string(contentBytes)
			}
		}
	}

	// Download to disk if requested.
	download, _ := cmd.Flags().GetBool("download")
	if download {
		downloadDir, _ := cmd.Flags().GetString("download-dir")
		fileName := fileInfo["name"].(string)
		if fileName == "" {
			fileName = fileID
		}
		dest := filepath.Join(downloadDir, sanitizeFilename(fileName))
		downloadURL := fileInfo["url_private_download"].(string)
		if downloadURL == "" {
			downloadURL = fileInfo["url_private"].(string)
		}
		if downloadURL == "" {
			fmt.Fprintf(os.Stderr, "warning: no download URL available for this file\n")
		} else {
			if err := client.DownloadFile(downloadURL, dest); err != nil {
				fmt.Fprintf(os.Stderr, "warning: download failed: %v\n", err)
			} else {
				fileInfo["local_path"] = dest
				if !output.IsJSON(format) {
					fmt.Fprintf(os.Stderr, "Downloaded to %s\n", dest)
				}
			}
		}
	}

	// Render output.
	if output.IsJSON(format) {
		jsonOut := fileInfo
		if textContent != "" {
			jsonOut["content"] = textContent
		}
		return output.PrintJSON(jsonOut)
	}

	// Table output: key-value pairs.
	renderFileInfoTable(fileInfo)

	// Print text content to stdout after the metadata table.
	if textContent != "" {
		fmt.Println()
		fmt.Println(textContent)
	}

	return nil
}

// renderFileInfoTable renders file metadata as a two-column key-value table.
// Standard fields use renderKeyValueTable; extra rows (size, URL, download
// path) are appended with special formatting.
func renderFileInfoTable(info map[string]any) {
	t := table.New()
	t.Header("KEY", "VALUE")

	// Standard fields (skip empty).
	for _, f := range fileInfoFields {
		val := info[f.Key]
		if val == nil || val == "" {
			continue
		}
		t.Row(f.Label, fmt.Sprintf("%v", val))
	}

	// Size gets special formatting.
	if size, ok := info["size"].(int); ok && size > 0 {
		t.Row("Size", formatSize(size))
	}

	// Show URL for binary files.
	mimetype, _ := info["mimetype"].(string)
	if !isTextMimetype(mimetype) {
		if u, ok := info["url_private"].(string); ok && u != "" {
			t.Row("URL", u)
		}
	}

	// Show local path if file was downloaded.
	if lp, ok := info["local_path"].(string); ok && lp != "" {
		t.Row("Downloaded", lp)
	}

	_ = t.Flush()
}
