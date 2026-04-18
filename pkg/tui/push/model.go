package push

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/skill"
	"github.com/salaboy/skills-oci/pkg/tui"
	"github.com/salaboy/skills-oci/pkg/tui/components"
)

type phase int

const (
	phaseValidating phase = iota
	phaseParsing
	phaseArchiving
	phasePushing
	phaseDone
	phaseError
)

// Messages
type phaseMsg phase
type configMsg skill.SkillConfig
type pushResultMsg struct{ result *oci.PushResult }
type pushErrMsg struct{ err error }

// Model is the Bubble Tea model for the push workflow.
type Model struct {
	phase     phase
	spinner   spinner.Model
	ref       string
	skillDir  string
	plainHTTP bool
	config    *skill.SkillConfig
	result    *oci.PushResult
	err       error
}

// NewModel creates a new push TUI model.
func NewModel(ref, skillDir string, plainHTTP bool) Model {
	return Model{
		phase:     phaseValidating,
		spinner:   components.NewSpinner(),
		ref:       ref,
		skillDir:  skillDir,
		plainHTTP: plainHTTP,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.runPush(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case phaseMsg:
		m.phase = phase(msg)

	case configMsg:
		cfg := skill.SkillConfig(msg)
		m.config = &cfg

	case pushResultMsg:
		m.phase = phaseDone
		m.result = msg.result
		return m, tea.Quit

	case pushErrMsg:
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
	b.WriteString(tui.TitleStyle.Render("  Skills OCI — Push"))
	b.WriteString("\n\n")

	phases := []struct {
		name  string
		phase phase
	}{
		{"Validating skill directory", phaseValidating},
		{"Parsing SKILL.md", phaseParsing},
		{"Creating archive", phaseArchiving},
		{"Pushing to registry", phasePushing},
	}

	for _, p := range phases {
		if m.phase > p.phase {
			b.WriteString(fmt.Sprintf("  %s %s\n", tui.CheckMark, p.name))
		} else if m.phase == p.phase {
			b.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), p.name))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s\n", tui.MutedStyle.Render("○"), tui.MutedStyle.Render(p.name)))
		}
	}

	if m.config != nil {
		b.WriteString("\n")
		b.WriteString(tui.SubtitleStyle.Render("  Skill Details"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Name:        %s\n", m.config.Name))
		if m.config.Version != "" {
			b.WriteString(fmt.Sprintf("  Version:     %s\n", m.config.Version))
		}
		if m.config.Description != "" {
			b.WriteString(fmt.Sprintf("  Description: %s\n", m.config.Description))
		}
		if m.config.License != "" {
			b.WriteString(fmt.Sprintf("  License:     %s\n", m.config.License))
		}
	}

	if m.phase == phaseDone && m.result != nil {
		b.WriteString("\n")
		b.WriteString(tui.SuccessStyle.Render("  ✓ Successfully pushed!"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Reference: %s:%s\n", m.result.Reference, m.result.Tag))
		b.WriteString(fmt.Sprintf("  Digest:    %s\n", m.result.Digest))
		b.WriteString(fmt.Sprintf("  Size:      %d bytes\n", m.result.Size))
	}

	if m.phase == phaseError && m.err != nil {
		b.WriteString("\n")
		b.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("  ✗ Push failed: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// Err returns the error if the push failed.
func (m Model) Err() error {
	return m.err
}

// runPush executes the full push workflow, sending phase messages along the way.
func (m Model) runPush() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Validate
		if err := skill.Validate(m.skillDir); err != nil {
			return pushErrMsg{err: fmt.Errorf("validation failed: %w", err)}
		}

		// Parse — we do it here to send the config message,
		// but oci.Push will also parse internally.
		sd, err := skill.Parse(m.skillDir)
		if err != nil {
			return pushErrMsg{err: fmt.Errorf("parse failed: %w", err)}
		}

		// We can't send intermediate tea messages from a Cmd function directly.
		// The push runs as a single operation and we show the result.
		// Phase transitions happen based on the OnStatus callback.

		result, err := oci.Push(ctx, oci.PushOptions{
			Reference: m.ref,
			SkillDir:  m.skillDir,
			PlainHTTP: m.plainHTTP,
			OnStatus:  func(phase string) {},
		})
		if err != nil {
			return pushErrMsg{err: err}
		}

		_ = sd // parsed for validation; oci.Push parses internally
		return pushResultMsg{result: result}
	}
}
