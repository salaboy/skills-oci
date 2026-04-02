package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newRegisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a Claude Code hook to load skills on session start",
		Long:  "Adds a SessionStart hook to .claude/settings.json that runs 'skills-oci load' every time a Claude Code session starts, ensuring all skills from skills.json are present.",
		Example: `  # Register hook for .claude/skills
  skills-oci register --claude

  # Register hook for .agents/skills
  skills-oci register`,
		RunE: runRegister,
	}

	cmd.Flags().String("project-dir", ".", "Project directory to register the hook in")

	return cmd
}

// claudeSettings represents the .claude/settings.json file structure.
type claudeSettings struct {
	Hooks map[string][]hookMatcher `json:"hooks,omitempty"`
	// Preserve unknown fields
	Extra map[string]json.RawMessage `json:"-"`
}

type hookMatcher struct {
	Matcher string       `json:"matcher"`
	Hooks   []hookEntry  `json:"hooks"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func runRegister(cmd *cobra.Command, args []string) error {
	projectDir, _ := cmd.Flags().GetString("project-dir")
	claude, _ := cmd.Flags().GetBool("claude")

	// Find skills-oci binary path
	binaryPath, err := resolveSkillsOCIPath()
	if err != nil {
		return fmt.Errorf("finding skills-oci binary: %w", err)
	}

	// Build the load command
	loadCmd := binaryPath + " load --plain"
	if claude {
		loadCmd += " --claude"
	}

	settingsDir := filepath.Join(projectDir, ".claude")
	settingsFile := filepath.Join(settingsDir, "settings.json")

	// Read existing settings or start fresh
	settings := make(map[string]json.RawMessage)
	data, err := os.ReadFile(settingsFile)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsFile, err)
		}
	}

	// Parse existing hooks or start fresh
	hooks := make(map[string][]hookMatcher)
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return fmt.Errorf("parsing hooks in %s: %w", settingsFile, err)
		}
	}

	// Check if a skills-oci load hook already exists
	newHook := hookEntry{
		Type:    "command",
		Command: loadCmd,
		Timeout: 30,
	}

	matchers := hooks["SessionStart"]
	found := false
	for i, m := range matchers {
		for j, h := range m.Hooks {
			if h.Type == "command" && isSkillsOCILoadCommand(h.Command) {
				// Update existing hook
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
		matchers = append(matchers, hookMatcher{
			Matcher: "",
			Hooks:   []hookEntry{newHook},
		})
	}
	hooks["SessionStart"] = matchers

	// Serialize hooks back into settings
	hooksRaw, err := json.Marshal(hooks)
	if err != nil {
		return fmt.Errorf("marshaling hooks: %w", err)
	}
	settings["hooks"] = hooksRaw

	// Write settings file
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("creating .claude directory: %w", err)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(settingsFile, out, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", settingsFile, err)
	}

	fmt.Printf("Registered SessionStart hook in %s\n", settingsFile)
	fmt.Printf("  Command: %s\n", loadCmd)
	fmt.Println("\nSkills from skills.json will be loaded automatically when a Claude Code session starts.")

	return nil
}

// resolveSkillsOCIPath returns the absolute path to the skills-oci binary.
func resolveSkillsOCIPath() (string, error) {
	// Try the current executable path first
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			return exe, nil
		}
	}

	// Fall back to looking up in PATH
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

// isSkillsOCILoadCommand checks if a command string is a skills-oci load invocation.
func isSkillsOCILoadCommand(cmd string) bool {
	return strings.Contains(cmd, "skills-oci load") || strings.Contains(cmd, "skills-oci\" load")
}
