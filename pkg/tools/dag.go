package tools

import (
	"errors"
	"fmt"
)

/*
DAG Structure:
  subfinder ─┐
             │
  findomain ─┼──► httpxbb ─┬──► gowitness
             │             │
chaos-client ─┘             └──► nuclei

Execution Levels:
Level 1 (Parallel):
+---------------+   +---------------+   +---------------+
|  subfinder   |   |  findomain   |   | chaos-client |
+---------------+   +---------------+   +---------------+
          │                 │                 │
          └───────┬─────────┼─────────┬───────┘
                  │         │         │
                  ▼         ▼         ▼
             [Combine Subdomain Files]
                         │
                         ▼
Level 2 (Single):
+---------------+
|   httpxbb    |
+---------------+
          │
          └───────┬─────────┬───────┐
                  │         │       │
                  ▼         ▼       ▼
Level 3 (Parallel):
+---------------+          +---------------+
|  gowitness   |          |    nuclei     |
+---------------+          +---------------+
*/

type depGraph struct {
	nodes       map[string]Tool
	children    map[string][]string
	remaining   map[string]int
	failedDeps  map[string]int
	initialized bool
}

func newDepGraph(tools []Tool) (*depGraph, error) {
	g := &depGraph{
		nodes:      make(map[string]Tool, len(tools)),
		children:   make(map[string][]string, len(tools)),
		remaining:  make(map[string]int, len(tools)),
		failedDeps: make(map[string]int, len(tools)),
	}
	// Build nodes
	for _, t := range tools {
		name := t.Name()
		if _, exists := g.nodes[name]; exists {
			return nil, fmt.Errorf("duplicate tool name: %s", name)
		}
		g.nodes[name] = t
	}

	// Build edges and indegrees
	for _, t := range tools {
		name := t.Name()
		deps := t.DependsOn()
		g.remaining[name] = len(deps)
		for _, p := range deps {
			if _, ok := g.nodes[p]; !ok {
				return nil, fmt.Errorf("tool %s depends on unknown tool %s", name, p)
			}
			g.children[p] = append(g.children[p], name)
		}
	}
	g.initialized = true
	return g, nil
}

func (g *depGraph) validate() error {
	if !g.initialized {
		return errors.New("graph not initialized")
	}
	inDeg := make(map[string]int, len(g.remaining))
	for k, v := range g.remaining {
		inDeg[k] = v
	}
	queue := make([]string, 0)
	for n, d := range inDeg {
		if d == 0 {
			queue = append(queue, n)
		}
	}
	seen := 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		seen++
		for _, c := range g.children[n] {
			inDeg[c]--
			if inDeg[c] == 0 {
				queue = append(queue, c)
			}
		}
	}
	if seen != len(g.nodes) {
		return fmt.Errorf("dependency cycle detected (seen %d of %d)", seen, len(g.nodes))
	}
	return nil
}

func (g *depGraph) initialReady() []Tool {
	var ready []Tool
	for name, deg := range g.remaining {
		if deg == 0 && g.failedDeps[name] == 0 {
			ready = append(ready, g.nodes[name])
		}
	}
	return ready
}

// onComplete updates children after 'name' completes.
// Returns:
//   - newReady: tools that became ready to run
//   - skipped: tools that are now terminally skipped (due to a failed ancestor)
//
// This function also recursively propagates skips down the graph.
func (g *depGraph) onComplete(name string, success bool) (newReady []Tool, skipped []string) {
	// Update direct children first
	queue := make([]string, 0)
	for _, child := range g.children[name] {
		g.remaining[child]--
		if !success {
			g.failedDeps[child]++
		}

		// If child has no remaining deps
		if g.remaining[child] == 0 {
			if g.failedDeps[child] == 0 {
				newReady = append(newReady, g.nodes[child])
			} else {
				// child is skipped; propagate failure as if it failed
				queue = append(queue, child)
			}
		}
	}

	// Propagate skips breadth-first
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		skipped = append(skipped, cur)

		for _, gc := range g.children[cur] {
			g.remaining[gc]--
			g.failedDeps[gc]++ // skipped parent counts as failed prerequisite
			if g.remaining[gc] == 0 {
				if g.failedDeps[gc] == 0 {
					newReady = append(newReady, g.nodes[gc])
				} else {
					queue = append(queue, gc)
				}
			}
		}
	}

	return newReady, skipped
}
