package shacl

// ValidationReport represents a SHACL validation report (sh:ValidationReport).
type ValidationReport struct {
	Conforms bool
	Results  []ValidationResult
}

// ValidationResult represents a single validation result (sh:ValidationResult).
type ValidationResult struct {
	FocusNode                 Term
	ResultPath                Term
	Value                     Term
	SourceShape               Term
	SourceConstraintComponent Term
	ResultSeverity            Term
	ResultMessages            []Term
	Details                   []ValidationResult
}

// Standard SHACL severity levels.
var (
	SHViolation = IRI(SH + "Violation")
	SHWarning   = IRI(SH + "Warning")
	SHInfo      = IRI(SH + "Info")
)
