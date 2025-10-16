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
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return ui.State{}, err
		}

		rootName := filepath.Base(absTarget)
		loader := tree.NewFSLoader(absTarget)
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
				RootDir:           absTarget,
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
			RootDir:           absTarget,
			DisplayRoot:       rootName,
			ActiveAbsPath:     "",
			FocusTree:         true,
		}, nil
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return ui.State{}, err
	}

	data, err := os.ReadFile(absTarget)
	if err != nil {
		return ui.State{}, err
	}

	displayPath := absTarget
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, absTarget); err == nil {
			displayPath = rel
		}
	}

	return ui.State{
		RawContent:    string(data),
		HeaderPath:    filepath.ToSlash(displayPath),
		ActiveAbsPath: absTarget,
	}, nil
}
