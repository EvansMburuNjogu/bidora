package domain

import "fmt"

// CapturedFactsInput is untrusted manual capture, not a verified source or HTTP
// contract. Empty fields preserve unknown facts. References are descriptive;
// their format, existence and access must be checked by future application code.
type CapturedFactsInput struct {
	Title                   string
	BuyerName               string
	SourceURI               string
	SourceDocumentVersionID string
	SourceDateText          string
	SourceTimezone          string
	InterpretedDeadline     *Deadline
}

func (CapturedFactsInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED captured facts input]")
}

// CapturedFacts is an immutable, unverified value. It deliberately exposes no
// verification transition, workspace conversion or fetching operation. It is
// not a persisted source-version aggregate or proof of document accessibility.
type CapturedFacts struct {
	title, buyerName, sourceURI, sourceDocumentVersionID string
	sourceDateText, sourceTimezone                       string
	deadline                                             Deadline
	hasDeadline                                          bool
}

// CaptureFacts retains supplied wording without normalization or inference.
// A missing interpretation preserves raw ambiguous text rather than dropping
// evidence, inventing an instant or preventing capture of unknown facts.
func CaptureFacts(input CapturedFactsInput) (CapturedFacts, error) {
	var deadline Deadline
	if input.InterpretedDeadline != nil {
		deadline = *input.InterpretedDeadline // Own a value, never the caller's pointer.
		if _, known := deadline.Instant(); !known {
			return CapturedFacts{}, invalid("capture_deadline_unknown", nil)
		}
		if deadline.SourceText() != input.SourceDateText || deadline.SourceZone() != input.SourceTimezone {
			return CapturedFacts{}, invalid("capture_deadline_source_mismatch", nil)
		}
	}
	return CapturedFacts{
		title: input.Title, buyerName: input.BuyerName,
		sourceURI: input.SourceURI, sourceDocumentVersionID: input.SourceDocumentVersionID,
		sourceDateText: input.SourceDateText, sourceTimezone: input.SourceTimezone,
		deadline: deadline, hasDeadline: input.InterpretedDeadline != nil,
	}, nil
}

func (f CapturedFacts) Title() string                   { return f.title }
func (f CapturedFacts) BuyerName() string               { return f.buyerName }
func (f CapturedFacts) SourceURI() string               { return f.sourceURI }
func (f CapturedFacts) SourceDocumentVersionID() string { return f.sourceDocumentVersionID }
func (f CapturedFacts) SourceDateText() string          { return f.sourceDateText }
func (f CapturedFacts) SourceTimezone() string          { return f.sourceTimezone }
func (f CapturedFacts) Deadline() (Deadline, bool)      { return f.deadline, f.hasDeadline }
func (CapturedFacts) Verified() bool                    { return false }
func (CapturedFacts) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED captured facts]")
}
