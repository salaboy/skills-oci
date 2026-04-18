package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/skill"
	"github.com/salaboy/skills-oci/pkg/tui/add"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [OPTIONS] NAME[:TAG]",
		Short: "Install a skill from an OCI registry",
		Long:  "Pulls a skill artifact from a remote container registry by its NAME[:TAG] reference, extracts it to .agents/skills, and creates symlinks in .claude/skills, .codex/skills, .cursor/skills, and .gemini/skills.",
		Example: `  # Install a skill from GHCR
  skills-oci add ghcr.io/myorg/skills/my-skill:1.0.0

  # Install from a local registry (plain HTTP)
  skills-oci add localhost:5000/my-skill:1.0.0 --plain-http

  # Install to a custom output directory
  skills-oci add ghcr.io/myorg/skills/my-skill:1.0.0 --output ./custom/path

  # Install relative to a specific project directory
  skills-oci add ghcr.io/myorg/skills/my-skill:1.0.0 --project-dir ./my-project`,
		Args: cobra.ExactArgs(1),
		RunE: runAdd,
	}

	cmd.Flags().String("output", "", "Output directory for skill extraction (overrides default)")
	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json and skills.lock.json")
	cmd.Flags().StringArray("additional-base-path", nil, "Extra base directory to also install the skill into (repeatable)")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	ref := args[0]
	output, _ := cmd.Flags().GetString("output")
	projectDir, _ := cmd.Flags().GetString("project-dir")
	extraPaths, _ := cmd.Flags().GetStringArray("additional-base-path")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	// Always install to .agents/skills and symlink into all tool-specific dirs.
	additionalBasePaths := additionalSkillsDirs[:]
	for _, p := range extraPaths {
		additionalBasePaths = appendIfMissing(additionalBasePaths, p)
	}

	if output == "" {
		output = filepath.Join(projectDir, defaultSkillsDir)
	}

	additionalOutputDirs := resolveAdditionalDirs(projectDir, additionalBasePaths)

	if plain {
		return runAddPlain(ref, output, additionalOutputDirs, additionalBasePaths, projectDir, defaultSkillsDir, plainHTTP)
	}

	m := add.NewModel(ref, output, additionalOutputDirs, additionalBasePaths, projectDir, defaultSkillsDir, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if fm, ok := finalModel.(add.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

// appendIfMissing appends s to slice only if it is not already present.
func appendIfMissing(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// resolveAdditionalDirs resolves additional base paths relative to projectDir.
func resolveAdditionalDirs(projectDir string, additionalBasePaths []string) []string {
	if len(additionalBasePaths) == 0 {
		return nil
	}
	dirs := make([]string, len(additionalBasePaths))
	for i, p := range additionalBasePaths {
		dirs[i] = filepath.Join(projectDir, p)
	}
	return dirs
}

func runAddPlain(ref, output string, additionalOutputDirs, additionalBasePaths []string, projectDir, skillsDir string, plainHTTP bool) error {
	result, err := oci.Pull(context.Background(), oci.PullOptions{
		Reference:            ref,
		OutputDir:            output,
		AdditionalOutputDirs: additionalOutputDirs,
		PlainHTTP:            plainHTTP,
		OnStatus: func(phase string) {
			fmt.Printf("  %s\n", phase)
		},
	})
	if err != nil {
		return err
	}

	// Update skills.json
	fmt.Println("  Updating skills.json")
	if err := updateManifest(projectDir, skillsDir, additionalBasePaths, result); err != nil {
		return fmt.Errorf("updating skills.json: %w", err)
	}

	// Update skills.lock.json
	fmt.Println("  Updating skills.lock.json")
	if err := updateLockFile(projectDir, skillsDir, additionalBasePaths, result); err != nil {
		return fmt.Errorf("updating skills.lock.json: %w", err)
	}

	fmt.Printf("\nSuccessfully installed!\n")
	fmt.Printf("  Name:      %s\n", result.Name)
	fmt.Printf("  Version:   %s\n", result.Version)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Extracted: %s\n", result.ExtractTo)
	for _, p := range additionalBasePaths {
		fmt.Printf("  Also in:   %s\n", filepath.Join(p, result.Name))
	}
	return nil
}

// updateManifest loads skills.json, adds/updates the skill entry, and saves it.
func updateManifest(projectDir, skillsDir string, additionalBasePaths []string, result *oci.PullResult) error {
	m, err := skill.LoadManifest(projectDir)
	if err != nil {
		return err
	}

	skill.AddToManifest(m, result.Name, result.Source(), result.Version, additionalBasePaths)

	return skill.SaveManifest(projectDir, m)
}

// updateLockFile loads skills.lock.json, adds/updates the skill entry, and saves it.
func updateLockFile(projectDir, skillsDir string, additionalBasePaths []string, result *oci.PullResult) error {
	l, err := skill.LoadLock(projectDir)
	if err != nil {
		return err
	}

	extractPath := filepath.Join(skillsDir, result.Name)

	var additionalInstalledPaths []string
	for _, base := range additionalBasePaths {
		additionalInstalledPaths = append(additionalInstalledPaths, filepath.Join(base, result.Name))
	}

	entry := skill.LockedSkill{
		Name: result.Name,
		Path: extractPath,
		Source: skill.LockSource{
			Registry:   result.Registry,
			Repository: result.Repository,
			Tag:        result.Tag,
			Digest:     result.Digest,
			Ref:        result.FullRef(),
		},
		InstalledAt:              time.Now().UTC().Format(time.RFC3339),
		AdditionalInstalledPaths: additionalInstalledPaths,
	}

	skill.AddToLock(l, entry)

	return skill.SaveLock(projectDir, l)
}
