package tokens

import "github.com/alecthomas/participle/v2/lexer"

// ComputeRanges walks the AST and computes EndPos for all nodes
func (f *File) ComputeRanges() {
	for _, entry := range f.Entries {
		computeEntryRange(entry)
	}
}

func computeEntryRange(entry *Entry) {
	if entry == nil {
		return
	}

	// Compute ranges for nested structures first
	if entry.Method != nil {
		computeMethodRange(entry.Method)
		entry.EndPos = entry.Method.EndPos
	}
	if entry.Class != nil {
		computeClassRange(entry.Class)
		entry.EndPos = entry.Class.EndPos
	}
	if entry.Trait != nil {
		computeTraitRange(entry.Trait)
		entry.EndPos = entry.Trait.EndPos
	}
	if entry.Implementation != nil {
		computeImplementationRange(entry.Implementation)
		entry.EndPos = entry.Implementation.EndPos
	}
	if entry.If != nil {
		computeIfRange(entry.If)
		entry.EndPos = entry.If.EndPos
	}
	if entry.Loop != nil {
		computeLoopRange(entry.Loop)
		entry.EndPos = entry.Loop.EndPos
	}
	if entry.Field != nil {
		computeFieldRange(entry.Field)
		entry.EndPos = entry.Field.EndPos
	}
}

func computeMethodRange(m *Method) {
	if m == nil {
		return
	}

	// Process body entries
	for _, entry := range m.Value {
		computeEntryRange(entry)
	}

	// EndPos is after the last body entry, or estimate from method start
	if len(m.Value) > 0 {
		lastEntry := m.Value[len(m.Value)-1]
		m.EndPos = lastEntry.EndPos
		// Add 1 line for the closing brace
		m.EndPos.Line++
	} else {
		// No body - estimate end as same line (declaration only or empty body)
		m.EndPos = m.Pos
		m.EndPos.Line++
	}
}

func computeClassRange(c *Class) {
	if c == nil {
		return
	}

	var maxEnd lexer.Position = c.Pos

	for _, field := range c.Fields {
		if field.Method != nil {
			computeMethodRange(field.Method)
			if field.Method.EndPos.Line > maxEnd.Line {
				maxEnd = field.Method.EndPos
			}
		}
		if field.Field != nil {
			computeFieldRange(field.Field)
			if field.Field.EndPos.Line > maxEnd.Line {
				maxEnd = field.Field.EndPos
			}
		}
	}

	c.EndPos = maxEnd
	c.EndPos.Line++ // Account for closing brace
}

func computeTraitRange(t *Trait) {
	if t == nil {
		return
	}

	var maxEnd lexer.Position = t.Pos

	for _, field := range t.Fields {
		computeImplFieldRange(field)
		if field.EndPos.Line > maxEnd.Line {
			maxEnd = field.EndPos
		}
	}

	t.EndPos = maxEnd
	t.EndPos.Line++ // Account for closing brace
}

func computeImplementationRange(impl *Implementation) {
	if impl == nil {
		return
	}

	var maxEnd lexer.Position = impl.Pos

	for _, field := range impl.GetFields() {
		computeImplFieldRange(field)
		if field.EndPos.Line > maxEnd.Line {
			maxEnd = field.EndPos
		}
	}

	impl.EndPos = maxEnd
	impl.EndPos.Line++ // Account for closing brace
}

func computeImplFieldRange(f *ImplementationField) {
	if f == nil {
		return
	}

	for _, entry := range f.Value {
		computeEntryRange(entry)
	}

	if len(f.Value) > 0 {
		lastEntry := f.Value[len(f.Value)-1]
		f.EndPos = lastEntry.EndPos
		f.EndPos.Line++
	} else {
		f.EndPos = f.Pos
		f.EndPos.Line++
	}
}

func computeIfRange(i *If) {
	if i == nil {
		return
	}

	for _, entry := range i.Value {
		computeEntryRange(entry)
	}

	var maxEnd lexer.Position = i.Pos
	if len(i.Value) > 0 {
		maxEnd = i.Value[len(i.Value)-1].EndPos
	}

	// Process else-if chain
	if i.ElseIf != nil {
		computeElseIfRange(i.ElseIf)
		if i.ElseIf.EndPos.Line > maxEnd.Line {
			maxEnd = i.ElseIf.EndPos
		}
	}

	// Process else
	if i.Else != nil {
		computeElseRange(i.Else)
		if i.Else.EndPos.Line > maxEnd.Line {
			maxEnd = i.Else.EndPos
		}
	}

	i.EndPos = maxEnd
	i.EndPos.Line++
}

func computeElseIfRange(ei *ElseIf) {
	if ei == nil {
		return
	}

	for _, entry := range ei.Value {
		computeEntryRange(entry)
	}

	var maxEnd lexer.Position = ei.Pos
	if len(ei.Value) > 0 {
		maxEnd = ei.Value[len(ei.Value)-1].EndPos
	}

	if ei.ElseIf != nil {
		computeElseIfRange(ei.ElseIf)
		if ei.ElseIf.EndPos.Line > maxEnd.Line {
			maxEnd = ei.ElseIf.EndPos
		}
	}

	if ei.Else != nil {
		computeElseRange(ei.Else)
		if ei.Else.EndPos.Line > maxEnd.Line {
			maxEnd = ei.Else.EndPos
		}
	}

	ei.EndPos = maxEnd
	ei.EndPos.Line++
}

func computeElseRange(e *Else) {
	if e == nil {
		return
	}

	for _, entry := range e.Value {
		computeEntryRange(entry)
	}

	if len(e.Value) > 0 {
		e.EndPos = e.Value[len(e.Value)-1].EndPos
	} else {
		e.EndPos = e.Pos
	}
	e.EndPos.Line++
}

func computeLoopRange(l *Loop) {
	if l == nil {
		return
	}

	for _, entry := range l.Value {
		computeEntryRange(entry)
	}

	if len(l.Value) > 0 {
		l.EndPos = l.Value[len(l.Value)-1].EndPos
	} else {
		l.EndPos = l.Pos
	}
	l.EndPos.Line++
}

func computeFieldRange(f *Field) {
	if f == nil {
		return
	}
	// For simple fields, end is on the same line
	f.EndPos = f.Pos
}

// ContainsPosition checks if a position is within this token's range
func (b *baseToken) ContainsPosition(line, col int) bool {
	if line < b.Pos.Line || line > b.EndPos.Line {
		return false
	}
	if line == b.Pos.Line && col < b.Pos.Column {
		return false
	}
	if line == b.EndPos.Line && col > b.EndPos.Column {
		return false
	}
	return true
}

// ContainsLine checks if a line is within this token's range (simpler check)
func (b *baseToken) ContainsLine(line int) bool {
	return line >= b.Pos.Line && line <= b.EndPos.Line
}
