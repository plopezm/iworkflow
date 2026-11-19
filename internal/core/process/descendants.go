package process

import (
	"context"
	"fmt"
	"slices"
)

// Descendants returns every process below root in the process tree, root excluded, sorted by pid. It is
// the set a caller must not list when it is about to stop root: root's children, their children, and so on.
// A root that is not running has no descendants and yields an empty result, not an error.
func Descendants(ctx context.Context, root int) ([]int, error) {
	if root <= 0 {
		return nil, fmt.Errorf("invalid root pid %d", root)
	}
	parents, err := processParents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the process tree: %w", err)
	}
	return subtree(root, parents), nil
}

// subtree returns the pids whose chain of parents reaches root, root excluded, sorted. parents maps each pid
// to its parent pid. It is pure, so it is tested on every platform even though each OS builds the map.
// A cycle in the map (which the kernel never reports, but a racing snapshot can look like one) cannot loop:
// every pid is visited once.
func subtree(root int, parents map[int]int) []int {
	children := make(map[int][]int, len(parents))
	for pid, parent := range parents {
		children[parent] = append(children[parent], pid)
	}
	visited := map[int]bool{root: true}
	queue := []int{root}
	var found []int
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if visited[child] {
				continue
			}
			visited[child] = true
			found = append(found, child)
			queue = append(queue, child)
		}
	}
	slices.Sort(found)
	return found
}
