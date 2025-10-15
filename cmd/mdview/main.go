package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const helpText = "(j/k: scroll, Ctrl+D/U: half page, q: quit)"

type model struct {
	vp       viewport.Model
	content  string
	ready    bool
	help     string
	renderer *glamour.TermRenderer
}

func initialModel(md string) model {
	// Glamour: ターミナル向けMarkdownレンダラ（テーマはまず "dark"）
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0), // ビューポート幅に合わせる（後で動的更新）
	)
	if err != nil {
		log.Fatal(err)
	}

	return model{
		vp:       viewport.Model{},
		content:  md,
		help:     helpText,
		renderer: r,
	}

}

func (m model) Init() tea.Cmd { return nil }

func (m model) View() string {
	border := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	header := lipgloss.NewStyle().Bold(true).Render("mdview")
	return lipgloss.JoinVertical(lipgloss.Left,
		header+"  "+m.help,

		border.Render(m.vp.View()),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {

			m.vp = viewport.New(msg.Width, msg.Height-2) // header分 -2 くらい
			m.ready = true
			// 初回レンダ
			out, _ := m.renderer.Render(m.content)
			m.vp.SetContent(out)
			return m, nil
		}
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 2
		// 折返しを更新するためレンダラ作り直し（簡便実装）
		r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(m.vp.Width-4))
		m.renderer = r
		out, _ := m.renderer.Render(m.content)
		m.vp.SetContent(out)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j":
			m.vp.LineDown(1)
		case "k":
			m.vp.LineUp(1)
		case "ctrl+d":
			m.vp.HalfViewDown()
		case "ctrl+u":

			m.vp.HalfViewUp()
		}
	}
	// viewportにキーを渡して慣性スクロールなども利用（必要最小限）
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mdview <markdown-file>")
		os.Exit(1)
	}
	path := os.Args[1]
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	// ざっくり: 先頭にファイル名を表示（軽いヘッダ）
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# %s\n\n", path)
	buf.Write(b)

	p := tea.NewProgram(initialModel(buf.String()), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
