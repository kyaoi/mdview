package tree

import "strings"

// Build constructs a tree that mirrors the provided relative paths. The root
// node represents the directory chosen by the user.
func Build(rootName string, files []string) *Node {
	root := &Node{
		Name:  rootName,
		Path:  "",
		IsDir: true,
		Open:  true,
	}

	for _, rel := range files {
		parts := strings.Split(rel, "/")
		current := root
		currentPath := ""

		for i, part := range parts {
			isDir := i < len(parts)-1
			if isDir {
				currentPath = joinPath(currentPath, part)
				child := current.ChildByName(part)
				if child == nil {
					child = &Node{
						Name:  part,
						Path:  currentPath,
						IsDir: true,
					}
					current.AddChild(child)
				}
				current = child
				continue
			}

			filePath := joinPath(currentPath, part)
			if current.ChildByName(part) != nil {
				continue
			}
			current.AddChild(&Node{
				Name:  part,
				Path:  filePath,
				IsDir: false,
			})
		}
	}

	root.SortRecursive()
	return root
}

func joinPath(base, part string) string {
	if base == "" {
		return part
	}
	return base + "/" + part
}
