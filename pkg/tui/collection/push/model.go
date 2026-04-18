package push

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
	phaseResolving phase = iota
	phaseBuilding
	phasePushing
	phaseDone
	phaseError
)

type pushResultMsg struct{ result *oci.PushCollectionResult }
type pushErrMsg struct{ err error }

// Model is the Bubble Tea model for the collection push workflow.
type Model struct {
	phase     phase
	spinner   spinner.Model
	ref       string
	name      string
	skillRefs []string
	plainHTTP bool
	result    *oci.PushCollectionResult
	err       error
}

// NewModel creates a new collection push TUI model.
func NewModel(ref, name string, skillRefs []string, plainHTTP bool) Model {
	return Model{
		phase:     phaseResolving,
		spinner:   components.NewSpinner(),
		ref:       ref,
		name:      name,
		skillRefs: skillRefs,
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
	b.WriteString(tui.TitleStyle.Render("  Skills OCI — Collection Push"))
	b.WriteString("\n\n")

	phases := []struct {
		name  string
		phase phase
	}{
		{fmt.Sprintf("Resolving %d skill reference(s)", len(m.skillRefs)), phaseResolving},
		{"Building collection index", phaseBuilding},
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

	if m.phase == phaseDone && m.result != nil {
		b.WriteString("\n")
		b.WriteString(tui.SuccessStyle.Render("  ✓ Collection pushed successfully!"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Collection: %s\n", m.name))
		b.WriteString(fmt.Sprintf("  Reference:  %s:%s\n", m.result.Reference, m.result.Tag))
		b.WriteString(fmt.Sprintf("  Digest:     %s\n", m.result.Digest))
		b.WriteString(fmt.Sprintf("  Skills:     %d\n", m.result.Skills))
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

func (m Model) runPush() tea.Cmd {
	return func() tea.Msg {
		result, err := oci.PushCollection(context.Background(), oci.PushCollectionOptions{
			Reference: m.ref,
			Name:      m.name,
			SkillRefs: m.skillRefs,
			PlainHTTP: m.plainHTTP,
			OnStatus:  func(phase string) {},
		})
		if err != nil {
			return pushErrMsg{err: err}
		}
		return pushResultMsg{result: result}
	}
}
