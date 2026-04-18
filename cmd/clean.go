package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/salaboy/skills-oci/pkg/skill"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove all locally installed skill files managed by skills.json",
		Long:  "Removes the extracted skill directories from .agents/skills and all associated symlinks in tool-specific directories. skills.json is left intact so skills can be re-installed with 'skills-oci install'.",
		Example: `  # Clean all installed skills in the current project
  skills-oci clean

  # Clean skills in a specific project directory
  skills-oci clean --project-dir ./my-project`,
		RunE: runClean,
	}

	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json and skills.lock.json")

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	projectDir, _ := cmd.Flags().GetString("project-dir")

	manifest, err := skill.LoadManifest(projectDir)
	if err != nil {
		return fmt.Errorf("reading skills.json: %w", err)
	}

	if len(manifest.Skills) == 0 {
		fmt.Println("No skills defined in skills.json")
		return nil
	}

	lock, err := skill.LoadLock(projectDir)
	if err != nil {
		return fmt.Errorf("reading skills.lock.json: %w", err)
	}

	var removed, missing []string

	for _, s := range manifest.Skills {
		// Collect all paths to remove: primary dir + any additional installed paths
		// recorded in the lock file (symlinks to tool-specific directories).
		var paths []string

		primaryDir := filepath.Join(projectDir, defaultSkillsDir, s.Name)
		paths = append(paths, primaryDir)

		if locked := skill.GetLockedSkill(lock, s.Name); locked != nil {
			for _, p := range locked.AdditionalInstalledPaths {
				paths = append(paths, filepath.Join(projectDir, p))
			}
		} else {
			// Lock entry absent — fall back to the known symlink directories.
			for _, base := range additionalSkillsDirs {
				paths = append(paths, filepath.Join(projectDir, base, s.Name))
			}
		}

		found := false
		for _, p := range paths {
			if _, err := os.Lstat(p); err == nil {
				if err := os.RemoveAll(p); err != nil {
					return fmt.Errorf("removing %s: %w", p, err)
				}
				fmt.Printf("  Removed %s\n", p)
				found = true
			}
		}

		if found {
			removed = append(removed, s.Name)
		} else {
			missing = append(missing, s.Name)
		}

		skill.RemoveFromLock(lock, s.Name)
	}

	// Reset the lock file — all entries have been cleaned.
	lock.Skills = []skill.LockedSkill{}
	lock.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	if err := skill.SaveLock(projectDir, lock); err != nil {
		return fmt.Errorf("updating skills.lock.json: %w", err)
	}

	fmt.Println()
	if len(removed) > 0 {
		fmt.Printf("Cleaned %d skill(s): ", len(removed))
		for i, name := range removed {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(name)
		}
		fmt.Println()
	}
	if len(missing) > 0 {
		fmt.Printf("Already absent (%d): ", len(missing))
		for i, name := range missing {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(name)
		}
		fmt.Println()
	}

	fmt.Println("\nRun 'skills-oci install' to re-install all skills.")
	return nil
}
