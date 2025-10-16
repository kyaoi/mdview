package tree

import (
	"sort"
	"strings"
)

// Loader retrieves child entries for a particular node path.
type Loader interface {
	List(path string) ([]*Node, error)
}

// Node represents a single entry in the file tree.
type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Open     bool
	Parent   *Node
	Children []*Node

	loader Loader
	loaded bool
}

// NewRoot creates the root node for the tree.
func NewRoot(name string, loader Loader) *Node {
	return &Node{
		Name:   name,
		Path:   "",
		IsDir:  true,
		Open:   true,
		loader: loader,
	}
}

// ChildByName returns the child node with the given name if it exists.
func (n *Node) ChildByName(name string) *Node {
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// EnsureLoaded lazily loads child entries for directory nodes.
func (n *Node) EnsureLoaded() error {
	if !n.IsDir || n.loaded || n.loader == nil {
		return nil
	}

	children, err := n.loader.List(n.Path)
	if err != nil {
		return err
	}

	n.Children = children
	for _, child := range n.Children {
		child.Parent = n
		child.loader = n.loader
	}
	n.sortChildren()
	n.loaded = true
	return nil
}

func (n *Node) sortChildren() {
	sort.Slice(n.Children, func(i, j int) bool {
		ci, cj := n.Children[i], n.Children[j]
		switch {
		case ci.IsDir == cj.IsDir:
			return strings.ToLower(ci.Name) < strings.ToLower(cj.Name)
		case ci.IsDir:
			return true
		default:
			return false
		}
	})
}
