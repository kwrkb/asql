package ui

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/profile"
)

func (m model) updateProfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.profileNaming {
		return m.updateProfileNaming(msg)
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.textarea.Blur()
		m.setStatus("Normal mode", false)
		return m, nil
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "j":
			if len(m.profiles) > 0 && m.profileCursor < len(m.profiles)-1 {
				m.profileCursor++
			}
		case "k":
			if m.profileCursor > 0 {
				m.profileCursor--
			}
		case "d":
			if len(m.profiles) > 0 {
				newProfiles := append(m.profiles[:m.profileCursor], m.profiles[m.profileCursor+1:]...)
				if err := profile.Save(newProfiles); err != nil {
					m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
				} else {
					m.profiles = newProfiles
					if m.profileCursor >= len(m.profiles) && m.profileCursor > 0 {
						m.profileCursor--
					}
					m.setStatus("Profile deleted", false)
				}
			}
		case "a":
			if m.rawDSN == "" {
				m.setStatus("No active connection to save", true)
				return m, nil
			}
			m.profileNaming = true
			m.profileInput.Reset()
			m.profileInput.Focus()
			return m, textinput.Blink
		}
	case tea.KeyDown:
		if len(m.profiles) > 0 && m.profileCursor < len(m.profiles)-1 {
			m.profileCursor++
		}
	case tea.KeyUp:
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case tea.KeyEnter:
		if len(m.profiles) > 0 {
			dsn := m.profiles[m.profileCursor].DSN
			maskedDSN := maskDSNForUI(dsn)
			m.setStatus(fmt.Sprintf("Profile DSN: %s (use @%s to connect)", maskedDSN, m.profiles[m.profileCursor].Name), false)
			m.mode = normalMode
			m.textarea.Blur()
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateProfileNaming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.profileNaming = false
		m.profileInput.Blur()
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.profileInput.Value())
		if name == "" {
			return m, nil
		}
		// Remove existing profile with same name
		var newProfiles []profile.Profile
		for _, p := range m.profiles {
			if p.Name != name {
				newProfiles = append(newProfiles, p)
			}
		}
		newProfiles = append(newProfiles, profile.Profile{Name: name, DSN: m.rawDSN})
		if err := profile.Save(newProfiles); err != nil {
			m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
			m.profileNaming = false
			m.profileInput.Blur()
			return m, nil
		}
		m.profiles = newProfiles
		m.setStatus(fmt.Sprintf("Saved profile: %s", name), false)
		m.profileNaming = false
		m.profileInput.Blur()
		m.profileCursor = len(m.profiles) - 1
		return m, nil
	}

	var cmd tea.Cmd
	m.profileInput, cmd = m.profileInput.Update(msg)
	return m, cmd
}

var rePasswordInDSNUI = regexp.MustCompile(`(://[^:]*:)([^@]*)(@)`)

func maskDSNForUI(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return rePasswordInDSNUI.ReplaceAllString(dsn, "${1}***${3}")
	}
	masked := false
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
			masked = true
		}
	}
	q := u.Query()
	if q.Get("password") != "" {
		q.Set("password", "***")
		u.RawQuery = q.Encode()
		masked = true
	}
	if !masked {
		return dsn
	}
	return u.String()
}

func (m model) renderWithProfileOverlay(background string) string {
	modalWidth := min(m.width-4, 60)
	if modalWidth < 20 {
		modalWidth = 20
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		MarginBottom(1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(modalWidth).
		Background(panelBackground)

	itemStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(panelBackground).
		Width(modalWidth - 6).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(panelBackground).
		Background(accentColor).
		Bold(true).
		Width(modalWidth - 6).
		Padding(0, 1)

	dsnPreviewStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor).
		Background(panelBackground).
		Width(modalWidth - 6).
		Padding(0, 1)

	var items strings.Builder

	if m.profileNaming {
		items.WriteString(lipgloss.NewStyle().Foreground(textColor).Background(panelBackground).Render("Name: "))
		items.WriteString(m.profileInput.View())
	} else if len(m.profiles) == 0 {
		items.WriteString(lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("(no saved profiles)"))
	} else {
		maxVisible := max(min((m.height-8)/2, len(m.profiles)), 1)
		start := 0
		if m.profileCursor >= maxVisible {
			start = m.profileCursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.profiles))

		for i := start; i < end; i++ {
			p := m.profiles[i]
			label := sanitize(p.Name)
			if i == m.profileCursor {
				items.WriteString(selectedStyle.Render(label))
			} else {
				items.WriteString(itemStyle.Render(label))
			}
			// Show masked DSN preview
			preview := maskDSNForUI(p.DSN)
			maxPreview := modalWidth - 10
			runes := []rune(preview)
			if maxPreview > 0 && len(runes) > maxPreview {
				preview = string(runes[:maxPreview]) + "..."
			}
			items.WriteByte('\n')
			items.WriteString(dsnPreviewStyle.Render(preview))
			if i < end-1 {
				items.WriteByte('\n')
			}
		}
	}

	title := "Connection Profiles"
	if m.profileNaming {
		title = "Save Profile"
	}

	var footer string
	if !m.profileNaming {
		footer = "\n" + lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("Enter:show d:delete a:add Esc:close")
	}

	content := titleStyle.Render(title) + "\n" + items.String() + footer
	modal := boxStyle.Render(content)

	bgH := lipgloss.Height(background)

	return lipgloss.Place(m.width, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}
