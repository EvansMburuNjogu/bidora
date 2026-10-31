package measureprocess

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/measurementprofile"
	"bidora/backend/internal/opportunity/measurementprotocol"
)

var ErrConfigurationInvalid = errors.New("document_measurer_process_config_invalid")

type Config struct {
	ExecutablePath string
}

type Measurer struct {
	executablePath string
}

func New(config Config) (*Measurer, error) {
	if config.ExecutablePath == "" || !filepath.IsAbs(config.ExecutablePath) || filepath.Clean(config.ExecutablePath) != config.ExecutablePath {
		return nil, ErrConfigurationInvalid
	}
	info, err := os.Lstat(config.ExecutablePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, ErrConfigurationInvalid
	}
	return &Measurer{executablePath: config.ExecutablePath}, nil
}

func (m *Measurer) Measure(ctx context.Context, reader io.Reader, request application.MeasurementRequest, limits application.MeasurementLimits) (application.DocumentMeasurement, error) {
	if m == nil || ctx == nil || reader == nil || !request.Valid() || !measurementprofile.ApprovedLimits(limits) {
		return application.DocumentMeasurement{}, application.ErrDocumentMeasurementInvalid
	}
	header, err := measurementprotocol.MarshalHeader(measurementprotocol.Request{
		MediaType: request.MediaType,
		ByteSize:  request.ByteSize,
		SHA256:    hex.EncodeToString(request.SHA256[:]),
		Limits: measurementprotocol.Limits{
			MaxPDFPages:        limits.MaxPDFPages,
			MaxDOCXCharacters:  limits.MaxDOCXCharacters,
			MaxArchiveEntries:  limits.MaxArchiveEntries,
			MaxExpandedBytes:   limits.MaxExpandedBytes,
			MaxMemoryBytes:     limits.MaxMemoryBytes,
			MaxCPUMilliseconds: limits.MaxCPUMilliseconds,
			MaxXMLDepth:        limits.MaxXMLDepth,
			MaxArchiveDepth:    limits.MaxArchiveDepth,
			WallTimeoutMillis:  limits.WallTimeout.Milliseconds(),
		},
	})
	if err != nil || len(header) == 0 || len(header) > measurementprotocol.MaxHeaderBytes {
		return application.DocumentMeasurement{}, application.ErrDocumentMeasurementInvalid
	}
	var prefix bytes.Buffer
	if err = binary.Write(&prefix, binary.BigEndian, uint32(len(header))); err != nil {
		return application.DocumentMeasurement{}, application.ErrDocumentMeasurementUnavailable
	}
	if _, err = prefix.Write(header); err != nil {
		return application.DocumentMeasurement{}, application.ErrDocumentMeasurementUnavailable
	}

	executionContext, cancel := context.WithTimeout(ctx, limits.WallTimeout)
	defer cancel()
	command := exec.CommandContext(executionContext, m.executablePath)
	command.Dir = "/"
	command.Env = []string{}
	command.Stdin = io.MultiReader(bytes.NewReader(prefix.Bytes()), reader)
	command.Stderr = io.Discard
	output := &boundedBuffer{remaining: measurementprotocol.MaxResultBytes}
	command.Stdout = output
	if err = command.Run(); err != nil {
		if executionContext.Err() != nil {
			return application.DocumentMeasurement{}, application.NewMeasurementFailure(application.MeasurementParserTimeout, executionContext.Err())
		}
		return application.DocumentMeasurement{}, application.NewMeasurementFailure(application.MeasurementParserUnavailable, err)
	}

	result, err := decodeResult(output.Bytes())
	if err != nil {
		return application.DocumentMeasurement{}, application.NewMeasurementFailure(application.MeasurementParserUnavailable, err)
	}
	if !result.OK {
		code := application.MeasurementFailureCode(result.FailureCode)
		if !code.Valid() {
			return application.DocumentMeasurement{}, application.NewMeasurementFailure(application.MeasurementParserUnavailable, errors.New("invalid measurement failure code"))
		}
		return application.DocumentMeasurement{}, application.NewMeasurementFailure(code, nil)
	}
	expectedSemantics := measurementprofile.PDFSemantics
	if request.MediaType == application.DOCXMediaType {
		expectedSemantics = measurementprofile.DOCXSemantics
	}
	if result.ParserKey != measurementprofile.ParserKey || result.ParserVersion != measurementprofile.ParserVersion || result.ProfileVersion != measurementprofile.ProfileVersion || result.SemanticsVersion != expectedSemantics || result.ConsumedBytes != request.ByteSize {
		return application.DocumentMeasurement{}, application.ErrDocumentMeasurementMismatch
	}
	consumedDigest, err := hex.DecodeString(result.ConsumedSHA256)
	if err != nil || len(consumedDigest) != sha256.Size {
		return application.DocumentMeasurement{}, application.ErrDocumentMeasurementMismatch
	}
	var digest [sha256.Size]byte
	copy(digest[:], consumedDigest)
	measurement, err := application.NewDocumentMeasurement(request, result.ParserKey, result.ParserVersion, result.SemanticsVersion, result.ProfileVersion, application.MeasurementUnit(result.Unit), result.MeasuredUnits, result.ConsumedBytes, digest)
	if err != nil {
		return application.DocumentMeasurement{}, err
	}
	if err = measurement.ValidateFor(request, limits); err != nil {
		return application.DocumentMeasurement{}, err
	}
	return measurement, nil
}

func decodeResult(value []byte) (measurementprotocol.Result, error) {
	var result measurementprotocol.Result
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(value), measurementprotocol.MaxResultBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return result, errors.New("measurement result has trailing content")
	}
	return result, nil
}

type boundedBuffer struct {
	buffer    bytes.Buffer
	remaining int
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	if len(value) > b.remaining {
		return 0, errors.New("measurement result exceeds limit")
	}
	n, err := b.buffer.Write(value)
	b.remaining -= n
	return n, err
}

func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }

var _ application.DocumentMeasurer = (*Measurer)(nil)
