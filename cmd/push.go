package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/tui/push"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push [OPTIONS] NAME[:TAG] PATH",
		Short: "Package and push a skill to an OCI registry",
		Long:  "Validates a skill directory, packages it as an OCI artifact, and pushes it to a remote container registry.",
		Example: `  # Push the skill in the current directory
  skills-oci push ghcr.io/myorg/skills/my-skill:1.0.0 .

  # Push a skill from a specific directory
  skills-oci push ghcr.io/myorg/skills/my-skill:1.0.0 ./my-skill

  # Push to a local registry (plain HTTP)
  skills-oci push localhost:5000/my-skill:1.0.0 . --plain-http`,
		Args: cobra.ExactArgs(2),
		RunE: runPush,
	}

	return cmd
}

func runPush(cmd *cobra.Command, args []string) error {
	ref := args[0]
	path := args[1]
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	if plain {
		return runPushPlain(ref, path, plainHTTP)
	}

	m := push.NewModel(ref, path, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if the final model has an error
	if fm, ok := finalModel.(push.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runPushPlain(ref, path string, plainHTTP bool) error {
	result, err := oci.Push(context.Background(), oci.PushOptions{
		Reference: ref,
		SkillDir:  path,
		PlainHTTP: plainHTTP,
		OnStatus: func(phase string) {
			fmt.Printf("  %s\n", phase)
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nSuccessfully pushed!\n")
	fmt.Printf("  Reference: %s:%s\n", result.Reference, result.Tag)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Size:      %d bytes\n", result.Size)
	return nil
}
