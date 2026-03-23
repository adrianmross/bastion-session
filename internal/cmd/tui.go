package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type tuiItem struct {
	id      string
	title   string
	detail  string
	source  string
	tracked app.TrackedBastion
	scoped  app.BastionInfo
}

func (i tuiItem) Title() string       { return i.title }
func (i tuiItem) Description() string { return i.detail }
func (i tuiItem) FilterValue() string { return i.title + " " + i.id }

type tuiMode string

const (
	modeScoped  tuiMode = "scoped"
	modeTracked tuiMode = "tracked"
)

type reloadMsg struct {
	mode    tuiMode
	items   []list.Item
	status  string
	loadErr error
}

type tuiModel struct {
	cfg      app.Config
	list     list.Model
	mode     tuiMode
	status   string
	selected string
	width    int
	height   int
}

var (
	bannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("33")).Padding(0, 1)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

func newTUICmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive bastion picker scoped to oci-context with tracked escape view",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := loadScopedItems(opts.cfg)
			if err != nil {
				items = []list.Item{}
			}
			l := list.New(items, list.NewDefaultDelegate(), 0, 0)
			l.Title = "Bastions"
			l.SetShowStatusBar(false)
			l.SetFilteringEnabled(true)
			l.SetShowHelp(false)
			l.SetShowTitle(false)

			m := tuiModel{
				cfg:    opts.cfg,
				list:   l,
				mode:   modeScoped,
				status: "Enter: select  /: filter  e: tracked view  s: scoped view  r: refresh  q: quit",
			}
			if err != nil {
				m.status = "Failed to load scoped bastions: " + err.Error()
			}
			p := tea.NewProgram(m)
			result, err := p.Run()
			if err != nil {
				return err
			}
			fm := result.(tuiModel)
			if fm.selected != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", fm.selected)
			}
			return nil
		},
	}
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-2, msg.Height-8)
		return m, nil
	case reloadMsg:
		if msg.mode == m.mode {
			m.list.SetItems(msg.items)
			if msg.loadErr != nil {
				m.status = msg.status + ": " + msg.loadErr.Error()
			} else {
				m.status = msg.status
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "e":
			m.mode = modeTracked
			m.status = "Tracked bastions mode (press s to return to scoped context view)"
			return m, reloadModeCmd(m.cfg, m.mode)
		case "s":
			m.mode = modeScoped
			m.status = "Scoped bastions mode"
			return m, reloadModeCmd(m.cfg, m.mode)
		case "r":
			return m, reloadModeCmd(m.cfg, m.mode)
		case "enter":
			if it, ok := m.list.SelectedItem().(tuiItem); ok {
				m.selected = it.id
				_ = app.UpsertTracked(m.cfg.TrackedBastionsPath, app.TrackedBastion{
					ID:   it.id,
					Name: it.title,
					CompartmentID: func() string {
						if it.source == "tracked" {
							return it.tracked.CompartmentID
						}
						return it.scoped.CompartmentID
					}(),
					Region: func() string {
						if it.source == "tracked" {
							return it.tracked.Region
						}
						return it.scoped.Region
					}(),
					Profile: func() string {
						if it.source == "tracked" {
							return it.tracked.Profile
						}
						return it.scoped.Profile
					}(),
					SSHPublicKey: func() string {
						if it.source == "tracked" {
							return it.tracked.SSHPublicKey
						}
						return m.cfg.SSHPublicKey
					}(),
					ContextName: func() string {
						if it.source == "tracked" {
							return it.tracked.ContextName
						}
						return it.scoped.ScopeContext
					}(),
					LastSeenAt: time.Now().UTC(),
				})
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string {
	contextText := "none"
	if m.cfg.ScopedContext != nil {
		contextText = fmt.Sprintf("%s (profile=%s region=%s compartment=%s)",
			m.cfg.ScopedContext.Name,
			m.cfg.ScopedContext.Profile,
			m.cfg.ScopedContext.Region,
			shorten(m.cfg.ScopedContext.CompartmentOCID),
		)
	}
	banner := bannerStyle.Render("oci-context scope: " + contextText)
	if m.mode == modeTracked {
		banner = warnStyle.Render("escape mode: showing tracked bastions (press s for scoped view)")
	}
	header := lipgloss.JoinVertical(lipgloss.Left,
		banner,
		mutedStyle.Render("Mode: "+string(m.mode)+" | Enter selects bastion ID"),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		m.list.View(),
		"",
		mutedStyle.Render(m.status),
	)
}

func reloadModeCmd(cfg app.Config, mode tuiMode) tea.Cmd {
	return func() tea.Msg {
		if mode == modeTracked {
			items, err := loadTrackedItems(cfg)
			return reloadMsg{mode: mode, items: items, status: fmt.Sprintf("Loaded %d tracked bastions", len(items)), loadErr: err}
		}
		items, err := loadScopedItems(cfg)
		return reloadMsg{mode: mode, items: items, status: fmt.Sprintf("Loaded %d scoped bastions", len(items)), loadErr: err}
	}
}

func loadScopedItems(cfg app.Config) ([]list.Item, error) {
	bastions, err := app.ListScopedBastions(cfg)
	if err != nil {
		return nil, err
	}
	items := make([]list.Item, 0, len(bastions))
	for _, b := range bastions {
		title := b.Name
		if strings.TrimSpace(title) == "" {
			title = b.ID
		}
		detail := fmt.Sprintf("id=%s  lifecycle=%s  compartment=%s", b.ID, b.LifecycleState, shorten(b.CompartmentID))
		items = append(items, tuiItem{id: b.ID, title: title, detail: detail, source: "scoped", scoped: b})
	}
	return items, nil
}

func loadTrackedItems(cfg app.Config) ([]list.Item, error) {
	tracked, err := app.LoadTracked(cfg.TrackedBastionsPath)
	if err != nil {
		return nil, err
	}
	items := make([]list.Item, 0, len(tracked))
	for _, b := range tracked {
		title := b.Name
		if strings.TrimSpace(title) == "" {
			title = b.ID
		}
		detail := fmt.Sprintf("id=%s  context=%s  last_seen=%s", b.ID, emptyDash(b.ContextName), b.LastSeenAt.Format(time.RFC3339))
		items = append(items, tuiItem{id: b.ID, title: title, detail: detail, source: "tracked", tracked: b})
	}
	return items, nil
}

func shorten(s string) string {
	if len(s) <= 14 {
		return s
	}
	return s[:6] + "..." + s[len(s)-6:]
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
