package scaffold

import (
	"path"
	"sort"
	"strings"
)

// TreeLine is one rendered row of a file tree.
type TreeLine struct {
	Prefix string // the box drawing characters leading up to the name
	Name   string // the file or directory name, with a trailing slash for dirs
	IsDir  bool
}

// treeNode is a directory entry while the tree is being built.
type treeNode struct {
	name     string
	children map[string]*treeNode
	isDir    bool
}

func newNode(name string, isDir bool) *treeNode {
	return &treeNode{name: name, children: map[string]*treeNode{}, isDir: isDir}
}

// Tree turns a flat list of slash separated paths into renderable rows.
// Directories are listed before files, each group sorted alphabetically.
func Tree(paths []string) []TreeLine {
	root := newNode("", true)
	for _, p := range paths {
		parts := strings.Split(path.Clean(strings.TrimPrefix(p, "./")), "/")
		node := root
		for i, part := range parts {
			isDir := i < len(parts)-1
			child, ok := node.children[part]
			if !ok {
				child = newNode(part, isDir)
				node.children[part] = child
			}
			node = child
		}
	}

	var lines []TreeLine
	var walk func(n *treeNode, prefix string)
	walk = func(n *treeNode, prefix string) {
		entries := sortedChildren(n)
		for i, child := range entries {
			last := i == len(entries)-1
			branch := "├── "
			nextPrefix := prefix + "│   "
			if last {
				branch = "└── "
				nextPrefix = prefix + "    "
			}
			name := child.name
			if child.isDir {
				name += "/"
			}
			lines = append(lines, TreeLine{Prefix: prefix + branch, Name: name, IsDir: child.isDir})
			if child.isDir {
				walk(child, nextPrefix)
			}
		}
	}
	walk(root, "")
	return lines
}

// sortedChildren orders directories first, then files, alphabetically.
func sortedChildren(n *treeNode) []*treeNode {
	entries := make([]*treeNode, 0, len(n.children))
	for _, child := range n.children {
		entries = append(entries, child)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return entries[i].name < entries[j].name
	})
	return entries
}

// RenderTree produces a plain text tree rooted at the given name.
func RenderTree(root string, paths []string) string {
	var b strings.Builder
	b.WriteString(root + "/\n")
	for _, line := range Tree(paths) {
		b.WriteString(line.Prefix + line.Name + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
