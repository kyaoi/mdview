package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	styles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

const (
	headerHeight      = 0
	minContentWidth   = 20
	minTreePanelWidth = 18
	defaultTreeWidth  = 28
)

type initialState struct {
	rawContent         string
	headerPath         string
	treeVisible        bool
	treePreferredWidth int
	treeRoot           *treeEntry
	treeSelectionPath  string
	rootDir            string
	displayRoot        string
	activeAbsPath      string
	focusTree          bool
}

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

type model struct {
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

	treeRoot      *treeEntry
	flatTree      []treeLine
	treeSelection int
	rootDir       string
	displayRoot   string
	activeAbsPath string
}

type treeEntry struct {
	name     string
	path     string
	isDir    bool
	open     bool
	parent   *treeEntry
	children []*treeEntry
}

type treeLine struct {
	entry *treeEntry
	label string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mdview <path-to-markdown-or-directory>")
		os.Exit(1)
	}

	target := filepath.Clean(os.Args[1])
	state, err := loadInitialState(target)
	if err != nil {
		log.Fatal(err)
	}

	p := tea.NewProgram(initialModel(state), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func initialModel(state initialState) model {
	contentVP := viewport.New(0, 0)
	contentVP.Style = lipgloss.NewStyle().Padding(0, 1)
	contentVP.SetHorizontalStep(2)

	treeVP := viewport.New(0, 0)
	treeVP.Style = treePanelStyle(treeBlurBorderColor)
	treeVP.MouseWheelEnabled = false

	m := model{
		contentVP:          contentVP,
		treeVP:             treeVP,
		rawContent:         state.rawContent,
		headerPath:         state.headerPath,
		treeVisible:        state.treeVisible && state.treeRoot != nil,
		treePreferredWidth: state.treePreferredWidth,
		treeRoot:           state.treeRoot,
		rootDir:            state.rootDir,
		displayRoot:        state.displayRoot,
		activeAbsPath:      state.activeAbsPath,
	}

	if m.treeRoot != nil {
		m.refreshTreeViewWithSelection(state.treeSelectionPath)
	}
	m.updateTreePanelStyle()

	if state.focusTree {
		m.focusTree()
	}

	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) View() string {
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		var cmd tea.Cmd
		if !handled {
			m.contentVP, cmd = m.contentVP.Update(msg)
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.contentVP, cmd = m.contentVP.Update(msg)
	return m, cmd
}

func (m *model) handleContentKey(key string) bool {
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

func (m *model) handleTreeKey(key string) bool {
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
	case "ctrl+d":
		m.contentVP.ScrollDown(1)
	case "ctrl+u":
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

func (m *model) resize(width, height int) {
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

func (m model) treeWidth(totalWidth int) int {
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

func (m *model) moveTreeSelection(delta int) {
	if len(m.flatTree) == 0 {
		return
	}
	m.treeSelection = clamp(m.treeSelection+delta, 0, len(m.flatTree)-1)
	m.updateTreeContent(m.treeContentWidth)
}

func (m *model) openOrDescend() {
	entry := m.currentTreeEntry()
	if entry == nil {
		return
	}
	if entry.isDir {
		if !entry.open {
			entry.open = true
			m.refreshTreeViewWithSelection(entry.path)
			return
		}
		if len(entry.children) > 0 {
			m.moveTreeSelection(1)
		}
		return
	}
	m.openFileEntry(entry)
}

func (m *model) closeOrAscend() {
	entry := m.currentTreeEntry()
	if entry == nil {
		return
	}
	if entry.isDir && entry.open {
		entry.open = false
		maxWidth := m.rebuildFlatTree()
		if idx := m.indexForPath(entry.path); idx >= 0 {
			m.treeSelection = idx
		} else {
			m.treeSelection = clamp(m.treeSelection, 0, len(m.flatTree)-1)
		}
		m.treeContentWidth = maxWidth
		m.updateTreeContent(maxWidth)
		return
	}
	if entry.parent != nil {
		m.refreshTreeViewWithSelection(entry.parent.path)
	}
}

func (m *model) currentTreeEntry() *treeEntry {
	if len(m.flatTree) == 0 || m.treeSelection < 0 || m.treeSelection >= len(m.flatTree) {
		return nil
	}
	return m.flatTree[m.treeSelection].entry
}

func (m *model) openFileEntry(entry *treeEntry) {
	if m.rootDir == "" {
		return
	}
	absPath := filepath.Join(m.rootDir, filepath.FromSlash(entry.path))
	data, err := os.ReadFile(absPath)
	if err != nil {
		m.err = err
		return
	}
	m.rawContent = string(data)
	m.activeAbsPath = absPath
	m.headerPath = composeDisplayPath(m.displayRoot, entry.path)
	m.renderMarkdown()
	m.contentVP.GotoTop()
}

func (m *model) renderMarkdown() {
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

func (m *model) refreshTreeViewWithSelection(path string) {
	if m.treeRoot == nil {
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

func (m *model) expandPath(path string) {
	if m.treeRoot == nil {
		return
	}
	if path == "" {
		return
	}
	if !m.treeRoot.open {
		m.treeRoot.open = true
	}
	parts := strings.Split(path, "/")
	current := m.treeRoot
	for _, part := range parts {
		child := current.childByName(part)
		if child == nil {
			return
		}
		if child.isDir {
			child.open = true
		}
		current = child
	}
}

func (m *model) rebuildFlatTree() int {
	if m.treeRoot == nil {
		m.flatTree = nil
		return 0
	}
	var lines []treeLine
	maxWidth := 0
	var walk func(node *treeEntry, depth int)
	walk = func(node *treeEntry, depth int) {
		label := formatTreeLabel(node, depth)
		if w := lipgloss.Width(label); w > maxWidth {
			maxWidth = w
		}
		lines = append(lines, treeLine{entry: node, label: label})
		if node.isDir && node.open {
			for _, child := range node.children {
				walk(child, depth+1)
			}
		}
	}
	walk(m.treeRoot, 0)
	m.flatTree = lines
	return maxWidth
}

func (m *model) updateTreeContent(width int) {
	if m.treeRoot == nil {
		return
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

func (m *model) indexForPath(path string) int {
	for i, line := range m.flatTree {
		if line.entry.path == path {
			return i
		}
	}
	return -1
}

func (m *model) ensureSelectionVisible() {
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

func (m *model) focusTree() {
	m.treeFocus = true
	m.updateTreePanelStyle()
	m.updateTreeContent(m.treeContentWidth)
	m.ensureSelectionVisible()
}

func (m *model) blurTree() {
	if !m.treeVisible {
		m.treeFocus = false
	} else if m.treeFocus {
		m.treeFocus = false
	}
	m.updateTreePanelStyle()
	m.updateTreeContent(m.treeContentWidth)
}

func (m *model) updateTreePanelStyle() {
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

func formatTreeLabel(entry *treeEntry, depth int) string {
	if depth == 0 {
		return entry.name + "/"
	}
	indent := strings.Repeat("  ", depth-1)
	indicator := "  "
	if entry.isDir {
		if entry.open {
			indicator = "- "
		} else {
			indicator = "+ "
		}
	}
	label := indent + indicator + entry.name
	if entry.isDir {
		label += "/"
	}
	return label
}

func loadInitialState(target string) (initialState, error) {
	info, err := os.Stat(target)
	if err != nil {
		return initialState{}, err
	}

	if info.IsDir() {
		files, err := collectMarkdownFiles(target)
		if err != nil {
			return initialState{}, err
		}

		rootName := filepath.Base(target)
		tree := buildTree(rootName, files)

		if len(files) == 0 {
			message := fmt.Sprintf("%s にMarkdownファイルが見つかりません。", rootName)
			return initialState{
				rawContent:        message,
				headerPath:        rootName + "/",
				treeVisible:       true,
				treeRoot:          tree,
				rootDir:           target,
				displayRoot:       rootName,
				treeSelectionPath: "",
				focusTree:         true,
			}, nil
		}

		return initialState{
			rawContent:        "",
			headerPath:        rootName + "/",
			treeVisible:       true,
			treeRoot:          tree,
			treeSelectionPath: "",
			rootDir:           target,
			displayRoot:       rootName,
			activeAbsPath:     "",
			focusTree:         true,
		}, nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return initialState{}, err
	}

	displayPath := target
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, target); err == nil {
			displayPath = rel
		}
	}

	return initialState{
		rawContent: string(data),
		headerPath: filepath.ToSlash(displayPath),
	}, nil
}

func collectMarkdownFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if isMarkdown(d.Name()) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i]) < strings.ToLower(files[j])
	})
	return files, nil
}

func shouldSkipDir(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case ".git", "node_modules", ".hg", ".svn", ".idea", ".vscode":
		return true
	}
	return false
}

func isMarkdown(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func buildTree(rootName string, files []string) *treeEntry {
	root := &treeEntry{name: rootName, path: "", isDir: true, open: true}
	for _, rel := range files {
		parts := strings.Split(rel, "/")
		current := root
		currentPath := ""
		for i, part := range parts {
			isDir := i < len(parts)-1
			if isDir {
				currentPath = joinPath(currentPath, part)
				child := current.childByName(part)
				if child == nil {
					child = &treeEntry{name: part, path: currentPath, isDir: true, open: false, parent: current}
					current.children = append(current.children, child)
				}
				current = child
			} else {
				filePath := joinPath(currentPath, part)
				if child := current.childByName(part); child == nil {
					current.children = append(current.children, &treeEntry{name: part, path: filePath, isDir: false, parent: current})
				}
			}
		}
	}
	root.sortRecursive()
	return root
}

func (n *treeEntry) childByName(name string) *treeEntry {
	for _, child := range n.children {
		if child.name == name {
			return child
		}
	}
	return nil
}

func (n *treeEntry) sortRecursive() {
	sort.Slice(n.children, func(i, j int) bool {
		if n.children[i].isDir == n.children[j].isDir {
			return strings.ToLower(n.children[i].name) < strings.ToLower(n.children[j].name)
		}
		return n.children[i].isDir && !n.children[j].isDir
	})
	for _, child := range n.children {
		if len(child.children) > 0 {
			child.sortRecursive()
		}
	}
}

func joinPath(base, part string) string {
	if base == "" {
		return part
	}
	return base + "/" + part
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
