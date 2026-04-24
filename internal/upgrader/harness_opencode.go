package upgrader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// openCodeAdapter delivers /docops-* commands to OpenCode.
//
// LocalDir:  .opencode/command   (parent; no docops/ subdirectory)
// FilenameFor("get") = "docops-get.md"
// ManifestDir: .opencode/command  (manifest sits flat next to the files)
// Layout: LayoutFlatPrefixFile — docops-get.md → /docops-get
//
// GlobalDir precedence (mirrors GSD getOpencodeGlobalDir):
//  1. $OPENCODE_CONFIG_DIR
//  2. filepath.Dir($OPENCODE_CONFIG)
//  3. $XDG_CONFIG_HOME/opencode
//  4. ~/.config/opencode
//
// Then "/command" is appended to whatever base is resolved.
type openCodeAdapter struct{}

func (openCodeAdapter) Slug() string     { return "opencode" }
func (openCodeAdapter) LocalDir() string { return ".opencode/command" }

func (openCodeAdapter) GlobalDir() (string, bool) {
	// 1. $OPENCODE_CONFIG_DIR
	if v := os.Getenv("OPENCODE_CONFIG_DIR"); v != "" {
		return filepath.Join(v, "command"), true
	}
	// 2. dirname($OPENCODE_CONFIG)
	if v := os.Getenv("OPENCODE_CONFIG"); v != "" {
		return filepath.Join(filepath.Dir(v), "command"), true
	}
	// 3. $XDG_CONFIG_HOME/opencode
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "opencode", "command"), true
	}
	// 4. ~/.config/opencode
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".config", "opencode", "command"), true
}

func (openCodeAdapter) Layout() Layout { return LayoutFlatPrefixFile }

func (openCodeAdapter) FilenameFor(cmd string) string {
	return fmt.Sprintf("docops-%s.md", cmd)
}

// ManifestDir returns LocalDir — the manifest sits flat in
// .opencode/command/.docops-manifest alongside the prefixed files.
func (openCodeAdapter) ManifestDir() string { return ".opencode/command" }

// TransformFrontmatter converts Claude-canonical frontmatter into the
// OpenCode dialect (per GSD convertClaudeToOpencodeFrontmatter):
//
//   - Drop "name:" — filename is the command ID in OpenCode.
//   - Convert "allowed-tools:" []string → "tools:" map[string]bool.
//   - Apply Claude→OpenCode tool-name mapping (see convertOpenCodeToolName).
//   - Preserve "description", "argument-hint", and all other keys verbatim.
func (openCodeAdapter) TransformFrontmatter(src map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(src))
	for k, v := range src {
		switch k {
		case "name":
			// Drop: filename serves as the command name in OpenCode.
		case "allowed-tools":
			// Convert list → tools map.
			tools := toStringSlice(v)
			if len(tools) > 0 {
				m := make(map[string]bool, len(tools))
				for _, t := range tools {
					m[convertOpenCodeToolName(t)] = true
				}
				out["tools"] = m
			}
		default:
			out[k] = v
		}
	}
	return out, nil
}

// convertOpenCodeToolName maps a Claude Code tool name to its OpenCode
// equivalent. Mirrors GSD's convertToolName (install.js:651):
//
//   - Special renames: AskUserQuestion→question, SlashCommand→skill,
//     TodoWrite→todowrite, WebFetch→webfetch, WebSearch→websearch.
//   - MCP tools (mcp__*): preserved verbatim.
//   - All others: lowercased.
func convertOpenCodeToolName(claudeTool string) string {
	switch claudeTool {
	case "AskUserQuestion":
		return "question"
	case "SlashCommand":
		return "skill"
	case "TodoWrite":
		return "todowrite"
	case "WebFetch":
		return "webfetch"
	case "WebSearch":
		return "websearch"
	}
	if strings.HasPrefix(claudeTool, "mcp__") {
		return claudeTool
	}
	return strings.ToLower(claudeTool)
}

// toStringSlice coerces an any value that should be a []string (or
// []any containing strings) into []string. Returns nil if v is nil.
func toStringSlice(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
