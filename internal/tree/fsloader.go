package tree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var errNotDir = errors.New("path is not a directory")

// FSLoader loads tree nodes by reading the filesystem under the given root.
type FSLoader struct {
	root  string
	cache map[string]bool
}

// NewFSLoader creates a loader that reads from the provided root directory.
func NewFSLoader(root string) *FSLoader {
	return &FSLoader{
		root:  root,
		cache: make(map[string]bool),
	}
}

// List returns immediate child entries for the provided relative path.
func (l *FSLoader) List(relPath string) ([]*Node, error) {
	dir := l.abs(relPath)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errNotDir
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var nodes []*Node
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipDir(name) {
				continue
			}
			childPath := join(relPath, name)
			has, err := l.HasMarkdown(childPath)
			if err != nil {
				return nil, err
			}
			if !has {
				continue
			}
			nodes = append(nodes, &Node{
				Name:  name,
				Path:  childPath,
				IsDir: true,
			})
			continue
		}
		if !isMarkdown(name) {
			continue
		}
		nodes = append(nodes, &Node{
			Name:  name,
			Path:  join(relPath, name),
			IsDir: false,
		})
	}
	return nodes, nil
}

// HasMarkdown reports whether the path (relative to the loader root) contains at
// least one Markdown file within its subtree.
func (l *FSLoader) HasMarkdown(relPath string) (bool, error) {
	if cached, ok := l.cache[relPath]; ok {
		return cached, nil
	}

	entries, err := os.ReadDir(l.abs(relPath))
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipDir(name) {
				continue
			}
			childPath := join(relPath, name)
			has, err := l.HasMarkdown(childPath)
			if err != nil {
				return false, err
			}
			if has {
				l.cache[relPath] = true
				return true, nil
			}
			continue
		}
		if isMarkdown(name) {
			l.cache[relPath] = true
			return true, nil
		}
	}

	l.cache[relPath] = false
	return false, nil
}

func (l *FSLoader) abs(relPath string) string {
	if relPath == "" {
		return l.root
	}
	return filepath.Join(l.root, filepath.FromSlash(relPath))
}

func join(base, part string) string {
	if base == "" {
		return part
	}
	return base + "/" + part
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", ".hg", ".svn", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func isMarkdown(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}
