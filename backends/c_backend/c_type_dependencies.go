// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

// TopologicalSortStructs sorts struct definitions so dependencies come first.
// Uses Kahn's algorithm for topological sorting.
func TopologicalSortStructs(structs []*StructDefinition) []*StructDefinition {
	if len(structs) == 0 {
		return structs
	}

	// Build maps for quick lookup
	nameToStruct := make(map[string]*StructDefinition)
	for _, s := range structs {
		nameToStruct[s.Name] = s
	}

	// Build in-degree map (count of dependencies not yet processed)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // type -> structs that depend on it

	for _, s := range structs {
		inDegree[s.Name] = 0
	}

	for _, s := range structs {
		for _, dep := range s.Dependencies {
			if _, exists := nameToStruct[dep]; exists {
				inDegree[s.Name]++
				dependents[dep] = append(dependents[dep], s.Name)
			}
		}
	}

	// Start with structs that have no dependencies
	queue := []string{}
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	result := make([]*StructDefinition, 0, len(structs))

	for len(queue) > 0 {
		// Pop from queue
		name := queue[0]
		queue = queue[1:]

		if s, ok := nameToStruct[name]; ok {
			result = append(result, s)
		}

		// Reduce in-degree for dependents
		for _, dependent := range dependents[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// If we couldn't process all structs, there's a cycle - return original order
	if len(result) < len(structs) {
		return structs
	}

	return result
}

// CircularDependency represents a cycle in type dependencies
type CircularDependency struct {
	Types []*StructDefinition // Types involved in the cycle
}

// DetectCircularValueDependencies checks for cycles in non-pointer (value) dependencies.
// These cycles cause infinite struct sizes and are compile errors.
// Returns a list of cycles found, empty if no cycles.
func DetectCircularValueDependencies(structs []*StructDefinition) []CircularDependency {
	if len(structs) == 0 {
		return nil
	}

	// Build maps for quick lookup
	nameToStruct := make(map[string]*StructDefinition)
	for _, s := range structs {
		nameToStruct[s.Name] = s
	}

	// Build adjacency list from VALUE dependencies only
	adj := make(map[string][]string)
	for _, s := range structs {
		for _, dep := range s.ValueDependencies {
			if _, exists := nameToStruct[dep]; exists {
				adj[s.Name] = append(adj[s.Name], dep)
			}
		}
	}

	// Track visited state for cycle detection
	// 0 = unvisited, 1 = in current path (gray), 2 = fully visited (black)
	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles []CircularDependency

	// DFS to find cycles
	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = 1 // Mark as being visited

		for _, neighbor := range adj[node] {
			if color[neighbor] == 1 {
				// Found a cycle - reconstruct it
				cycle := []*StructDefinition{nameToStruct[neighbor]}
				curr := node
				for curr != neighbor {
					cycle = append([]*StructDefinition{nameToStruct[curr]}, cycle...)
					curr = parent[curr]
				}
				cycles = append(cycles, CircularDependency{Types: cycle})
				return true
			}
			if color[neighbor] == 0 {
				parent[neighbor] = node
				if dfs(neighbor) {
					return true
				}
			}
		}

		color[node] = 2 // Mark as fully visited
		return false
	}

	// Run DFS from each unvisited node
	for _, s := range structs {
		if color[s.Name] == 0 {
			dfs(s.Name)
		}
	}

	return cycles
}
