package tasks

import (
	"sort"
	"strings"
)

func BuildForest(all []*Task) []*Node {
	nodes := make(map[string]*Node, len(all))
	for _, task := range all {
		nodes[task.ID] = &Node{Task: task}
	}

	roots := make([]*Node, 0)
	for _, task := range all {
		node := nodes[task.ID]
		if task.ParentID == "" {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodes[task.ParentID]
		if !ok || parent == node || createsCycle(parent, node, nodes) {
			node.Orphan = true
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	sortNodes(roots)
	return roots
}

func FilterForest(forest []*Node, includeClosed bool) []*Node {
	if includeClosed {
		return cloneNodes(forest, false)
	}
	var result []*Node
	for _, node := range forest {
		result = append(result, filterNode(node)...)
	}
	sortNodes(result)
	return result
}

func GroupForest(forest []*Node) []StatusGroup {
	byName := make(map[string]*StatusGroup)
	keys := make([]string, 0)
	for _, root := range forest {
		key := normalizeStatusName(root.Task.Status.Name)
		if key == "" {
			key = "sin estado"
		}
		group, exists := byName[key]
		if !exists {
			status := root.Task.Status
			if status.Name == "" {
				status.Name = "Sin estado"
			}
			group = &StatusGroup{Status: status}
			byName[key] = group
			keys = append(keys, key)
		} else if root.Task.Status.OrderIndex < group.Status.OrderIndex {
			group.Status.OrderIndex = root.Task.Status.OrderIndex
		}
		group.Roots = append(group.Roots, root)
	}

	groups := make([]StatusGroup, 0, len(keys))
	for _, key := range keys {
		group := byName[key]
		sortNodes(group.Roots)
		groups = append(groups, *group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		leftRank := statusSortRank(groups[i].Status.Name)
		rightRank := statusSortRank(groups[j].Status.Name)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if groups[i].Status.OrderIndex != groups[j].Status.OrderIndex {
			return groups[i].Status.OrderIndex < groups[j].Status.OrderIndex
		}
		return strings.ToLower(groups[i].Status.Name) < strings.ToLower(groups[j].Status.Name)
	})
	return groups
}

// statusSortRank defines top-to-bottom workflow order. Unlisted statuses stay last.
func statusSortRank(name string) int {
	normalized := normalizeStatusName(name)
	switch {
	case normalized == "produccion", normalized == "production":
		return 0
	case normalized == "esperando deploy", normalized == "waiting for deploy", normalized == "esperando despliegue":
		return 1
	case normalized == "staging":
		return 2
	case normalized == "esperando release", normalized == "waiting for release":
		return 3
	case normalized == "qa testing":
		return 4
	case normalized == "ready for test", normalized == "ready for testing", normalized == "en ready for testing":
		return 5
	case strings.HasPrefix(normalized, "bloquead"), normalized == "blocked":
		return 6
	case normalized == "en review", normalized == "in review":
		return 7
	case normalized == "en curso", normalized == "in progress":
		return 8
	case normalized == "por hacer", normalized == "to do", normalized == "todo":
		return 9
	case normalized == "en refinamiento", normalized == "en refinaimiento", normalized == "por refinar", normalized == "to refine":
		return 10
	default:
		return 11
	}
}

func normalizeStatusName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
	).Replace(normalized)
	return strings.Join(strings.Fields(normalized), " ")
}

func filterNode(node *Node) []*Node {
	children := make([]*Node, 0)
	for _, child := range node.Children {
		children = append(children, filterNode(child)...)
	}
	if node.Task.Status.Closed() {
		for _, child := range children {
			child.Promoted = true
		}
		return children
	}
	copy := &Node{
		Task:     node.Task,
		Children: children,
		Orphan:   node.Orphan,
		Promoted: node.Promoted,
	}
	return []*Node{copy}
}

func cloneNodes(nodes []*Node, promoted bool) []*Node {
	result := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, &Node{
			Task:     node.Task,
			Children: cloneNodes(node.Children, false),
			Orphan:   node.Orphan,
			Promoted: node.Promoted || promoted,
		})
	}
	return result
}

func createsCycle(parent, child *Node, nodes map[string]*Node) bool {
	seen := map[string]bool{child.Task.ID: true}
	current := parent
	for current != nil {
		if seen[current.Task.ID] {
			return true
		}
		seen[current.Task.ID] = true
		parentID := current.Task.ParentID
		if parentID == "" {
			return false
		}
		current = nodes[parentID]
	}
	return false
}
