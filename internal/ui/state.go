package ui

import "github.com/kyaoi/mdview/internal/tree"

// State contains the data required to bootstrap the Bubble Tea model.
type State struct {
	RawContent         string
	HeaderPath         string
	TreeVisible        bool
	TreePreferredWidth int
	TreeRoot           *tree.Node
	TreeSelectionPath  string
	RootDir            string
	DisplayRoot        string
	ActiveAbsPath      string
	FocusTree          bool
}
