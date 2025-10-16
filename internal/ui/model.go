package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	styles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/kyaoi/mdview/internal/tree"
)

const (
	headerHeight      = 0
	minContentWidth   = 20
	minTreePanelWidth = 18
	defaultTreeWidth  = 28
)

var (
	treeBlurBorderColor  = lipgloss.Color("#3b4261")
	treeFocusBorderColor = lipgloss.Color("#7aa2f7")
	treeLineStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#a9b1d6"))
	treeSelectedActive   = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1a1b26")).
				Background(lipgloss.Color("#7aa2f7")).
				Bold(true)
	treeSelectedInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#c0caf5")).
				Background(lipgloss.Color("#283457"))
	helpBoxStyle = lipgloss.NewStyle().
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7aa2f7")).
			Background(lipgloss.Color("#1f2335"))
)

// Model implements the Bubble Tea program for the markdown viewer.
type Model struct {
	contentVP          viewport.Model
	treeVP             viewport.Model
	renderer           *glamour.TermRenderer
	rawContent         string
	headerPath         string
	treeVisible        bool
	treePreferredWidth int
	treeContentWidth   int
	treeFocus          bool
	showHelp           bool
	pendingKey         string
	ready              bool
	width              int
	height             int
	err                error

	treeRoot      *tree.Node
	flatTree      []treeLine
	treeSelection int
	rootDir       string
	displayRoot   string
	activeAbsPath string
}

type treeLine struct {
	entry *tree.Node
	label string
}

// NewModel constructs the viewer model with the provided initial state.
func NewModel(state State) *Model {
	contentVP := viewport.New(0, 0)
	contentVP.Style = lipgloss.NewStyle().Padding(0, 1)
	contentVP.SetHorizontalStep(2)

	treeVP := viewport.New(0, 0)
	treeVP.Style = treePanelStyle(treeBlurBorderColor)
	treeVP.MouseWheelEnabled = false

	m := &Model{
		contentVP:          contentVP,
		treeVP:             treeVP,
		rawContent:         state.RawContent,
		headerPath:         state.HeaderPath,
		treeVisible:        state.TreeVisible && state.TreeRoot != nil,
		treePreferredWidth: state.TreePreferredWidth,
		treeRoot:           state.TreeRoot,
		rootDir:            state.RootDir,
		displayRoot:        state.DisplayRoot,
		activeAbsPath:      state.ActiveAbsPath,
	}

	if m.treeRoot != nil {
		m.refreshTreeViewWithSelection(state.TreeSelectionPath)
	}
	m.updateTreePanelStyle()

	if state.FocusTree {
		m.focusTree()
	}

	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// View implements tea.Model.
func (m *Model) View() string {
	body := m.contentVP.View()
	if m.treeVisible {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.treeVP.View(), body)
	}

	if m.err != nil {
		errLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")).Render(m.err.Error())
		body = lipgloss.JoinVertical(lipgloss.Left, errLine, body)
	}

	if m.showHelp {
		helpContent := strings.Join([]string{
			"ヘルプ (?:閉じる / Esc)",
			"Ctrl+h / Ctrl+l : ツリー↔本文フォーカス切替",
			"j / k            : 選択/スクロール (フォーカス中のペイン)",
			"Ctrl+d / Ctrl+u : 半ページ移動 (本文フォーカス時)",
			"gg / G           : 先頭 / 末尾へ移動",
			"h / l            : ツリー開閉・水平スクロール",
			"Enter / l        : ツリーでファイルを開く",
			"t                : ツリー表示のトグル",
			"q / Ctrl+c       : 終了",
		}, "\n")
		helpOverlay := helpBoxStyle.Render(helpContent)
		if m.width > 0 && m.height > 0 {
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpOverlay)
		}
		return helpOverlay
	}

	return body
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if key != "g" {
			m.pendingKey = ""
		}

		if m.showHelp {
			m.pendingKey = ""
			switch key {
			case "q", "?", "esc":
				m.showHelp = false
			}
			return m, nil
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = true
			m.pendingKey = ""
			return m, nil
		case "ctrl+h":
			if m.treeVisible {
				m.focusTree()
			}
			return m, nil
		case "ctrl+l":
			m.blurTree()
			return m, nil
		case "t":
			if m.treeRoot != nil {
				m.treeVisible = !m.treeVisible
				if !m.treeVisible {
					m.blurTree()
				}
				m.resize(m.width, m.height)
			}
			return m, nil
		}

		if m.treeFocus && m.treeVisible {
			if m.handleTreeKey(key) {
				return m, nil
			}
			return m, nil
		}

		handled := m.handleContentKey(key)
		if handled {
			return m, nil
		}

		var cmd tea.Cmd
		m.contentVP, cmd = m.contentVP.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.contentVP, cmd = m.contentVP.Update(msg)
	return m, cmd
}

func (m *Model) handleContentKey(key string) bool {
	switch key {
	case "j":
		m.contentVP.ScrollDown(1)
	case "k":
		m.contentVP.ScrollUp(1)
	case "ctrl+d":
		m.contentVP.HalfPageDown()
	case "ctrl+u":
		m.contentVP.HalfPageUp()
	case "h":
		m.contentVP.ScrollLeft(max(2, m.contentVP.Width/6))
	case "l":
		m.contentVP.ScrollRight(max(2, m.contentVP.Width/6))
	case "g":
		if m.pendingKey == "g" {
			m.contentVP.GotoTop()
			m.pendingKey = ""
		} else {
			m.pendingKey = "g"
		}
		return true
	case "G":
		m.pendingKey = ""
		m.contentVP.GotoBottom()
	default:
		return false
	}
	m.pendingKey = ""
	return true
}

func (m *Model) handleTreeKey(key string) bool {
	if m.treeRoot == nil {
		return false
	}
	switch key {
	case "j":
		m.moveTreeSelection(1)
		return true
	case "k":
		m.moveTreeSelection(-1)
		return true
	case "ctrl+j":
		m.contentVP.ScrollDown(1)
	case "ctrl+k":
		m.contentVP.ScrollUp(1)
	case "l", "right":
		m.openOrDescend()
		return true
	case "h", "left":
		m.closeOrAscend()
		return true
	case "enter":
		m.openOrDescend()
		return true
	case "g":
		if m.pendingKey == "g" {
			if len(m.flatTree) > 0 {
				m.treeSelection = 0
				m.pendingKey = ""
				m.updateTreeContent(m.treeContentWidth)
				m.ensureSelectionVisible()
			}
		} else {
			m.pendingKey = "g"
		}
		return true
	case "G":
		m.pendingKey = ""
		if len(m.flatTree) > 0 {
			m.treeSelection = len(m.flatTree) - 1
			m.updateTreeContent(m.treeContentWidth)
			m.ensureSelectionVisible()
		}
		return true
	}
	m.pendingKey = ""
	return false
}

func (m *Model) resize(width, height int) {
	if width <= 0 || height <= headerHeight {
		return
	}

	m.width = width
	m.height = height
	m.ready = true

	treeWidth := m.treeWidth(width)
	contentWidth := width - treeWidth
	if m.treeVisible && treeWidth > 0 {
		contentWidth--
	}
	if contentWidth < minContentWidth {
		contentWidth = minContentWidth
	}

	contentHeight := max(height-headerHeight, 1)
	m.contentVP.Width = contentWidth
	m.contentVP.Height = contentHeight

	wrapWidth := contentWidth - m.contentVP.Style.GetHorizontalFrameSize()
	if wrapWidth < 0 {
		wrapWidth = 0
	}

	renderer, err := newRenderer(wrapWidth)
	if err != nil {
		m.err = err
		return
	}
	m.renderer = renderer

	rendered, err := m.renderer.Render(m.rawContent)
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.contentVP.SetContent(rendered)

	if m.treeVisible && treeWidth > 0 {
		m.treeVP.Width = treeWidth
		m.treeVP.Height = contentHeight
		m.ensureSelectionVisible()
	} else {
		m.treeVP.Width = 0
		m.treeVP.Height = contentHeight
	}
}

func (m *Model) treeWidth(totalWidth int) int {
	if !m.treeVisible {
		return 0
	}
	preferred := m.treePreferredWidth
	if preferred <= 0 {
		preferred = defaultTreeWidth
	}

	frame := m.treeVP.Style.GetHorizontalFrameSize()
	minPanel := max(minTreePanelWidth-frame, 0)
	maxPanel := max(totalWidth/2-frame, minPanel)
	panelContentWidth := clamp(preferred, minPanel, maxPanel)

	width := panelContentWidth + frame
	if totalWidth-width < minContentWidth {
		width = max(totalWidth-minContentWidth, 0)
	}
	if width > totalWidth {
		width = totalWidth
	}
	return width
}

func (m *Model) moveTreeSelection(delta int) {
	if len(m.flatTree) == 0 {
		return
	}
	m.treeSelection = clamp(m.treeSelection+delta, 0, len(m.flatTree)-1)
	m.updateTreeContent(m.treeContentWidth)
}

func (m *Model) openOrDescend() {
	entry := m.currentTreeEntry()
	if entry == nil {
		return
	}
	if entry.IsDir {
		if !entry.Open {
			entry.Open = true
			if !m.loadNode(entry) {
				return
			}
			m.refreshTreeViewWithSelection(entry.Path)
			return
		}
		if !m.loadNode(entry) {
			return
		}
		if len(entry.Children) > 0 {
			m.moveTreeSelection(1)
		}
		return
	}
	m.openFileEntry(entry)
}

func (m *Model) closeOrAscend() {
	entry := m.currentTreeEntry()
	if entry == nil {
		return
	}
	if entry.IsDir && entry.Open {
		entry.Open = false
		maxWidth := m.rebuildFlatTree()
		if idx := m.indexForPath(entry.Path); idx >= 0 {
			m.treeSelection = idx
		} else {
			m.treeSelection = clamp(m.treeSelection, 0, len(m.flatTree)-1)
		}
		m.treeContentWidth = maxWidth
		m.updateTreeContent(maxWidth)
		return
	}
	if entry.Parent != nil {
		m.refreshTreeViewWithSelection(entry.Parent.Path)
	}
}

func (m *Model) currentTreeEntry() *tree.Node {
	if len(m.flatTree) == 0 || m.treeSelection < 0 || m.treeSelection >= len(m.flatTree) {
		return nil
	}
	return m.flatTree[m.treeSelection].entry
}

func (m *Model) openFileEntry(entry *tree.Node) {
	if m.rootDir == "" {
		return
	}
	absPath := filepath.Join(m.rootDir, filepath.FromSlash(entry.Path))
	data, err := os.ReadFile(absPath)
	if err != nil {
		m.err = err
		return
	}
	m.rawContent = string(data)
	m.activeAbsPath = absPath
	m.headerPath = composeDisplayPath(m.displayRoot, entry.Path)
	m.renderMarkdown()
	m.contentVP.GotoTop()
}

func (m *Model) renderMarkdown() {
	if m.renderer == nil {
		return
	}
	rendered, err := m.renderer.Render(m.rawContent)
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.contentVP.SetContent(rendered)
}

func (m *Model) refreshTreeViewWithSelection(path string) {
	if m.treeRoot == nil {
		return
	}
	if !m.loadNode(m.treeRoot) {
		return
	}
	m.expandPath(path)
	maxWidth := m.rebuildFlatTree()
	if len(m.flatTree) > 0 {
		if idx := m.indexForPath(path); idx >= 0 {
			m.treeSelection = idx
		} else {
			m.treeSelection = clamp(m.treeSelection, 0, len(m.flatTree)-1)
		}
	} else {
		m.treeSelection = 0
	}
	m.treeContentWidth = maxWidth
	m.updateTreeContent(maxWidth)
}

func (m *Model) expandPath(path string) {
	if m.treeRoot == nil || path == "" {
		return
	}
	if !m.treeRoot.Open {
		m.treeRoot.Open = true
	}
	parts := strings.Split(path, "/")
	current := m.treeRoot
	for _, part := range parts {
		if !m.loadNode(current) {
			return
		}
		child := current.ChildByName(part)
		if child == nil {
			return
		}
		if child.IsDir {
			child.Open = true
		}
		current = child
	}
}

func (m *Model) rebuildFlatTree() int {
	if m.treeRoot == nil {
		m.flatTree = nil
		return 0
	}
	var lines []treeLine
	maxWidth := 0
	var walk func(*tree.Node, int)
	walk = func(node *tree.Node, depth int) {
		label := formatTreeLabel(node, depth)
		if w := lipgloss.Width(label); w > maxWidth {
			maxWidth = w
		}
		lines = append(lines, treeLine{entry: node, label: label})
		if node.IsDir && node.Open {
			if !m.loadNode(node) {
				return
			}
			for _, child := range node.Children {
				walk(child, depth+1)
			}
		}
	}
	walk(m.treeRoot, 0)
	m.flatTree = lines
	return maxWidth
}

func (m *Model) updateTreeContent(width int) {
	if m.treeRoot == nil {
		return
	}
	if width <= 0 {
		width = minTreePanelWidth
	}
	var builder strings.Builder
	for i, line := range m.flatTree {
		text := line.label
		switch {
		case i == m.treeSelection && m.treeFocus:
			builder.WriteString(treeSelectedActive.Render(text))
		case i == m.treeSelection:
			builder.WriteString(treeSelectedInactive.Render(text))
		default:
			builder.WriteString(treeLineStyle.Render(text))
		}
		if i < len(m.flatTree)-1 {
			builder.WriteByte('\n')
		}
	}
	m.treePreferredWidth = max(width+4, minTreePanelWidth)
	m.treeVP.SetContent(builder.String())
	m.ensureSelectionVisible()
}

func (m *Model) indexForPath(path string) int {
	for i, line := range m.flatTree {
		if line.entry.Path == path {
			return i
		}
	}
	return -1
}

func (m *Model) ensureSelectionVisible() {
	if len(m.flatTree) == 0 || m.treeVP.Height == 0 {
		return
	}
	if m.treeSelection < m.treeVP.YOffset {
		m.treeVP.SetYOffset(m.treeSelection)
		return
	}
	bottom := m.treeVP.YOffset + m.treeVP.Height - 1
	if m.treeSelection > bottom {
		m.treeVP.SetYOffset(m.treeSelection - m.treeVP.Height + 1)
	}
}

func (m *Model) focusTree() {
	m.treeFocus = true
	m.updateTreePanelStyle()
	m.updateTreeContent(m.treeContentWidth)
	m.ensureSelectionVisible()
}

func (m *Model) blurTree() {
	if !m.treeVisible {
		m.treeFocus = false
	} else if m.treeFocus {
		m.treeFocus = false
	}
	m.updateTreePanelStyle()
	m.updateTreeContent(m.treeContentWidth)
}

func (m *Model) updateTreePanelStyle() {
	color := treeBlurBorderColor
	if m.treeFocus {
		color = treeFocusBorderColor
	}
	m.treeVP.Style = treePanelStyle(color)
}

func treePanelStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(color)
}

func formatTreeLabel(entry *tree.Node, depth int) string {
	if depth == 0 {
		return entry.Name + "/"
	}
	indent := strings.Repeat("  ", depth-1)
	indicator := "  "
	if entry.IsDir {
		if entry.Open {
			indicator = "- "
		} else {
			indicator = "+ "
		}
	}
	label := indent + indicator + entry.Name
	if entry.IsDir {
		label += "/"
	}
	return label
}

func composeDisplayPath(root, rel string) string {
	rel = filepath.ToSlash(rel)
	if root == "" {
		return rel
	}
	if rel == "" {
		return root + "/"
	}
	return filepath.ToSlash(filepath.Join(root, rel))
}

func newRenderer(width int) (*glamour.TermRenderer, error) {
	opts := []glamour.TermRendererOption{glamour.WithStandardStyle(styles.TokyoNightStyle)}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	} else {
		opts = append(opts, glamour.WithWordWrap(0))
	}
	return glamour.NewTermRenderer(opts...)
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Model) loadNode(node *tree.Node) bool {
	if node == nil {
		return false
	}
	if err := node.EnsureLoaded(); err != nil {
		m.err = err
		return false
	}
	return true
}
