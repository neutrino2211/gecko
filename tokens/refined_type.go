package tokens

import (
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
)

// Predicate represents a constraint like `self > 0`
type Predicate struct {
	Operator string
	Value    any
	Pos      lexer.Position
}

// EffectSet represents effects on a function
type EffectSet struct {
	Throws  []*TypeRef
	IsAsync bool
	IsPure  bool
}

// TypeProvenance tracks where a type constraint came from
type TypeProvenance struct {
	Source      lexer.Position
	NarrowedBy  []NarrowingFact
}

// NarrowingFact represents a control flow condition that narrowed a type
type NarrowingFact struct {
	Condition string
	Pos       lexer.Position
}

// RefinedType wraps a TypeRef with refinement information
type RefinedType struct {
	Base       *TypeRef
	Predicates []*Predicate
	Effects    *EffectSet
	Provenance *TypeProvenance
}

// NewRefinedType creates a RefinedType from a base TypeRef
func NewRefinedType(base *TypeRef) *RefinedType {
	return &RefinedType{
		Base:       base,
		Predicates: []*Predicate{},
		Effects:    &EffectSet{},
		Provenance: &TypeProvenance{},
	}
}

// WithPredicate adds a predicate and returns the RefinedType for chaining
func (r *RefinedType) WithPredicate(op string, value any, pos lexer.Position) *RefinedType {
	r.Predicates = append(r.Predicates, &Predicate{
		Operator: op,
		Value:    value,
		Pos:      pos,
	})
	return r
}

// WithThrows adds a throws effect and returns the RefinedType for chaining
func (r *RefinedType) WithThrows(errType *TypeRef) *RefinedType {
	if r.Effects == nil {
		r.Effects = &EffectSet{}
	}
	r.Effects.Throws = append(r.Effects.Throws, errType)
	return r
}

// AddNarrowingFact records a narrowing fact from control flow analysis
func (r *RefinedType) AddNarrowingFact(condition string, pos lexer.Position) {
	if r.Provenance == nil {
		r.Provenance = &TypeProvenance{}
	}
	r.Provenance.NarrowedBy = append(r.Provenance.NarrowedBy, NarrowingFact{
		Condition: condition,
		Pos:       pos,
	})
}

// EvaluatePredicate evaluates a predicate against a concrete value.
// Supports int64 and float64 values for numeric comparisons.
// Returns true if the predicate is satisfied by the value.
func EvaluatePredicate(pred *Predicate, value any) bool {
	if pred == nil {
		return true
	}

	// Convert value to float64 for comparison
	var valueFloat float64
	switch v := value.(type) {
	case int64:
		valueFloat = float64(v)
	case float64:
		valueFloat = v
	case int:
		valueFloat = float64(v)
	case int32:
		valueFloat = float64(v)
	default:
		return false
	}

	// Convert predicate value to float64
	var predFloat float64
	switch pv := pred.Value.(type) {
	case int64:
		predFloat = float64(pv)
	case float64:
		predFloat = pv
	case int:
		predFloat = float64(pv)
	case int32:
		predFloat = float64(pv)
	default:
		return false
	}

	// Evaluate based on operator
	switch pred.Operator {
	case ">":
		return valueFloat > predFloat
	case "<":
		return valueFloat < predFloat
	case ">=":
		return valueFloat >= predFloat
	case "<=":
		return valueFloat <= predFloat
	case "==":
		return valueFloat == predFloat
	case "!=":
		return valueFloat != predFloat
	default:
		return false
	}
}

// SatisfiedBy checks if all predicates of this RefinedType are satisfied by the given value.
// Returns true if the value satisfies all predicates, or if there are no predicates.
func (r *RefinedType) SatisfiedBy(value any) bool {
	if r == nil || len(r.Predicates) == 0 {
		return true
	}

	for _, pred := range r.Predicates {
		if !EvaluatePredicate(pred, value) {
			return false
		}
	}
	return true
}

// InferPredicatesFromLiteral infers predicates from a literal value.
// For positive numbers: infers `> 0`
// For zero: infers `== 0`
// For negative numbers: infers `< 0`
func InferPredicatesFromLiteral(lit *Literal) []*Predicate {
	if lit == nil || lit.Number == "" {
		return nil
	}

	// Try to parse as float first (handles both int and float)
	numValue, err := strconv.ParseFloat(lit.Number, 64)
	if err != nil {
		// Try parsing as int (for hex, octal, etc.)
		intValue, err := strconv.ParseInt(lit.Number, 0, 64)
		if err != nil {
			return nil
		}
		numValue = float64(intValue)
	}

	var predicates []*Predicate
	pos := lit.Pos

	if numValue > 0 {
		predicates = append(predicates, &Predicate{
			Operator: ">",
			Value:    int64(0),
			Pos:      pos,
		})
	} else if numValue < 0 {
		predicates = append(predicates, &Predicate{
			Operator: "<",
			Value:    int64(0),
			Pos:      pos,
		})
	} else {
		predicates = append(predicates, &Predicate{
			Operator: "==",
			Value:    int64(0),
			Pos:      pos,
		})
	}

	return predicates
}
