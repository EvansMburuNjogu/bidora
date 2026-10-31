package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"time"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/measurementparser"
	"bidora/backend/internal/opportunity/measurementprofile"
	"bidora/backend/internal/opportunity/measurementprotocol"
)

func main() {
	result := execute(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(result); err != nil {
		os.Exit(1)
	}
}

func execute(input io.Reader) measurementprotocol.Result {
	reader := bufio.NewReaderSize(input, measurementprotocol.MaxHeaderBytes)
	var sizeBuffer [4]byte
	if _, err := io.ReadFull(reader, sizeBuffer[:]); err != nil {
		return failure(application.MeasurementParserUnavailable)
	}
	headerSize := binary.BigEndian.Uint32(sizeBuffer[:])
	if headerSize == 0 || headerSize > measurementprotocol.MaxHeaderBytes {
		return failure(application.MeasurementParserUnavailable)
	}
	header := make([]byte, int(headerSize))
	if _, err := io.ReadFull(reader, header); err != nil {
		return failure(application.MeasurementParserUnavailable)
	}
	request, limits, expectedHash, ok := decodeRequest(header)
	if !ok {
		return failure(application.MeasurementParserUnavailable)
	}

	ctx, cancel := context.WithTimeout(context.Background(), limits.WallTimeout)
	defer cancel()
	content, err := io.ReadAll(io.LimitReader(&cancellableReader{ctx: ctx, reader: reader}, request.ByteSize+1))
	if err != nil {
		if ctx.Err() != nil {
			return failure(application.MeasurementParserTimeout)
		}
		return failure(application.MeasurementParserUnavailable)
	}
	if int64(len(content)) != request.ByteSize {
		return failure(application.MeasurementInputSizeMismatch)
	}
	consumedHash := sha256.Sum256(content)
	if consumedHash != expectedHash {
		return failure(application.MeasurementInputHashMismatch)
	}

	measured := measurementparser.Measure(ctx, content, request.MediaType, limits)
	if measured.FailureCode.Valid() {
		return failure(measured.FailureCode)
	}
	return measurementprotocol.Result{
		OK:               true,
		MeasuredUnits:    measured.MeasuredUnits,
		ConsumedBytes:    int64(len(content)),
		ConsumedSHA256:   hex.EncodeToString(consumedHash[:]),
		ParserKey:        measurementprofile.ParserKey,
		ParserVersion:    measurementprofile.ParserVersion,
		SemanticsVersion: measured.Semantics,
		ProfileVersion:   measurementprofile.ProfileVersion,
		Unit:             string(measured.Unit),
	}
}

func decodeRequest(header []byte) (measurementprotocol.Request, application.MeasurementLimits, [32]byte, bool) {
	var request measurementprotocol.Request
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(header), int64(len(header))))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, application.MeasurementLimits{}, [32]byte{}, false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return request, application.MeasurementLimits{}, [32]byte{}, false
	}
	limits := application.MeasurementLimits{
		MaxPDFPages:        request.Limits.MaxPDFPages,
		MaxDOCXCharacters:  request.Limits.MaxDOCXCharacters,
		MaxArchiveEntries:  request.Limits.MaxArchiveEntries,
		MaxExpandedBytes:   request.Limits.MaxExpandedBytes,
		MaxMemoryBytes:     request.Limits.MaxMemoryBytes,
		MaxCPUMilliseconds: request.Limits.MaxCPUMilliseconds,
		MaxXMLDepth:        request.Limits.MaxXMLDepth,
		MaxArchiveDepth:    request.Limits.MaxArchiveDepth,
		WallTimeout:        time.Duration(request.Limits.WallTimeoutMillis) * time.Millisecond,
	}
	if request.ByteSize < 1 || request.ByteSize > application.MaxTenderDocumentBytes || (request.MediaType != application.PDFMediaType && request.MediaType != application.DOCXMediaType) || !measurementprofile.ApprovedLimits(limits) {
		return request, limits, [32]byte{}, false
	}
	digest, err := hex.DecodeString(request.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return request, limits, [32]byte{}, false
	}
	var expected [32]byte
	copy(expected[:], digest)
	return request, limits, expected, true
}

func failure(code application.MeasurementFailureCode) measurementprotocol.Result {
	return measurementprotocol.Result{OK: false, FailureCode: string(code)}
}

type cancellableReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *cancellableReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
