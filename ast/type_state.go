// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/modules.md, spec/scoping.md

package ast

// TypeState tracks refined types for variables at a program point.
// Used for flow-sensitive type analysis where variable types can be
// narrowed based on control flow (e.g., null checks, type guards).
type TypeState struct {
	// Facts maps variable names to their refined type information
	Facts map[string]*RefinedTypeInfo
	// Path is a branch identifier used for merging at join points
	Path int
	// Parent enables scope nesting - lookups check parent if not found locally
	Parent *TypeState
}

// RefinedTypeInfo holds refinement information for a variable.
// Refinements are facts we've proven about a variable through control flow.
type RefinedTypeInfo struct {
	// BaseType is the declared/inferred type name
	BaseType string
	// IsNonNull indicates we've proven this variable is not null
	IsNonNull bool
	// IsMoved indicates ownership was moved out of this variable
	IsMoved bool
	// Predicates are active conditions proven about the variable (e.g., "self > 0")
	Predicates []string
	// NarrowedAt tracks source locations where narrowing occurred (as "file:line")
	NarrowedAt []string
}

// NewTypeState creates an empty type state with no parent.
func NewTypeState() *TypeState {
	return &TypeState{
		Facts:  make(map[string]*RefinedTypeInfo),
		Path:   0,
		Parent: nil,
	}
}

// Fork creates a child state for a branch with the given path identifier.
// The child inherits access to parent facts through Lookup.
func (ts *TypeState) Fork(path int) *TypeState {
	return &TypeState{
		Facts:  make(map[string]*RefinedTypeInfo),
		Path:   path,
		Parent: ts,
	}
}

// SetNonNull marks a variable as proven non-null at the current program point.
// Creates the RefinedTypeInfo if it doesn't exist.
func (ts *TypeState) SetNonNull(varName string) {
	info := ts.getOrCreateLocal(varName)
	info.IsNonNull = true
}

// SetMoved marks a variable as moved in the current state.
func (ts *TypeState) SetMoved(varName string) {
	info := ts.getOrCreateLocal(varName)
	info.IsMoved = true
}

// ClearMoved marks a variable as reinitialized (owned) in the current state.
func (ts *TypeState) ClearMoved(varName string) {
	info := ts.getOrCreateLocal(varName)
	info.IsMoved = false
}

// IsMoved returns whether a variable is currently considered moved.
func (ts *TypeState) IsMoved(varName string) bool {
	info := ts.Lookup(varName)
	if info == nil {
		return false
	}
	return info.IsMoved
}

// AddPredicate adds a predicate to a variable's refinement information.
// Predicates are string representations of proven conditions (e.g., "self > 0").
func (ts *TypeState) AddPredicate(varName string, predicate string) {
	info := ts.getOrCreateLocal(varName)
	info.Predicates = append(info.Predicates, predicate)
}

// Lookup returns the refined type info for a variable, checking parent scopes.
// Returns nil if the variable has no refinements in any scope.
func (ts *TypeState) Lookup(varName string) *RefinedTypeInfo {
	if info, exists := ts.Facts[varName]; exists {
		return info
	}
	if ts.Parent != nil {
		return ts.Parent.Lookup(varName)
	}
	return nil
}

// Merge combines two branch states conservatively.
// Only keeps facts that are true in both branches (intersection semantics).
// Returns a new TypeState with the merged facts.
func (ts *TypeState) Merge(other *TypeState) *TypeState {
	merged := &TypeState{
		Facts:  make(map[string]*RefinedTypeInfo),
		Path:   0, // Reset path at join point
		Parent: ts.Parent,
	}

	// Only keep variables that exist in both states
	for varName, info := range ts.Facts {
		otherInfo, exists := other.Facts[varName]
		if !exists {
			continue
		}

		// Conservative merge: only keep facts true in both branches
		mergedInfo := &RefinedTypeInfo{
			BaseType:  info.BaseType,
			IsNonNull: info.IsNonNull && otherInfo.IsNonNull,
			IsMoved:   info.IsMoved || otherInfo.IsMoved,
		}

		// Only keep predicates present in both branches
		mergedInfo.Predicates = intersectStrings(info.Predicates, otherInfo.Predicates)

		// Combine narrowing locations from both branches
		mergedInfo.NarrowedAt = append(
			append([]string{}, info.NarrowedAt...),
			otherInfo.NarrowedAt...,
		)

		merged.Facts[varName] = mergedInfo
	}

	return merged
}

// getOrCreateLocal gets or creates a RefinedTypeInfo in the local Facts map.
// If the variable exists in a parent scope, copies it locally first.
func (ts *TypeState) getOrCreateLocal(varName string) *RefinedTypeInfo {
	if info, exists := ts.Facts[varName]; exists {
		return info
	}

	// Check if parent has info to copy
	var newInfo *RefinedTypeInfo
	if parentInfo := ts.Lookup(varName); parentInfo != nil {
		// Copy from parent so we don't mutate parent state
		newInfo = &RefinedTypeInfo{
			BaseType:   parentInfo.BaseType,
			IsNonNull:  parentInfo.IsNonNull,
			IsMoved:    parentInfo.IsMoved,
			Predicates: append([]string{}, parentInfo.Predicates...),
			NarrowedAt: append([]string{}, parentInfo.NarrowedAt...),
		}
	} else {
		newInfo = &RefinedTypeInfo{}
	}

	ts.Facts[varName] = newInfo
	return newInfo
}

// intersectStrings returns elements present in both slices.
func intersectStrings(a, b []string) []string {
	result := []string{}
	bSet := make(map[string]bool)
	for _, s := range b {
		bSet[s] = true
	}
	for _, s := range a {
		if bSet[s] {
			result = append(result, s)
		}
	}
	return result
}
