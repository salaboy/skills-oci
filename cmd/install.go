package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/tui/load"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install all skills from skills.json that are not already present",
		Long:  "Reads skills.json and pulls any skills whose directories are missing from .agents/skills, then recreates symlinks in the tool-specific directories.",
		Example: `  # Install all missing skills
  skills-oci install

  # Install from a local registry
  skills-oci install --plain-http`,
		RunE: runInstall,
	}

	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json")

	return cmd
}

func runInstall(cmd *cobra.Command, args []string) error {
	projectDir, _ := cmd.Flags().GetString("project-dir")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	if plain {
		return runInstallPlain(projectDir, defaultSkillsDir, plainHTTP)
	}

	m := load.NewModel(projectDir, defaultSkillsDir, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if fm, ok := finalModel.(load.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runInstallPlain(projectDir, skillsDir string, plainHTTP bool) error {
	fmt.Println("  Reading skills.json")

	installed, skipped, err := load.LoadSkills(projectDir, skillsDir, plainHTTP, func(status string) {
		fmt.Printf("  %s\n", status)
	})
	if err != nil {
		return err
	}

	if len(installed) == 0 && len(skipped) == 0 {
		fmt.Println("\nNo skills defined in skills.json")
		return nil
	}

	fmt.Println()
	for _, name := range installed {
		fmt.Printf("  ✓ Installed %s\n", name)
	}
	for _, name := range skipped {
		fmt.Printf("  ○ Already present: %s\n", name)
	}

	fmt.Printf("\nDone: %d installed, %d already present\n", len(installed), len(skipped))
	return nil
}
