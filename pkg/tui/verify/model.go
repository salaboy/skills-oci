package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/tui"
	"github.com/salaboy/skills-oci/pkg/tui/components"
)

type phase int

const (
	phaseVerifying phase = iota
	phaseDone
	phaseError
)

type verifyDoneMsg struct{ results []oci.VerifyResult }
type verifyErrMsg struct{ err error }

// Model is the Bubble Tea model for the verify workflow.
type Model struct {
	phase   phase
	spinner spinner.Model
	skills  []oci.VerifyOptions
	results []oci.VerifyResult
	err     error
}

// NewModel creates a new verify TUI model.
func NewModel(skills []oci.VerifyOptions) Model {
	return Model{
		phase:   phaseVerifying,
		spinner: components.NewSpinner(),
		skills:  skills,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.runVerify(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case verifyDoneMsg:
		m.phase = phaseDone
		m.results = msg.results
		return m, tea.Quit

	case verifyErrMsg:
		m.phase = phaseError
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(tui.TitleStyle.Render("  Skills OCI — Verify"))
	b.WriteString("\n\n")

	if m.phase == phaseVerifying {
		b.WriteString(fmt.Sprintf("  %s Verifying %d skill(s) with cosign...\n", m.spinner.View(), len(m.skills)))
		b.WriteString("\n")
		return b.String()
	}

	if m.phase == phaseError {
		b.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("  ✗ Verification failed: %s", m.err.Error())))
		b.WriteString("\n\n")
		return b.String()
	}

	if len(m.results) == 0 {
		b.WriteString(tui.MutedStyle.Render("  No skills to verify."))
		b.WriteString("\n\n")
		return b.String()
	}

	// Report header
	b.WriteString(tui.SubtitleStyle.Render("  Verification Report"))
	b.WriteString("\n\n")

	passed, failed, skipped := 0, 0, 0

	for _, r := range m.results {
		b.WriteString(fmt.Sprintf("  %s  %s\n", skillStatusIcon(r), tui.SubtitleStyle.Render(r.Name)))

		if r.CosignMissing {
			b.WriteString(tui.ErrorStyle.Render("      cosign not found — install cosign to enable verification"))
			b.WriteString("\n")
			skipped++
			continue
		}

		if r.NotInstalled {
			b.WriteString(tui.MutedStyle.Render("      not installed — run 'skills-oci install' first"))
			b.WriteString("\n")
			skipped++
			continue
		}

		b.WriteString(fmt.Sprintf("      %-10s  %s  %s\n",
			"Signature",
			checkOrCross(r.SignatureVerified),
			detail(r.SignatureVerified, r.SignatureOutput),
		))
		b.WriteString(fmt.Sprintf("      %-10s  %s  %s\n",
			"SLSA",
			checkOrCross(r.SLSAVerified),
			detail(r.SLSAVerified, r.SLSAOutput),
		))

		if r.SignatureVerified && r.SLSAVerified {
			passed++
		} else {
			failed++
		}

		b.WriteString("\n")
	}

	// Summary line
	b.WriteString(tui.MutedStyle.Render(fmt.Sprintf("  ─────────────────────────────────────────────")))
	b.WriteString("\n")

	summary := fmt.Sprintf("  %d passed", passed)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}
	if skipped > 0 {
		summary += fmt.Sprintf(", %d skipped", skipped)
	}

	if failed > 0 {
		b.WriteString(tui.ErrorStyle.Render(summary))
	} else if passed > 0 {
		b.WriteString(tui.SuccessStyle.Render(summary))
	} else {
		b.WriteString(tui.MutedStyle.Render(summary))
	}
	b.WriteString("\n\n")

	return b.String()
}

// Err returns the error if verification setup failed.
func (m Model) Err() error {
	return m.err
}

// Results returns the verification results.
func (m Model) Results() []oci.VerifyResult {
	return m.results
}

func (m Model) runVerify() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results := make([]oci.VerifyResult, len(m.skills))
		for i, s := range m.skills {
			results[i] = oci.Verify(ctx, s)
		}
		return verifyDoneMsg{results: results}
	}
}

// skillStatusIcon returns a composite icon based on both checks.
func skillStatusIcon(r oci.VerifyResult) string {
	if r.CosignMissing || r.NotInstalled {
		return tui.MutedStyle.Render("○")
	}
	if r.SignatureVerified && r.SLSAVerified {
		return tui.CheckMark
	}
	return tui.CrossMark
}

func checkOrCross(ok bool) string {
	if ok {
		return tui.CheckMark
	}
	return tui.CrossMark
}

func detail(ok bool, output string) string {
	if output == "" {
		return ""
	}
	if ok {
		return tui.MutedStyle.Render(output)
	}
	return tui.ErrorStyle.Render(output)
}
