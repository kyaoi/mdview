package tree

import (
	"sort"
	"strings"
)

// Node represents a single entry in the file tree.
type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Open     bool
	Parent   *Node
	Children []*Node
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

// AddChild attaches the provided child to the current node.
func (n *Node) AddChild(child *Node) {
	child.Parent = n
	n.Children = append(n.Children, child)
}

// SortRecursive sorts children so that directories appear before files and
// names are ordered case-insensitively. It then applies the same ordering
// recursively to descendants.
func (n *Node) SortRecursive() {
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
	for _, child := range n.Children {
		if len(child.Children) > 0 {
			child.SortRecursive()
		}
	}
}
