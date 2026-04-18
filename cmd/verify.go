package cmd

import (
	"context"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/skill"
	tuiverify "github.com/salaboy/skills-oci/pkg/tui/verify"
	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the integrity of all installed skills",
		Long: `Checks each skill listed in skills.json against its installed OCI artifact using cosign.

Two checks are performed for every skill:
  - Signature   verifies the artifact was signed via Sigstore keyless signing
  - SLSA        verifies an attached SLSA provenance attestation

cosign must be installed and available in PATH. Install it from https://docs.sigstore.dev/cosign/system_config/installation/`,
		Example: `  # Verify all installed skills
  skills-oci verify

  # Verify in a specific project directory
  skills-oci verify --project-dir ./my-project`,
		RunE: runVerify,
	}

	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json and skills.lock.json")

	return cmd
}

func runVerify(cmd *cobra.Command, args []string) error {
	projectDir, _ := cmd.Flags().GetString("project-dir")
	plain, _ := cmd.Flags().GetBool("plain")

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

	// Build the list of verify options from the lock file.
	// Skills present in skills.json but absent from the lock are marked NotInstalled.
	opts := make([]oci.VerifyOptions, len(manifest.Skills))
	for i, s := range manifest.Skills {
		opts[i] = oci.VerifyOptions{Name: s.Name}
		if locked := skill.GetLockedSkill(lock, s.Name); locked != nil {
			opts[i].Ref = locked.Source.Ref
		}
	}

	if plain {
		return runVerifyPlain(opts)
	}

	m := tuiverify.NewModel(opts)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if fm, ok := finalModel.(tuiverify.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runVerifyPlain(opts []oci.VerifyOptions) error {
	if _, err := exec.LookPath("cosign"); err != nil {
		return fmt.Errorf("cosign not found in PATH — install it from https://docs.sigstore.dev/cosign/system_config/installation/")
	}

	ctx := context.Background()

	passed, failed, skipped := 0, 0, 0

	for _, o := range opts {
		fmt.Printf("\n  %s\n", o.Name)

		if o.Ref == "" {
			fmt.Println("    not installed — run 'skills-oci install' first")
			skipped++
			continue
		}

		r := oci.Verify(ctx, o)

		printCheck("Signature", r.SignatureVerified, r.SignatureOutput)
		printCheck("SLSA     ", r.SLSAVerified, r.SLSAOutput)

		if r.SignatureVerified && r.SLSAVerified {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("\n  %d passed, %d failed, %d skipped\n", passed, failed, skipped)

	if failed > 0 {
		return fmt.Errorf("%d skill(s) failed verification", failed)
	}
	return nil
}

func printCheck(label string, ok bool, output string) {
	icon := "✓"
	if !ok {
		icon = "✗"
	}
	line := fmt.Sprintf("    %s  %s", icon, label)
	if output != "" {
		line += "  " + output
	}
	fmt.Println(line)
}
