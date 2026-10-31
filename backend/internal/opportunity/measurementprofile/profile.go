package measurementprofile

import (
	"time"

	"bidora/backend/internal/opportunity/application"
)

const (
	ParserKey      = "BIDORA_GO_DOCUMENT_MEASURER"
	ParserVersion  = "1.0.0-pdfcpu-v0.16.3"
	ProfileVersion = "BIDORA_DOCUMENT_MEASUREMENT_PILOT_V1"
	PDFSemantics   = "BIDORA_PDF_PAGE_COUNT_V1"
	DOCXSemantics  = "BIDORA_DOCX_TEXT_V1"
)

func AttemptIdentity() application.MeasurementAttemptIdentity {
	return application.MeasurementAttemptIdentity{ParserKey: ParserKey, ParserVersion: ParserVersion, PDFSemantics: PDFSemantics, DOCXSemantics: DOCXSemantics, ProfileVersion: ProfileVersion}
}

func ApprovedLimits(l application.MeasurementLimits) bool {
	return l.Valid() &&
		l.MaxPDFPages == 2_000 &&
		l.MaxDOCXCharacters == 5_000_000 &&
		l.MaxArchiveEntries == 10_000 &&
		l.MaxExpandedBytes == 100<<20 &&
		l.MaxMemoryBytes == 512<<20 &&
		l.MaxCPUMilliseconds == 120_000 &&
		l.MaxXMLDepth == 256 &&
		l.MaxArchiveDepth == 1 &&
		l.WallTimeout == 180*time.Second
}
