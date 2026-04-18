package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/skill"
	collectionadd "github.com/salaboy/skills-oci/pkg/tui/collection/add"
	collectionpush "github.com/salaboy/skills-oci/pkg/tui/collection/push"
	"github.com/spf13/cobra"
)

func newCollectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Manage skills collections",
		Long:  "Create, publish, and install Skills Collections — OCI Image Indexes that bundle skill references.",
	}

	cmd.AddCommand(newCollectionPushCmd())
	cmd.AddCommand(newCollectionAddCmd())
	cmd.AddCommand(newCollectionListCmd())

	return cmd
}

// collection push

func newCollectionPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push [OPTIONS] NAME[:TAG]",
		Short: "Create and push a skills collection to an OCI registry",
		Long:  "Resolves the given skill references, builds an OCI Image Index, and pushes it to a remote container registry.",
		Example: `  # Push a collection to GHCR
  skills-oci collection push ghcr.io/myorg/collections/dev-tools:latest \
    --name dev-tools \
    --skill ghcr.io/myorg/skills/manage-prs:1.0.0 \
    --skill ghcr.io/myorg/skills/lint-code:2.0.0

  # Push with a version tag
  skills-oci collection push ghcr.io/myorg/collections/dev-tools:v1.0.0 \
    --name dev-tools \
    --skill ghcr.io/myorg/skills/manage-prs:1.0.0`,
		Args: cobra.ExactArgs(1),
		RunE: runCollectionPush,
	}

	cmd.Flags().String("name", "", "Collection name (stored as io.agentskills.collection.name annotation)")
	cmd.Flags().StringArray("skill", nil, "Skill OCI reference to include (repeatable)")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("skill")

	return cmd
}

func runCollectionPush(cmd *cobra.Command, args []string) error {
	ref := args[0]
	name, _ := cmd.Flags().GetString("name")
	skillRefs, _ := cmd.Flags().GetStringArray("skill")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	if plain {
		return runCollectionPushPlain(ref, name, skillRefs, plainHTTP)
	}

	m := collectionpush.NewModel(ref, name, skillRefs, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if fm, ok := finalModel.(collectionpush.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runCollectionPushPlain(ref, name string, skillRefs []string, plainHTTP bool) error {
	fmt.Printf("Resolving %d skill reference(s)...\n", len(skillRefs))

	result, err := oci.PushCollection(context.Background(), oci.PushCollectionOptions{
		Reference: ref,
		Name:      name,
		SkillRefs: skillRefs,
		PlainHTTP: plainHTTP,
		OnStatus: func(phase string) {
			fmt.Printf("  %s\n", phase)
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nCollection pushed successfully!\n")
	fmt.Printf("  Collection: %s\n", name)
	fmt.Printf("  Reference:  %s:%s\n", result.Reference, result.Tag)
	fmt.Printf("  Digest:     %s\n", result.Digest)
	fmt.Printf("  Skills:     %d\n", result.Skills)
	return nil
}

// collection add

func newCollectionAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [OPTIONS] NAME[:TAG]",
		Short: "Install all skills from a collection",
		Long:  "Fetches a skills collection index from a remote registry and installs each referenced skill, updating skills.json and skills.lock.json.",
		Example: `  # Install all skills from a collection
  skills-oci collection add ghcr.io/myorg/collections/dev-tools:v1.0.0

  # Install from a local registry (plain HTTP)
  skills-oci collection add localhost:5000/collections/dev-tools:v1.0.0 --plain-http`,
		Args: cobra.ExactArgs(1),
		RunE: runCollectionAdd,
	}

	cmd.Flags().String("output", "", "Output directory for skill extraction (overrides default)")
	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json and skills.lock.json")

	return cmd
}

func runCollectionAdd(cmd *cobra.Command, args []string) error {
	ref := args[0]
	output, _ := cmd.Flags().GetString("output")
	projectDir, _ := cmd.Flags().GetString("project-dir")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")
	skillsDir := defaultSkillsDir

	if output == "" {
		output = filepath.Join(projectDir, skillsDir)
	}

	if plain {
		return runCollectionAddPlain(ref, output, projectDir, skillsDir, plainHTTP)
	}

	m := collectionadd.NewModel(ref, output, projectDir, skillsDir, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if fm, ok := finalModel.(collectionadd.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runCollectionAddPlain(ref, output, projectDir, skillsDir string, plainHTTP bool) error {
	fmt.Println("Fetching collection index...")

	collection, err := oci.FetchCollection(context.Background(), oci.FetchCollectionOptions{
		Reference: ref,
		PlainHTTP: plainHTTP,
		OnStatus: func(phase string) {
			fmt.Printf("  %s\n", phase)
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("  Collection: %s (%d skills)\n\n", collection.Name, len(collection.Skills))

	installed := 0
	for _, s := range collection.Skills {
		skillRef := s.Ref
		if skillRef == "" {
			skillRef = s.Digest
		}

		name := s.Name
		if name == "" {
			name = skillRef
		}
		fmt.Printf("  Installing %s...\n", name)

		result, err := oci.Pull(context.Background(), oci.PullOptions{
			Reference: skillRef,
			OutputDir: output,
			PlainHTTP: plainHTTP,
			OnStatus: func(phase string) {
				fmt.Printf("    %s\n", phase)
			},
		})
		if err != nil {
			return fmt.Errorf("installing skill %q: %w", skillRef, err)
		}

		if err := updateCollectionManifest(projectDir, skillsDir, result); err != nil {
			return fmt.Errorf("updating skills.json for %s: %w", result.Name, err)
		}
		if err := updateCollectionLockFile(projectDir, skillsDir, result); err != nil {
			return fmt.Errorf("updating skills.lock.json for %s: %w", result.Name, err)
		}

		installed++
		fmt.Printf("    ✓ %s (%s)\n", result.Name, result.Version)
	}

	fmt.Printf("\nSuccessfully installed %d skill(s) from collection %q\n", installed, collection.Name)
	return nil
}

func updateCollectionManifest(projectDir, skillsDir string, result *oci.PullResult) error {
	m, err := skill.LoadManifest(projectDir)
	if err != nil {
		return err
	}
	skill.AddToManifest(m, result.Name, result.Source(), result.Version, nil)
	return skill.SaveManifest(projectDir, m)
}

func updateCollectionLockFile(projectDir, skillsDir string, result *oci.PullResult) error {
	l, err := skill.LoadLock(projectDir)
	if err != nil {
		return err
	}

	extractPath := filepath.Join(skillsDir, result.Name)
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
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}

	skill.AddToLock(l, entry)
	return skill.SaveLock(projectDir, l)
}

// collection list

func newCollectionListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [OPTIONS] NAME[:TAG]",
		Short: "List skills in a collection without installing",
		Long:  "Fetches a skills collection index and displays the list of referenced skills.",
		Example: `  # List skills in a collection
  skills-oci collection list ghcr.io/myorg/collections/dev-tools:v1.0.0`,
		Args: cobra.ExactArgs(1),
		RunE: runCollectionList,
	}

	return cmd
}

func runCollectionList(cmd *cobra.Command, args []string) error {
	ref := args[0]
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	collection, err := oci.FetchCollection(context.Background(), oci.FetchCollectionOptions{
		Reference: ref,
		PlainHTTP: plainHTTP,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Collection: %s", collection.Name)
	if collection.Version != "" {
		fmt.Printf(" (%s)", collection.Version)
	}
	fmt.Printf("\nSource:     %s\n", ref)
	fmt.Printf("Skills:     %d\n\n", len(collection.Skills))

	for _, s := range collection.Skills {
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  %-32s", name)
		if s.Version != "" {
			fmt.Printf("  %-12s", s.Version)
		}
		if s.Description != "" {
			fmt.Printf("  %s", s.Description)
		}
		fmt.Println()
		if s.Ref != "" {
			fmt.Printf("    ref:    %s\n", s.Ref)
		}
		fmt.Printf("    digest: %s\n", s.Digest)
	}

	return nil
}
