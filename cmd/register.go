package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/salaboy/skills-oci/pkg/skill"
	"github.com/spf13/cobra"
)

// toolRegistrar maps a known tool-specific skills directory prefix to its
// display name and hook registration function.
type toolRegistrar struct {
	// dirPrefix is the leading path segment that identifies this tool
	// (e.g. ".claude" matches ".claude/skills" or ".claude/anything").
	dirPrefix string
	name      string
	register  func(projectDir, loadCmd string) error
}

// allTools is the ordered list of supported coding agents.
var allTools = []toolRegistrar{
	{".claude", "Claude Code", registerClaude},
	{".codex", "Codex CLI", registerCodex},
	{".cursor", "Cursor", registerCursor},
	{".gemini", "Gemini CLI", registerGemini},
}

func newRegisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register session-start hooks for coding agents",
		Long: `Reads skills.json to decide which agent hooks to register.

If no skill defines additionalBasePaths, hooks are registered for all
supported agents (Claude Code, Codex CLI, Cursor, Gemini CLI).

If any skill defines additionalBasePaths, only the agents whose
tool directory appears in those paths receive a hook.

Files written per agent:
  .claude/settings.json   — Claude Code  (hooks.SessionStart)
  .codex/hooks.json       — Codex CLI    (hooks.SessionStart)
  .codex/config.toml      — Codex CLI    (features.codex_hooks = true)
  .cursor/hooks.json      — Cursor       (hooks.sessionStart)
  .gemini/settings.json   — Gemini CLI   (hooks.SessionStart, timeout in ms)`,
		Example: `  skills-oci register`,
		RunE:    runRegister,
	}

	cmd.Flags().String("project-dir", ".", "Project directory to register hooks in")
	return cmd
}

func runRegister(cmd *cobra.Command, args []string) error {
	projectDir, _ := cmd.Flags().GetString("project-dir")

	binaryPath, err := resolveSkillsOCIPath()
	if err != nil {
		return fmt.Errorf("finding skills-oci binary: %w", err)
	}

	loadCmd := binaryPath + " install --plain"

	// Collect all additionalBasePaths declared across all skills in skills.json.
	manifest, err := skill.LoadManifest(projectDir)
	if err != nil {
		return fmt.Errorf("reading skills.json: %w", err)
	}

	allBasePaths := collectAdditionalBasePaths(manifest)

	// Determine which tools to register.
	// If no additionalBasePaths are configured anywhere, register all tools.
	// Otherwise register only the tools whose directory prefix is referenced.
	tools := selectTools(allBasePaths)

	for _, t := range tools {
		if err := t.register(projectDir, loadCmd); err != nil {
			fmt.Printf("  Warning: could not register %s hook: %v\n", t.name, err)
		} else {
			fmt.Printf("  Registered %s hook\n", t.name)
		}
	}

	if len(allBasePaths) == 0 {
		fmt.Println("\n  (no additionalBasePaths in skills.json — registered hooks for all agents)")
	}

	// Add skill directories to .gitignore so they are never accidentally committed.
	// Skills are fetched on demand via `skills-oci install`; only skills.json and
	// skills.lock.json should be tracked in version control.
	if err := updateGitignore(projectDir, tools); err != nil {
		fmt.Printf("  Warning: could not update .gitignore: %v\n", err)
	}

	fmt.Println("\nSkills from skills.json will be loaded automatically when sessions start.")
	return nil
}

// updateGitignore adds the primary skills directory and every registered tool's
// skills directory to .gitignore if the file already exists in projectDir.
// Entries that are already present are skipped silently.
func updateGitignore(projectDir string, tools []toolRegistrar) error {
	gitignorePath := filepath.Join(projectDir, ".gitignore")

	data, err := os.ReadFile(gitignorePath)
	if os.IsNotExist(err) {
		data = []byte{}
	} else if err != nil {
		return err
	}

	existing := string(data)

	// Build the list of paths to ignore: primary dir + one skills subdir per tool.
	paths := []string{defaultSkillsDir + "/"}
	for _, t := range tools {
		paths = append(paths, t.dirPrefix+"/skills/")
	}

	var added []string
	for _, p := range paths {
		if !gitignoreContains(existing, p) {
			added = append(added, p)
		}
	}

	if len(added) == 0 {
		return nil
	}

	// Append a clearly labelled block so the user can identify it later.
	var sb strings.Builder
	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("\n# Agent skills — fetched via `skills-oci install`, not stored in git\n")
	for _, p := range added {
		sb.WriteString(p)
		sb.WriteString("\n")
	}

	if err := os.WriteFile(gitignorePath, append(data, []byte(sb.String())...), 0644); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("  Updated .gitignore — the following skill directories will not be committed to git:")
	for _, p := range added {
		fmt.Printf("    %s\n", p)
	}
	fmt.Println("  Skills are fetched on demand by running `skills-oci install`.")

	return nil
}

// gitignoreContains reports whether line is already present in the gitignore content.
// It checks for an exact line match so partial paths don't produce false positives.
func gitignoreContains(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			return true
		}
	}
	return false
}

// collectAdditionalBasePaths returns the union of all additionalBasePaths
// declared across every skill in the manifest.
func collectAdditionalBasePaths(m *skill.SkillsManifest) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range m.Skills {
		for _, p := range s.AdditionalBasePaths {
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				result = append(result, p)
			}
		}
	}
	return result
}

// selectTools returns the subset of allTools that should have hooks registered.
// When basePaths is empty every tool is selected; otherwise only tools whose
// dirPrefix is referenced by at least one path are selected.
func selectTools(basePaths []string) []toolRegistrar {
	if len(basePaths) == 0 {
		return allTools
	}

	var selected []toolRegistrar
	for _, t := range allTools {
		for _, p := range basePaths {
			// Match if the path equals the prefix or starts with "<prefix>/".
			if p == t.dirPrefix || strings.HasPrefix(p, t.dirPrefix+"/") {
				selected = append(selected, t)
				break
			}
		}
	}
	return selected
}

// ---------------------------------------------------------------------------
// Claude Code — .claude/settings.json
// hooks.SessionStart, timeout in seconds
// ---------------------------------------------------------------------------

func registerClaude(projectDir, loadCmd string) error {
	settingsDir := filepath.Join(projectDir, ".claude")
	settingsFile := filepath.Join(settingsDir, "settings.json")
	return registerJSONHook(settingsFile, settingsDir, "SessionStart", loadCmd, 30)
}

// ---------------------------------------------------------------------------
// Gemini CLI — .gemini/settings.json
// hooks.SessionStart, timeout in milliseconds
// ---------------------------------------------------------------------------

func registerGemini(projectDir, loadCmd string) error {
	settingsDir := filepath.Join(projectDir, ".gemini")
	settingsFile := filepath.Join(settingsDir, "settings.json")
	// Gemini uses milliseconds for timeout
	return registerJSONHook(settingsFile, settingsDir, "SessionStart", loadCmd, 30000)
}

// ---------------------------------------------------------------------------
// Codex CLI — .codex/hooks.json + .codex/config.toml
// hooks.SessionStart, timeout in seconds; requires feature flag in config.toml
// ---------------------------------------------------------------------------

func registerCodex(projectDir, loadCmd string) error {
	codexDir := filepath.Join(projectDir, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return fmt.Errorf("creating .codex directory: %w", err)
	}

	// Enable the hooks feature flag in config.toml
	if err := ensureCodexFeatureFlag(filepath.Join(codexDir, "config.toml")); err != nil {
		return fmt.Errorf("updating .codex/config.toml: %w", err)
	}

	hooksFile := filepath.Join(codexDir, "hooks.json")
	return registerJSONHook(hooksFile, codexDir, "SessionStart", loadCmd, 30)
}

// ensureCodexFeatureFlag ensures `codex_hooks = true` is present under [features] in the TOML file.
func ensureCodexFeatureFlag(configFile string) error {
	const featureFlag = "codex_hooks = true"
	const featureSection = "[features]"

	data, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(data)

	// Already present — nothing to do.
	if strings.Contains(content, featureFlag) {
		return nil
	}

	// Find the [features] section and inject the flag after it.
	if strings.Contains(content, featureSection) {
		content = strings.Replace(content, featureSection, featureSection+"\n"+featureFlag, 1)
	} else {
		// Append a new section.
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += featureSection + "\n" + featureFlag + "\n"
	}

	return os.WriteFile(configFile, []byte(content), 0644)
}

// ---------------------------------------------------------------------------
// Cursor — .cursor/hooks.json
// hooks.sessionStart (lowercase s), timeout in seconds, no "type" or "matcher"
// ---------------------------------------------------------------------------

type cursorHooksFile struct {
	Version int                        `json:"version"`
	Hooks   map[string][]cursorHook    `json:"hooks,omitempty"`
}

type cursorHook struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func registerCursor(projectDir, loadCmd string) error {
	cursorDir := filepath.Join(projectDir, ".cursor")
	hooksFile := filepath.Join(cursorDir, "hooks.json")

	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		return fmt.Errorf("creating .cursor directory: %w", err)
	}

	// Read existing file or start fresh.
	hf := cursorHooksFile{Version: 1, Hooks: make(map[string][]cursorHook)}
	data, err := os.ReadFile(hooksFile)
	if err == nil {
		_ = json.Unmarshal(data, &hf)
		if hf.Hooks == nil {
			hf.Hooks = make(map[string][]cursorHook)
		}
	}

	newHook := cursorHook{Command: loadCmd, Timeout: 30}

	// Update or insert.
	existing := hf.Hooks["sessionStart"]
	found := false
	for i, h := range existing {
		if isSkillsOCILoadCommand(h.Command) {
			existing[i] = newHook
			found = true
			break
		}
	}
	if !found {
		existing = append(existing, newHook)
	}
	hf.Hooks["sessionStart"] = existing

	out, err := json.MarshalIndent(hf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling hooks: %w", err)
	}
	out = append(out, '\n')
	return os.WriteFile(hooksFile, out, 0644)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// hookMatcher is the nested hook structure used by Claude, Gemini, and Codex.
type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// registerJSONHook reads a JSON settings file that has a top-level "hooks" key,
// upserts a session-start entry, and writes it back.
func registerJSONHook(settingsFile, settingsDir, eventKey, loadCmd string, timeout int) error {
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", settingsDir, err)
	}

	settings := make(map[string]json.RawMessage)
	data, err := os.ReadFile(settingsFile)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsFile, err)
		}
	}

	hooks := make(map[string][]hookMatcher)
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return fmt.Errorf("parsing hooks in %s: %w", settingsFile, err)
		}
	}

	newHook := hookEntry{Type: "command", Command: loadCmd, Timeout: timeout}

	matchers := hooks[eventKey]
	found := false
	for i, m := range matchers {
		for j, h := range m.Hooks {
			if h.Type == "command" && isSkillsOCILoadCommand(h.Command) {
				matchers[i].Hooks[j] = newHook
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		matchers = append(matchers, hookMatcher{Matcher: "", Hooks: []hookEntry{newHook}})
	}
	hooks[eventKey] = matchers

	hooksRaw, err := json.Marshal(hooks)
	if err != nil {
		return fmt.Errorf("marshaling hooks: %w", err)
	}
	settings["hooks"] = hooksRaw

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	out = append(out, '\n')
	return os.WriteFile(settingsFile, out, 0644)
}

// resolveSkillsOCIPath returns the absolute path to the skills-oci binary.
func resolveSkillsOCIPath() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			return exe, nil
		}
	}

	path, err := exec.LookPath("skills-oci")
	if err != nil {
		return "", fmt.Errorf("skills-oci not found in PATH: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

// isSkillsOCILoadCommand checks if a command string is a skills-oci install invocation.
func isSkillsOCILoadCommand(cmd string) bool {
	return strings.Contains(cmd, "skills-oci install") || strings.Contains(cmd, "skills-oci\" install")
}

