package opportunity_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bidora/backend/internal/opportunity/scanclamav"
)

func TestClamAVScannerStreamsExactBytesAndMapsOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		response   string
		wantClean  bool
		wantCode   string
		wantErr    string
		expectSize int64
	}{
		{name: "clean", response: "stream: OK\x00", wantClean: true, wantCode: "CLEAN", expectSize: 12},
		{name: "malware", response: "stream: Eicar-Test-Signature FOUND\x00", wantCode: "MALWARE_DETECTED", expectSize: 12},
		{name: "scanner failure fails closed", response: "stream: ERROR\x00", wantErr: "clamav_scan_failed", expectSize: 12},
		{name: "declared size mismatch fails closed", response: "stream: OK\x00", wantErr: "clamav_size_mismatch", expectSize: 13},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := []byte("exact-object")
			dir, err := os.MkdirTemp("/tmp", "bidora-clam-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			socket := filepath.Join(dir, "clamd.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			serverErr := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					serverErr <- err
					return
				}
				defer conn.Close()
				command := make([]byte, len("zINSTREAM\x00"))
				if _, err = io.ReadFull(conn, command); err != nil {
					serverErr <- err
					return
				}
				var received []byte
				for {
					var header [4]byte
					if _, err = io.ReadFull(conn, header[:]); err != nil {
						if tc.wantErr == "clamav_size_mismatch" && err == io.EOF && bytes.Equal(received, payload) {
							serverErr <- nil
						} else {
							serverErr <- err
						}
						return
					}
					n := binary.BigEndian.Uint32(header[:])
					if n == 0 {
						break
					}
					chunk := make([]byte, n)
					if _, err = io.ReadFull(conn, chunk); err != nil {
						serverErr <- err
						return
					}
					received = append(received, chunk...)
				}
				if !bytes.Equal(command, []byte("zINSTREAM\x00")) || !bytes.Equal(received, payload) {
					serverErr <- io.ErrUnexpectedEOF
					return
				}
				_, err = io.WriteString(conn, tc.response)
				serverErr <- err
			}()

			scanner, err := scanclamav.New(scanclamav.Config{SocketPath: socket, Timeout: time.Second, ChunkBytes: 4096})
			if err != nil {
				t.Fatal(err)
			}
			out, err := scanner.Scan(context.Background(), bytes.NewReader(payload), tc.expectSize)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("Scan error = %v, want %s", err, tc.wantErr)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if out.Clean != tc.wantClean || out.ResultCode != tc.wantCode || out.Engine != "CLAMAV" {
					t.Fatalf("unexpected outcome: %+v", out)
				}
			}
			if err = <-serverErr; err != nil {
				t.Fatalf("synthetic clamd: %v", err)
			}
		})
	}
}
