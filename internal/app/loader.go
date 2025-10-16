package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kyaoi/mdview/internal/tree"
	"github.com/kyaoi/mdview/internal/ui"
)

// LoadInitialState analyses the target path and prepares the UI state.
func LoadInitialState(target string) (ui.State, error) {
	info, err := os.Stat(target)
	if err != nil {
		return ui.State{}, err
	}

	if info.IsDir() {
		rootName := filepath.Base(target)
		loader := tree.NewFSLoader(target)
		root := tree.NewRoot(rootName, loader)

		hasMarkdown, err := loader.HasMarkdown("")
		if err != nil {
			return ui.State{}, err
		}
		if !hasMarkdown {
			message := fmt.Sprintf("%s にMarkdownファイルが見つかりません。", rootName)
			return ui.State{
				RawContent:        message,
				HeaderPath:        rootName + "/",
				TreeVisible:       true,
				TreeRoot:          root,
				RootDir:           target,
				DisplayRoot:       rootName,
				TreeSelectionPath: "",
				FocusTree:         true,
			}, nil
		}

		return ui.State{
			RawContent:        "",
			HeaderPath:        rootName + "/",
			TreeVisible:       true,
			TreeRoot:          root,
			TreeSelectionPath: "",
			RootDir:           target,
			DisplayRoot:       rootName,
			ActiveAbsPath:     "",
			FocusTree:         true,
		}, nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return ui.State{}, err
	}

	displayPath := target
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, target); err == nil {
			displayPath = rel
		}
	}

	return ui.State{
		RawContent: string(data),
		HeaderPath: filepath.ToSlash(displayPath),
	}, nil
}
