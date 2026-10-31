package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// MeasurementUnit is the only source-defined unit accepted by Commercial AI
// credit calculation. It is measurement evidence, not extracted content.
type MeasurementUnit string

const (
	PDFPages                MeasurementUnit = "PDF_PAGES"
	DOCXExtractedCharacters MeasurementUnit = "DOCX_EXTRACTED_CHARACTERS"
	CleanScanState                          = "CLEAN"
)

var ErrDocumentMeasurementInvalid = errors.New("document_measurement_invalid")
var ErrDocumentMeasurementMismatch = errors.New("document_measurement_mismatch")
var ErrDocumentMeasurementUnavailable = errors.New("document_measurement_unavailable")

type MeasurementFailureCode string

const (
	MeasurementInputHashMismatch      MeasurementFailureCode = "INPUT_HASH_MISMATCH"
	MeasurementInputSizeMismatch      MeasurementFailureCode = "INPUT_SIZE_MISMATCH"
	MeasurementUnsupportedFormat      MeasurementFailureCode = "UNSUPPORTED_OR_MISMATCHED_FORMAT"
	MeasurementEncryptedDocument      MeasurementFailureCode = "ENCRYPTED_DOCUMENT"
	MeasurementCorruptDocument        MeasurementFailureCode = "CORRUPT_DOCUMENT"
	MeasurementPageLimitExceeded      MeasurementFailureCode = "PAGE_LIMIT_EXCEEDED"
	MeasurementCharacterLimitExceeded MeasurementFailureCode = "CHARACTER_LIMIT_EXCEEDED"
	MeasurementArchiveLimitExceeded   MeasurementFailureCode = "ARCHIVE_LIMIT_EXCEEDED"
	MeasurementXMLLimitExceeded       MeasurementFailureCode = "XML_LIMIT_EXCEEDED"
	MeasurementResourceLimitExceeded  MeasurementFailureCode = "RESOURCE_LIMIT_EXCEEDED"
	MeasurementParserTimeout          MeasurementFailureCode = "PARSER_TIMEOUT"
	MeasurementParserUnavailable      MeasurementFailureCode = "PARSER_UNAVAILABLE"
)

func (c MeasurementFailureCode) Valid() bool {
	switch c {
	case MeasurementInputHashMismatch, MeasurementInputSizeMismatch, MeasurementUnsupportedFormat, MeasurementEncryptedDocument, MeasurementCorruptDocument, MeasurementPageLimitExceeded, MeasurementCharacterLimitExceeded, MeasurementArchiveLimitExceeded, MeasurementXMLLimitExceeded, MeasurementResourceLimitExceeded, MeasurementParserTimeout, MeasurementParserUnavailable:
		return true
	}
	return false
}

// MeasurementFailure exposes only a stable count-only failure code. The
// diagnostic cause remains available to trusted callers through errors.Unwrap
// and must never be returned through HTTP, events or ordinary logs.
type MeasurementFailure struct {
	Code  MeasurementFailureCode
	cause error
}

func NewMeasurementFailure(code MeasurementFailureCode, cause error) error {
	if !code.Valid() {
		return ErrDocumentMeasurementInvalid
	}
	return &MeasurementFailure{Code: code, cause: cause}
}

func (e *MeasurementFailure) Error() string {
	if e == nil || !e.Code.Valid() {
		return ErrDocumentMeasurementUnavailable.Error()
	}
	return "document_measurement_failed:" + string(e.Code)
}

func (e *MeasurementFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func MeasurementFailureCodeOf(err error) (MeasurementFailureCode, bool) {
	var failure *MeasurementFailure
	if !errors.As(err, &failure) || failure == nil || !failure.Code.Valid() {
		return "", false
	}
	return failure.Code, true
}

type MeasurementRequest struct {
	TenantID, DocumentVersionID ID
	MediaType, ScanState        string
	ByteSize                    int64
	SHA256                      [32]byte
}

func (r MeasurementRequest) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED document measurement request]")
}

// MeasurementLimits must be supplied from an approved parser profile. This
// package intentionally provides no fallback values.
type MeasurementLimits struct {
	MaxPDFPages, MaxDOCXCharacters, MaxArchiveEntries, MaxExpandedBytes int64
	MaxMemoryBytes, MaxCPUMilliseconds                                  int64
	MaxXMLDepth, MaxArchiveDepth                                        int32
	WallTimeout                                                         time.Duration
}

func (l MeasurementLimits) Valid() bool {
	return l.MaxPDFPages > 0 && l.MaxDOCXCharacters > 0 && l.MaxArchiveEntries > 0 && l.MaxExpandedBytes > 0 && l.MaxMemoryBytes > 0 && l.MaxCPUMilliseconds > 0 && l.MaxXMLDepth > 0 && l.MaxArchiveDepth > 0 && l.WallTimeout > 0
}

type DocumentMeasurement struct {
	TenantID, DocumentVersionID         ID
	MediaType, ParserKey, ParserVersion string
	SemanticsVersion, ProfileVersion    string
	Unit                                MeasurementUnit
	MeasuredUnits, ByteSize             int64
	SHA256, ApplicabilityHash           [32]byte
}

func (m DocumentMeasurement) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED document measurement]")
}

// DocumentMeasurer is an outbound adapter boundary. Implementations must run
// without network access or credentials, consume exactly ByteSize bytes, hash
// those bytes, enforce the supplied limits, and return no extracted document
// text through this contract.
type DocumentMeasurer interface {
	Measure(context.Context, io.Reader, MeasurementRequest, MeasurementLimits) (DocumentMeasurement, error)
}

func NewMeasurementRequest(tenantID, documentVersionID ID, mediaType, scanState string, byteSize int64, digest [32]byte) (MeasurementRequest, error) {
	request := MeasurementRequest{TenantID: tenantID, DocumentVersionID: documentVersionID, MediaType: mediaType, ScanState: scanState, ByteSize: byteSize, SHA256: digest}
	if !request.Valid() {
		return MeasurementRequest{}, ErrDocumentMeasurementInvalid
	}
	return request, nil
}

func (r MeasurementRequest) Valid() bool {
	return !r.TenantID.IsZero() && !r.DocumentVersionID.IsZero() && (r.MediaType == PDFMediaType || r.MediaType == DOCXMediaType) && r.ScanState == CleanScanState && r.ByteSize > 0 && r.ByteSize <= MaxTenderDocumentBytes && r.SHA256 != [32]byte{}
}

func NewDocumentMeasurement(request MeasurementRequest, parserKey, parserVersion, semanticsVersion, profileVersion string, unit MeasurementUnit, measuredUnits, consumedBytes int64, consumedHash [32]byte) (DocumentMeasurement, error) {
	if !request.Valid() || !safeMeasurementLabel(parserKey, 100) || !safeMeasurementLabel(parserVersion, 100) || !safeMeasurementLabel(semanticsVersion, 100) || !safeMeasurementLabel(profileVersion, 100) || unit != unitForMedia(request.MediaType) || measuredUnits < 1 || consumedBytes != request.ByteSize || consumedHash != request.SHA256 {
		return DocumentMeasurement{}, ErrDocumentMeasurementMismatch
	}
	measurement := DocumentMeasurement{TenantID: request.TenantID, DocumentVersionID: request.DocumentVersionID, MediaType: request.MediaType, ParserKey: parserKey, ParserVersion: parserVersion, SemanticsVersion: semanticsVersion, ProfileVersion: profileVersion, Unit: unit, MeasuredUnits: measuredUnits, ByteSize: consumedBytes, SHA256: consumedHash}
	applicability, err := measurementApplicability(measurement)
	if err != nil {
		return DocumentMeasurement{}, ErrDocumentMeasurementInvalid
	}
	measurement.ApplicabilityHash = applicability
	return measurement, nil
}

func (m DocumentMeasurement) ValidateFor(request MeasurementRequest, limits MeasurementLimits) error {
	if !request.Valid() || !limits.Valid() || m.TenantID != request.TenantID || m.DocumentVersionID != request.DocumentVersionID || m.MediaType != request.MediaType || m.ByteSize != request.ByteSize || m.SHA256 != request.SHA256 || m.Unit != unitForMedia(request.MediaType) || m.MeasuredUnits < 1 || !safeMeasurementLabel(m.ParserKey, 100) || !safeMeasurementLabel(m.ParserVersion, 100) || !safeMeasurementLabel(m.SemanticsVersion, 100) || !safeMeasurementLabel(m.ProfileVersion, 100) {
		return ErrDocumentMeasurementMismatch
	}
	expected, err := measurementApplicability(m)
	if err != nil || m.ApplicabilityHash != expected {
		return ErrDocumentMeasurementMismatch
	}
	if (m.Unit == PDFPages && m.MeasuredUnits > limits.MaxPDFPages) || (m.Unit == DOCXExtractedCharacters && m.MeasuredUnits > limits.MaxDOCXCharacters) {
		return ErrDocumentMeasurementMismatch
	}
	return nil
}

func measurementApplicability(m DocumentMeasurement) ([32]byte, error) {
	canonical, err := json.Marshal(struct {
		TenantID, DocumentVersionID                                           ID
		MediaType, ParserKey, ParserVersion, SemanticsVersion, ProfileVersion string
		Unit                                                                  MeasurementUnit
		MeasuredUnits, ByteSize                                               int64
		SHA256                                                                [32]byte
	}{m.TenantID, m.DocumentVersionID, m.MediaType, m.ParserKey, m.ParserVersion, m.SemanticsVersion, m.ProfileVersion, m.Unit, m.MeasuredUnits, m.ByteSize, m.SHA256})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(append([]byte("document-measurement:v1:"), canonical...)), nil
}

func unitForMedia(mediaType string) MeasurementUnit {
	if mediaType == PDFMediaType {
		return PDFPages
	}
	if mediaType == DOCXMediaType {
		return DOCXExtractedCharacters
	}
	return ""
}

func safeMeasurementLabel(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
