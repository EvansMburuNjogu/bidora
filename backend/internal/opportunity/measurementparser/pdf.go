package measurementparser

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/measurementprofile"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcore "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const mebibyte = int64(1 << 20)

func measurePDF(ctx context.Context, data []byte, limits application.MeasurementLimits) Result {
	result := Result{Unit: application.PDFPages, Semantics: measurementprofile.PDFSemantics}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		result.FailureCode = application.MeasurementUnsupportedFormat
		return result
	}
	if err := validatePDFTail(data); err != nil {
		result.FailureCode = application.MeasurementUnsupportedFormat
		return result
	}

	configuration := model.NewStatelessConfiguration()
	configuration.ValidationMode = model.ValidationStrict
	configuration.Offline = true
	configuration.ValidateLinks = false
	configuration.LocalImageRoot = ""
	configuration.DecodeAllStreams = true
	configuration.Limits = model.ResourceLimits{
		MaxInputBytes:        25 * mebibyte,
		MaxObjectBytes:       25 * mebibyte,
		MaxStreamBytes:       25 * mebibyte,
		MaxDecodeBytes:       100 * mebibyte,
		MaxImagePixels:       25_000_000,
		MaxImageBytes:        100 * mebibyte,
		MaxObjectCount:       250_000,
		MaxObjectStreamCount: 50_000,
		MaxObjectStreamFirst: 8 * mebibyte,
		MaxXRefEntries:       250_000,
		MaxRecursionDepth:    100,
	}

	count, err := api.PageCount(ctx, bytes.NewReader(data), configuration)
	if err != nil {
		result.FailureCode = classifyPDFError(ctx, err)
		return result
	}
	if count < 1 {
		result.FailureCode = application.MeasurementCorruptDocument
		return result
	}
	if int64(count) > limits.MaxPDFPages {
		result.FailureCode = application.MeasurementPageLimitExceeded
		return result
	}
	result.MeasuredUnits = int64(count)
	return result
}

func validatePDFTail(data []byte) error {
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		return errors.New("pdf_zip_polyglot")
	}
	trimmed := trimPDFWhitespace(data)
	const eofMarker = "%%EOF"
	if !bytes.HasSuffix(trimmed, []byte(eofMarker)) {
		return errors.New("pdf_final_eof_invalid")
	}
	beforeEOF := trimPDFWhitespace(trimmed[:len(trimmed)-len(eofMarker)])
	start := bytes.LastIndex(beforeEOF, []byte("startxref"))
	if start < 0 {
		return errors.New("pdf_startxref_missing")
	}
	offsetBytes := beforeEOF[start+len("startxref"):]
	if len(offsetBytes) == 0 || !isPDFWhitespace(offsetBytes[0]) {
		return errors.New("pdf_startxref_separator_invalid")
	}
	offsetBytes = trimPDFWhitespaceBoth(offsetBytes)
	if len(offsetBytes) == 0 {
		return errors.New("pdf_startxref_offset_missing")
	}
	for _, value := range offsetBytes {
		if value < '0' || value > '9' {
			return errors.New("pdf_startxref_epilogue_invalid")
		}
	}
	offset, err := strconv.ParseInt(string(offsetBytes), 10, 64)
	if err != nil || offset < 0 || offset >= int64(start) {
		return errors.New("pdf_startxref_offset_invalid")
	}
	priorEOF := bytes.LastIndex(beforeEOF[:start], []byte(eofMarker))
	if priorEOF >= 0 && offset <= int64(priorEOF+len(eofMarker)) {
		return errors.New("pdf_revision_not_monotonic")
	}
	if !validXRefTarget(data[offset:start]) {
		return errors.New("pdf_xref_target_invalid")
	}
	return nil
}

func validXRefTarget(target []byte) bool {
	if bytes.HasPrefix(target, []byte("xref")) {
		return true
	}
	index := consumeDecimal(target, 0)
	if index == 0 {
		return false
	}
	index = consumePDFWhitespace(target, index)
	generationStart := index
	index = consumeDecimal(target, index)
	if index == generationStart {
		return false
	}
	index = consumePDFWhitespace(target, index)
	return bytes.HasPrefix(target[index:], []byte("obj"))
}

func consumeDecimal(value []byte, index int) int {
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	return index
}

func consumePDFWhitespace(value []byte, index int) int {
	for index < len(value) && isPDFWhitespace(value[index]) {
		index++
	}
	return index
}

func trimPDFWhitespace(value []byte) []byte {
	for len(value) > 0 && isPDFWhitespace(value[len(value)-1]) {
		value = value[:len(value)-1]
	}
	return value
}

func trimPDFWhitespaceBoth(value []byte) []byte {
	value = trimPDFWhitespace(value)
	index := consumePDFWhitespace(value, 0)
	return value[index:]
}

func isPDFWhitespace(value byte) bool {
	switch value {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func classifyPDFError(ctx context.Context, err error) application.MeasurementFailureCode {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return timeoutCode(ctxErr)
	}
	if errors.Is(err, pdfcore.ErrEncrypted) || errors.Is(err, pdfcore.ErrWrongPassword) || errors.Is(err, pdfcore.ErrOwnerPasswordRequired) {
		return application.MeasurementEncryptedDocument
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "limit exceeded") || strings.Contains(message, "exceeds limit") || strings.Contains(message, "maximum ") && strings.Contains(message, " bytes") || strings.Contains(message, "max recursion depth") {
		return application.MeasurementResourceLimitExceeded
	}
	return application.MeasurementCorruptDocument
}
