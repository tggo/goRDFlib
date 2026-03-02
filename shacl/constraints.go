package shacl

func makeResult(shape *Shape, focusNode Term, value Term, component string) ValidationResult {
	r := ValidationResult{
		FocusNode:                 focusNode,
		Value:                     value,
		SourceShape:               shape.ID,
		SourceConstraintComponent: IRI(component),
		ResultSeverity:            shape.Severity,
	}
	if shape.Path != nil {
		r.ResultPath = pathToTerm(shape.Path)
	}
	if len(shape.Messages) > 0 {
		r.ResultMessages = shape.Messages
	}
	return r
}
