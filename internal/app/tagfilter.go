package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kyaoi/mdview/internal/tree"
	"github.com/kyaoi/mdview/internal/ui"
)

// RunTagFiltered launches the viewer with a tree composed only of the provided
// relative paths. The paths must be expressed using forward slashes and be
// relative to rootDir.
func RunTagFiltered(rootDir, displayRoot string, relPaths []string, tag string) error {
	if len(relPaths) == 0 {
		return fmt.Errorf("タグ %q に一致するファイルがありません", tag)
	}
	root := buildFilteredTree(displayRoot, relPaths)
	state := ui.State{
		RawContent:        fmt.Sprintf("タグ \"%s\" を含むファイルを選択してください。", tag),
		HeaderPath:        fmt.Sprintf("%s/ (tag: %s)", displayRoot, tag),
		TreeVisible:       true,
		TreeRoot:          root,
		TreeSelectionPath: relPaths[0],
		RootDir:           rootDir,
		DisplayRoot:       displayRoot,
		FocusTree:         true,
	}
	return runProgram(state)
}

func buildFilteredTree(displayRoot string, relPaths []string) *tree.Node {
	root := &tree.Node{
		Name:  displayRoot,
		Path:  "",
		IsDir: true,
		Open:  true,
	}
	for _, rel := range relPaths {
		trimmed := strings.Trim(rel, "/")
		if trimmed == "" {
			continue
		}
		insertPath(root, trimmed)
	}
	sortTree(root)
	return root
}

func insertPath(root *tree.Node, rel string) {
	parts := strings.Split(rel, "/")
	current := root
	parentPath := ""
	for i, part := range parts {
		isLast := i == len(parts)-1
		childPath := joinPath(parentPath, part)
		child := current.ChildByName(part)
		if child == nil {
			child = &tree.Node{
				Name:  part,
				Path:  childPath,
				IsDir: !isLast,
			}
			child.Parent = current
			current.Children = append(current.Children, child)
		}
		current = child
		parentPath = childPath
		if isLast {
			current.IsDir = false
			current.Children = nil
			current.Open = false
		}
	}
}

func sortTree(node *tree.Node) {
	if node == nil || len(node.Children) == 0 {
		return
	}
	sort.Slice(node.Children, func(i, j int) bool {
		ci, cj := node.Children[i], node.Children[j]
		switch {
		case ci.IsDir == cj.IsDir:
			return strings.ToLower(ci.Name) < strings.ToLower(cj.Name)
		case ci.IsDir:
			return true
		default:
			return false
		}
	})
	for _, child := range node.Children {
		sortTree(child)
	}
}

func joinPath(base, part string) string {
	if base == "" {
		return part
	}
	return base + "/" + part
}
