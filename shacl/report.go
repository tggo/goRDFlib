package shacl

// ValidationReport represents a SHACL validation report (sh:ValidationReport).
type ValidationReport struct {
	Conforms bool
	Results  []ValidationResult

	// Err is set only by the entry points that have no error result (Validate,
	// Prepared.Validate, CompiledShapes.Validate), when the run failed: the
	// data or shapes graph could not be read from its store, or a query
	// failed. Such a report has Conforms false and no Results, and says
	// nothing about the data; check Err before trusting it. The same error
	// goes to WithErrorHandler. The *Context variants return it as their
	// error instead and leave Err nil.
	Err error
}

// failedReport is the report a no-error entry point returns for a failed run.
func failedReport(err error) ValidationReport {
	return ValidationReport{Err: err}
}

// ValidationResult represents a single validation result (sh:ValidationResult).
type ValidationResult struct {
	FocusNode                 Term
	ResultPath                Term
	Value                     Term
	SourceShape               Term
	SourceConstraintComponent Term
	SourceConstraint          Term
	ResultSeverity            Term
	ResultMessages            []Term
	Details                   []ValidationResult

	// SourceLine is the 1-based line in the parsed source that this result
	// blames, or 0 when none could be determined. It is filled in only when
	// Validate was given WithSourceLines.
	//
	// SHACL itself has no notion of a source line — a validation report is
	// about a graph, and a graph has no text — so this is an extension, and it
	// is not part of the RDF form of the report.
	SourceLine int

	// SourceLineKind says whether SourceLine is the offending triple's own line
	// or the focus node's, which is the difference between "this value is
	// wrong" and "something is missing from this resource".
	SourceLineKind SourceLineKind
}

// Standard SHACL severity levels.
var (
	SHViolation = IRI(SH + "Violation")
	SHWarning   = IRI(SH + "Warning")
	SHInfo      = IRI(SH + "Info")
)
