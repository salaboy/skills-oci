package cmd

import (
	"github.com/spf13/cobra"
)

const (
	defaultSkillsDir = ".agents/skills"
)

// additionalSkillsDirs lists the tool-specific directories that receive
// symlinks pointing back to the primary .agents/skills installation.
var additionalSkillsDirs = []string{
	".claude/skills",
	".codex/skills",
	".cursor/skills",
	".gemini/skills",
}

// NewRootCmd creates the root command for skills-oci.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "skills-oci",
		Short:   "Manage agent skills as OCI artifacts",
		Long:    "A CLI tool for packaging, pushing, and pulling agent skills as OCI artifacts following the Agent Skills OCI Artifacts Specification.",
		Version: version,
	}

	cmd.PersistentFlags().Bool("plain", false, "Disable interactive TUI (plain text output)")
	cmd.PersistentFlags().Bool("plain-http", false, "Use plain HTTP instead of HTTPS for registry connections")

	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newCleanCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newRegisterCmd())
	cmd.AddCommand(newCollectionCmd())

	return cmd
}
