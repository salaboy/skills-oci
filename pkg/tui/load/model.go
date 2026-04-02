package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-oci/pkg/oci"
	"github.com/salaboy/skills-oci/pkg/skill"
	"github.com/salaboy/skills-oci/pkg/tui"
	"github.com/salaboy/skills-oci/pkg/tui/components"
)

type phase int

const (
	phaseInit phase = iota
	phaseReading
	phasePulling
	phaseDone
	phaseError
)

type loadResultMsg struct {
	installed []string
	skipped   []string
}
type loadErrMsg struct{ err error }

// Model is the Bubble Tea model for the load workflow.
type Model struct {
	phase      phase
	spinner    spinner.Model
	projectDir string
	skillsDir  string
	plainHTTP  bool
	installed  []string
	skipped    []string
	err        error
	status     string
}

// NewModel creates a new load TUI model.
func NewModel(projectDir, skillsDir string, plainHTTP bool) Model {
	if projectDir == "" {
		projectDir = "."
	}
	return Model{
		phase:      phaseInit,
		spinner:    components.NewSpinner(),
		projectDir: projectDir,
		skillsDir:  skillsDir,
		plainHTTP:  plainHTTP,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startLoad(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case loadResultMsg:
		m.phase = phaseDone
		m.installed = msg.installed
		m.skipped = msg.skipped
		return m, tea.Quit

	case loadErrMsg:
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
	b.WriteString(tui.TitleStyle.Render("  Skills OCI — Load"))
	b.WriteString("\n\n")

	phases := []struct {
		name  string
		phase phase
	}{
		{"Reading skills.json", phaseReading},
		{"Pulling missing skills", phasePulling},
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

	if m.phase == phaseDone {
		b.WriteString("\n")
		if len(m.installed) > 0 {
			b.WriteString(tui.SuccessStyle.Render("  ✓ Installed:"))
			b.WriteString("\n")
			for _, name := range m.installed {
				b.WriteString(fmt.Sprintf("    • %s\n", name))
			}
		}
		if len(m.skipped) > 0 {
			b.WriteString(tui.MutedStyle.Render("  ○ Already present:"))
			b.WriteString("\n")
			for _, name := range m.skipped {
				b.WriteString(fmt.Sprintf("    • %s\n", tui.MutedStyle.Render(name)))
			}
		}
		if len(m.installed) == 0 && len(m.skipped) == 0 {
			b.WriteString(tui.MutedStyle.Render("  No skills defined in skills.json"))
			b.WriteString("\n")
		}
	}

	if m.phase == phaseError && m.err != nil {
		b.WriteString("\n")
		b.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("  ✗ Load failed: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// Err returns the error if the load failed.
func (m Model) Err() error {
	return m.err
}

func (m Model) startLoad() tea.Cmd {
	return func() tea.Msg {
		installed, skipped, err := LoadSkills(m.projectDir, m.skillsDir, m.plainHTTP, nil)
		if err != nil {
			return loadErrMsg{err: err}
		}
		return loadResultMsg{installed: installed, skipped: skipped}
	}
}

// LoadSkills reads skills.json and pulls any skills whose directories are missing.
// Returns the list of installed and skipped skill names.
func LoadSkills(projectDir, skillsDir string, plainHTTP bool, onStatus func(string)) (installed, skipped []string, err error) {
	manifest, err := skill.LoadManifest(projectDir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading skills.json: %w", err)
	}

	if len(manifest.Skills) == 0 {
		return nil, nil, nil
	}

	outputDir := filepath.Join(projectDir, skillsDir)

	for _, s := range manifest.Skills {
		skillDir := filepath.Join(outputDir, s.Name)
		if _, err := os.Stat(skillDir); err == nil {
			skipped = append(skipped, s.Name)
			continue
		}

		ref := s.Source
		if s.Version != "" {
			ref += ":" + s.Version
		}

		if onStatus != nil {
			onStatus(fmt.Sprintf("Pulling %s", s.Name))
		}

		result, err := oci.Pull(context.Background(), oci.PullOptions{
			Reference: ref,
			OutputDir: outputDir,
			PlainHTTP: plainHTTP,
			OnStatus:  func(phase string) {},
		})
		if err != nil {
			return installed, skipped, fmt.Errorf("pulling %s: %w", s.Name, err)
		}

		// Update lock file with pulled skill metadata
		l, err := skill.LoadLock(projectDir)
		if err != nil {
			return installed, skipped, fmt.Errorf("loading skills.lock.json: %w", err)
		}

		entry := skill.LockedSkill{
			Name: result.Name,
			Path: filepath.Join(skillsDir, result.Name),
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
		if err := skill.SaveLock(projectDir, l); err != nil {
			return installed, skipped, fmt.Errorf("saving skills.lock.json: %w", err)
		}

		installed = append(installed, s.Name)
	}

	return installed, skipped, nil
}
