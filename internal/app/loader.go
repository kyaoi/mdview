package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		files, err := collectMarkdownFiles(target)
		if err != nil {
			return ui.State{}, err
		}

		rootName := filepath.Base(target)
		treeRoot := tree.Build(rootName, files)

		if len(files) == 0 {
			message := fmt.Sprintf("%s にMarkdownファイルが見つかりません。", rootName)
			return ui.State{
				RawContent:        message,
				HeaderPath:        rootName + "/",
				TreeVisible:       true,
				TreeRoot:          treeRoot,
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
			TreeRoot:          treeRoot,
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
