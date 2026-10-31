package measurementprotocol

import "encoding/json"

const (
	MaxHeaderBytes = 16 * 1024
	MaxResultBytes = 16 * 1024
)

type Limits struct {
	MaxPDFPages        int64 `json:"max_pdf_pages"`
	MaxDOCXCharacters  int64 `json:"max_docx_characters"`
	MaxArchiveEntries  int64 `json:"max_archive_entries"`
	MaxExpandedBytes   int64 `json:"max_expanded_bytes"`
	MaxMemoryBytes     int64 `json:"max_memory_bytes"`
	MaxCPUMilliseconds int64 `json:"max_cpu_milliseconds"`
	MaxXMLDepth        int32 `json:"max_xml_depth"`
	MaxArchiveDepth    int32 `json:"max_archive_depth"`
	WallTimeoutMillis  int64 `json:"wall_timeout_milliseconds"`
}

type Request struct {
	MediaType string `json:"media_type"`
	ByteSize  int64  `json:"byte_size"`
	SHA256    string `json:"sha256"`
	Limits    Limits `json:"limits"`
}

type Result struct {
	OK               bool   `json:"ok"`
	FailureCode      string `json:"failure_code,omitempty"`
	MeasuredUnits    int64  `json:"measured_units,omitempty"`
	ConsumedBytes    int64  `json:"consumed_bytes,omitempty"`
	ConsumedSHA256   string `json:"consumed_sha256,omitempty"`
	ParserKey        string `json:"parser_key,omitempty"`
	ParserVersion    string `json:"parser_version,omitempty"`
	SemanticsVersion string `json:"semantics_version,omitempty"`
	ProfileVersion   string `json:"profile_version,omitempty"`
	Unit             string `json:"unit,omitempty"`
}

func MarshalHeader(request Request) ([]byte, error) {
	return json.Marshal(request)
}
