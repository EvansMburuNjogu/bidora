package measurementparser

import (
	"context"
	"errors"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/measurementprofile"
)

type Result struct {
	MeasuredUnits int64
	Unit          application.MeasurementUnit
	Semantics     string
	FailureCode   application.MeasurementFailureCode
}

func Measure(ctx context.Context, data []byte, mediaType string, limits application.MeasurementLimits) Result {
	if ctx == nil || !measurementprofile.ApprovedLimits(limits) {
		return Result{FailureCode: application.MeasurementParserUnavailable}
	}
	if err := ctx.Err(); err != nil {
		return Result{FailureCode: timeoutCode(err)}
	}
	switch mediaType {
	case application.PDFMediaType:
		return measurePDF(ctx, data, limits)
	case application.DOCXMediaType:
		return measureDOCX(ctx, data, limits)
	default:
		return Result{FailureCode: application.MeasurementUnsupportedFormat}
	}
}

func timeoutCode(err error) application.MeasurementFailureCode {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return application.MeasurementParserTimeout
	}
	return application.MeasurementParserUnavailable
}
