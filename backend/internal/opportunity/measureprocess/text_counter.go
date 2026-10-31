package measureprocess

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"unicode/utf8"

	"bidora/backend/internal/opportunity/application"
)

// TextCounter counts Unicode scalar values in the exact locally extracted
// CLEAN source text. PDF extraction uses only the explicitly configured
// Poppler pdftotext executable; DOCX uses the approved isolated measurer.
type TextCounter struct {
	pdfToText string
	documents *Measurer
}

func NewTextCounter(pdfToText string, documents *Measurer) (*TextCounter, error) {
	if pdfToText == "" || !filepath.IsAbs(pdfToText) || filepath.Clean(pdfToText) != pdfToText || documents == nil {
		return nil, ErrConfigurationInvalid
	}
	info, err := os.Lstat(pdfToText)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, ErrConfigurationInvalid
	}
	return &TextCounter{pdfToText: pdfToText, documents: documents}, nil
}

func (c *TextCounter) Count(ctx context.Context, reader io.Reader, request application.MeasurementRequest, limits application.MeasurementLimits) (int64, error) {
	if c == nil || ctx == nil || reader == nil || !request.Valid() || !limits.Valid() || limits.MaxDOCXCharacters > 5_000_000 {
		return 0, application.ErrDocumentMeasurementInvalid
	}
	if request.MediaType == application.DOCXMediaType {
		measurement, err := c.documents.Measure(ctx, reader, request, limits)
		if err != nil {
			return 0, err
		}
		if measurement.Unit != application.DOCXExtractedCharacters {
			return 0, application.ErrDocumentMeasurementMismatch
		}
		return measurement.MeasuredUnits, nil
	}
	if request.MediaType != application.PDFMediaType {
		return 0, application.ErrDocumentMeasurementInvalid
	}
	content, err := io.ReadAll(io.LimitReader(reader, request.ByteSize+1))
	if err != nil || int64(len(content)) != request.ByteSize || sha256.Sum256(content) != request.SHA256 {
		return 0, application.ErrDocumentMeasurementMismatch
	}
	executionContext, cancel := context.WithTimeout(ctx, limits.WallTimeout)
	defer cancel()
	command := exec.CommandContext(executionContext, c.pdfToText, "-enc", "UTF-8", "-nopgbrk", "-q", "-", "-")
	command.Dir = "/"
	command.Env = []string{}
	command.Stdin = bytes.NewReader(content)
	command.Stderr = io.Discard
	output := &boundedBuffer{remaining: int(limits.MaxDOCXCharacters*4 + 1)}
	command.Stdout = output
	if err = command.Run(); err != nil {
		if executionContext.Err() != nil {
			return 0, application.NewMeasurementFailure(application.MeasurementParserTimeout, executionContext.Err())
		}
		return 0, application.NewMeasurementFailure(application.MeasurementParserUnavailable, err)
	}
	text := output.Bytes()
	if !utf8.Valid(text) {
		return 0, application.NewMeasurementFailure(application.MeasurementCorruptDocument, errors.New("pdftotext output is not UTF-8"))
	}
	count := int64(utf8.RuneCount(text))
	if count < 1 {
		return 0, application.NewMeasurementFailure(application.MeasurementCorruptDocument, errors.New("pdftotext output is empty"))
	}
	if count > limits.MaxDOCXCharacters {
		return 0, application.NewMeasurementFailure(application.MeasurementCharacterLimitExceeded, nil)
	}
	return count, nil
}

var _ application.GenerationTextCounter = (*TextCounter)(nil)
