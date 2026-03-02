package shacl

import (
	"regexp"
	"unicode/utf8"
)

// MinLengthConstraint implements sh:minLength.
type MinLengthConstraint struct{ MinLength int }

// MaxLengthConstraint implements sh:maxLength.
type MaxLengthConstraint struct{ MaxLength int }

func (c *MinLengthConstraint) ComponentIRI() string {
	return SH + "MinLengthConstraintComponent"
}
func (c *MaxLengthConstraint) ComponentIRI() string {
	return SH + "MaxLengthConstraintComponent"
}

func stringLen(t Term) (int, bool) {
	if t.IsIRI() {
		return utf8.RuneCountInString(t.Value()), true
	}
	if t.IsBlank() {
		return 0, false // blank nodes have no defined string length
	}
	return utf8.RuneCountInString(t.Value()), true
}

func (c *MinLengthConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		l, ok := stringLen(vn)
		if !ok || l < c.MinLength {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

func (c *MaxLengthConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		l, ok := stringLen(vn)
		if !ok || l > c.MaxLength {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// PatternConstraint implements sh:pattern.
// The regex is compiled once at construction time.
type PatternConstraint struct {
	Pattern string
	Flags   string
	re      *regexp.Regexp
}

// NewPatternConstraint creates a PatternConstraint with a pre-compiled regex.
// Returns nil if the pattern is invalid.
func NewPatternConstraint(pattern, flags string) *PatternConstraint {
	pat := pattern
	if flags != "" {
		pat = "(?" + flags + ")" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil
	}
	return &PatternConstraint{Pattern: pattern, Flags: flags, re: re}
}

func (c *PatternConstraint) ComponentIRI() string {
	return SH + "PatternConstraintComponent"
}

func (c *PatternConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !c.re.MatchString(vn.Value()) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// LanguageInConstraint implements sh:languageIn.
type LanguageInConstraint struct {
	Languages []string
}

func (c *LanguageInConstraint) ComponentIRI() string {
	return SH + "LanguageInConstraintComponent"
}

func (c *LanguageInConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !vn.IsLiteral() || !matchesLanguage(vn.Language(), c.Languages) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

func matchesLanguage(lang string, allowed []string) bool {
	if lang == "" {
		return false
	}
	for _, a := range allowed {
		if langMatches(lang, a) {
			return true
		}
	}
	return false
}

func langMatches(lang, tag string) bool {
	if lang == tag {
		return true
	}
	if len(lang) > len(tag) && lang[:len(tag)] == tag && lang[len(tag)] == '-' {
		return true
	}
	return false
}

// UniqueLangConstraint implements sh:uniqueLang.
type UniqueLangConstraint struct {
	UniqueLang bool
}

func (c *UniqueLangConstraint) ComponentIRI() string {
	return SH + "UniqueLangConstraintComponent"
}

func (c *UniqueLangConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if !c.UniqueLang {
		return nil
	}
	var results []ValidationResult
	langCount := make(map[string]int)
	for _, vn := range valueNodes {
		if !vn.IsLiteral() {
			continue
		}
		lang := vn.Language()
		if lang == "" {
			continue
		}
		langCount[lang]++
	}
	for _, count := range langCount {
		if count > 1 {
			// No sh:value for uniqueLang violations
			results = append(results, makeResult(shape, focusNode, Term{}, c.ComponentIRI()))
		}
	}
	return results
}
