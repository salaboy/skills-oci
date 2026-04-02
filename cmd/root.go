package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	defaultSkillsDir = ".agents/skills"
	claudeSkillsDir  = ".claude/skills"
)

// NewRootCmd creates the root command for skills-oci.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills-oci",
		Short: "Manage agent skills as OCI artifacts",
		Long:  "A CLI tool for packaging, pushing, and pulling agent skills as OCI artifacts following the Agent Skills OCI Artifacts Specification.",
	}

	cmd.PersistentFlags().Bool("plain", false, "Disable interactive TUI (plain text output)")
	cmd.PersistentFlags().Bool("plain-http", false, "Use plain HTTP instead of HTTPS for registry connections")
	cmd.PersistentFlags().Bool("claude", false, "Use .claude/skills instead of .agents/skills as the skills directory")

	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newLoadCmd())
	cmd.AddCommand(newRegisterCmd())

	return cmd
}

// resolveSkillsDir returns the skills directory path based on the --claude flag.
func resolveSkillsDir(cmd *cobra.Command) string {
	claude, _ := cmd.Flags().GetBool("claude")
	if claude {
		return claudeSkillsDir
	}
	return defaultSkillsDir
}

// resolveSkillsDirFromProject returns the full path to the skills directory within a project.
func resolveSkillsDirFromProject(cmd *cobra.Command, projectDir string) string {
	return filepath.Join(projectDir, resolveSkillsDir(cmd))
}
