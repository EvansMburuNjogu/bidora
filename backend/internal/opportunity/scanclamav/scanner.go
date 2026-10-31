package scanclamav

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"bidora/backend/internal/opportunity/application"
)

type Config struct {
	SocketPath string
	Timeout    time.Duration
	ChunkBytes int
}
type Scanner struct{ config Config }

func New(config Config) (*Scanner, error) {
	if config.SocketPath == "" || config.SocketPath[0] != '/' || config.Timeout < time.Second || config.Timeout > 2*time.Minute || config.ChunkBytes < 4096 || config.ChunkBytes > 1024*1024 {
		return nil, errors.New("clamav_config_invalid")
	}
	return &Scanner{config}, nil
}
func (s *Scanner) Scan(ctx context.Context, reader io.Reader, expectedSize int64) (application.ScanOutcome, error) {
	if ctx == nil || reader == nil || expectedSize < 1 || expectedSize > application.MaxTenderDocumentBytes {
		return application.ScanOutcome{}, errors.New("clamav_scan_invalid")
	}
	dialer := net.Dialer{Timeout: s.config.Timeout}
	conn, err := dialer.DialContext(ctx, "unix", s.config.SocketPath)
	if err != nil {
		return application.ScanOutcome{}, errors.New("clamav_unavailable")
	}
	defer conn.Close()
	deadline := time.Now().Add(s.config.Timeout)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return application.ScanOutcome{}, errors.New("clamav_unavailable")
	}
	if _, err = io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return application.ScanOutcome{}, errors.New("clamav_unavailable")
	}
	buffer := make([]byte, s.config.ChunkBytes)
	var total int64
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			total += int64(n)
			if total > expectedSize {
				return application.ScanOutcome{}, errors.New("clamav_size_mismatch")
			}
			var header [4]byte
			binary.BigEndian.PutUint32(header[:], uint32(n))
			if _, err = conn.Write(header[:]); err != nil {
				return application.ScanOutcome{}, errors.New("clamav_unavailable")
			}
			if _, err = conn.Write(buffer[:n]); err != nil {
				return application.ScanOutcome{}, errors.New("clamav_unavailable")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return application.ScanOutcome{}, errors.New("clamav_input_failed")
		}
	}
	if total != expectedSize {
		return application.ScanOutcome{}, errors.New("clamav_size_mismatch")
	}
	if _, err = conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return application.ScanOutcome{}, errors.New("clamav_unavailable")
	}
	response, err := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString(0)
	if err != nil && err != io.EOF {
		return application.ScanOutcome{}, errors.New("clamav_unavailable")
	}
	response = strings.TrimSuffix(response, "\x00")
	switch {
	case strings.HasSuffix(response, " OK"):
		return application.ScanOutcome{Clean: true, ResultCode: "CLEAN", Engine: "CLAMAV"}, nil
	case strings.HasSuffix(response, " FOUND"):
		return application.ScanOutcome{Clean: false, ResultCode: "MALWARE_DETECTED", Engine: "CLAMAV"}, nil
	default:
		return application.ScanOutcome{}, errors.New("clamav_scan_failed")
	}
}
