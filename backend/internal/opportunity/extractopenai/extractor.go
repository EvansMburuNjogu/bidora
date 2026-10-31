package extractopenai

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"bidora/backend/internal/opportunity/application"
)

const endpoint = "https://api.openai.com/v1/responses"

type Config struct {
	APIKey, Model string
	RootCAs       *x509.CertPool
	Timeout       time.Duration
}

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED OpenAI config]") }

type Extractor struct {
	managed    ConfigurationReader
	key, model string
	client     *http.Client
}

func New(config Config) (*Extractor, error) {
	if config.APIKey == "" || strings.TrimSpace(config.APIKey) != config.APIKey || config.Model == "" || len(config.Model) > 200 || strings.TrimSpace(config.Model) != config.Model || config.RootCAs == nil || config.Timeout < 10*time.Second || config.Timeout > 15*time.Minute {
		return nil, errors.New("openai_config_invalid")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "api.openai.com", RootCAs: config.RootCAs}
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = config.Timeout
	transport.MaxIdleConns = 4
	transport.MaxIdleConnsPerHost = 2
	transport.MaxConnsPerHost = 4
	return &Extractor{key: config.APIKey, model: config.Model, client: &http.Client{Transport: &exactTransport{base: transport}, Timeout: config.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("openai_redirect_denied") }}}, nil
}
func (e *Extractor) Model() string {
	if e == nil {
		return ""
	}
	if e.managed != nil {
		return "MANAGED_OPENAI"
	} // constructor identity only; jobs resolve the actual model
	return e.model
}
func (e *Extractor) Extract(ctx context.Context, reader io.Reader, size int64, mediaType string) (application.ExtractionOutput, error) {
	if e != nil && e.managed != nil {
		instance, release, err := e.resolved(ctx)
		if err != nil {
			return application.ExtractionOutput{}, err
		}
		defer release()
		return instance.Extract(ctx, reader, size, mediaType)
	}
	if ctx == nil || reader == nil || size < 1 || size > application.MaxTenderDocumentBytes || (mediaType != application.PDFMediaType && mediaType != application.DOCXMediaType) {
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderConfirmedNoEffect, errors.New("openai_extract_invalid"))
	}
	bodyReader, bodyWriter := io.Pipe()
	writeErr := make(chan error, 1)
	go func() { writeErr <- writeRequest(bodyWriter, e.model, reader, size, mediaType); close(writeErr) }()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderConfirmedNoEffect, errors.New("openai_request_invalid"))
	}
	req.Header.Set("Authorization", "Bearer "+e.key)
	req.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(req)
	if err != nil {
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderAmbiguousEffect, errors.New("openai_unavailable"))
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, copyErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		closeErr := response.Body.Close()
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderAmbiguousEffect, errors.Join(errors.New("openai_rejected"), copyErr, closeErr))
	}
	limited := io.LimitReader(response.Body, 4*1024*1024+1)
	raw, readErr := io.ReadAll(limited)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || len(raw) > 4*1024*1024 {
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderAmbiguousEffect, errors.Join(readErr, closeErr, errors.New("openai_response_invalid")))
	}
	if err = <-writeErr; err != nil {
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderAmbiguousEffect, err)
	}
	output, err := parseResponse(raw)
	if err != nil {
		return application.ExtractionOutput{}, application.NewProviderExecutionError(application.ProviderAmbiguousEffect, err)
	}
	return output, nil
}
func writeRequest(dst *io.PipeWriter, model string, src io.Reader, size int64, mediaType string) (err error) {
	defer func() { _ = dst.CloseWithError(err) }()
	modelJSON, _ := json.Marshal(model)
	prefix := `{"model":` + string(modelJSON) + `,"store":false,"max_output_tokens":12000,"input":[{"role":"user","content":[{"type":"input_file","filename":"tender` + extension(mediaType) + `","file_data":"data:` + mediaType + `;base64,`
	if _, err = io.WriteString(dst, prefix); err != nil {
		return err
	}
	encoder := base64.NewEncoder(base64.StdEncoding, dst)
	written, err := io.Copy(encoder, io.LimitReader(src, size+1))
	closeErr := encoder.Close()
	if err != nil {
		return errors.New("openai_input_failed")
	}
	if closeErr != nil {
		return errors.New("openai_input_failed")
	}
	if written != size {
		return errors.New("openai_input_size_mismatch")
	}
	suffix := `"},{"type":"input_text","text":"Extract explicit tender requirements only. Do not infer missing obligations. Every item must include a short exact quote and an inspectable page or section location. Return an empty array when no supported requirement is found."}]}],"text":{"format":{"type":"json_schema","name":"tender_requirements","strict":true,"schema":{"type":"object","additionalProperties":false,"required":["requirements"],"properties":{"requirements":{"type":"array","maxItems":500,"items":{"type":"object","additionalProperties":false,"required":["requirement_text","category","mandatory","citation_page","citation_location","citation_quote","confidence"],"properties":{"requirement_text":{"type":"string","maxLength":10000},"category":{"type":"string","maxLength":100},"mandatory":{"type":"boolean"},"citation_page":{"type":["integer","null"],"minimum":1,"maximum":10000},"citation_location":{"type":"string","maxLength":500},"citation_quote":{"type":"string","maxLength":2000},"confidence":{"type":"number","minimum":0,"maximum":1}}}}}}}}}`
	_, err = io.WriteString(dst, suffix)
	return err
}
func parseResponse(raw []byte) (application.ExtractionOutput, error) {
	var envelope struct {
		ID     string `json:"id"`
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := strictJSON(raw, &envelope); err != nil || envelope.ID == "" {
		return application.ExtractionOutput{}, errors.New("openai_response_invalid")
	}
	var text string
	for _, item := range envelope.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" {
				if text != "" {
					return application.ExtractionOutput{}, errors.New("openai_response_invalid")
				}
				text = content.Text
			}
		}
	}
	if text == "" {
		return application.ExtractionOutput{}, errors.New("openai_response_invalid")
	}
	var payload struct {
		Requirements []struct {
			RequirementText  string  `json:"requirement_text"`
			Category         string  `json:"category"`
			Mandatory        bool    `json:"mandatory"`
			CitationPage     *int32  `json:"citation_page"`
			CitationLocation string  `json:"citation_location"`
			CitationQuote    string  `json:"citation_quote"`
			Confidence       float64 `json:"confidence"`
		} `json:"requirements"`
	}
	if err := strictJSON([]byte(text), &payload); err != nil {
		return application.ExtractionOutput{}, errors.New("openai_output_invalid")
	}
	out := application.ExtractionOutput{ProviderResponseID: envelope.ID, Suggestions: make([]application.ExtractionSuggestion, len(payload.Requirements))}
	for i, row := range payload.Requirements {
		page := int32(0)
		if row.CitationPage != nil {
			page = *row.CitationPage
		}
		out.Suggestions[i] = application.ExtractionSuggestion{RequirementText: row.RequirementText, Category: row.Category, Mandatory: row.Mandatory, CitationPage: page, CitationLocation: row.CitationLocation, CitationQuote: row.CitationQuote, Confidence: row.Confidence}
	}
	return out, nil
}
func strictJSON(raw []byte, target any) error {
	if err := rejectDuplicateJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bufio.NewReader(strings.NewReader(string(raw))))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing_json")
	}
	return nil
}
func rejectDuplicateJSON(raw []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := walkJSON(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing_json")
	}
	return nil
}
func walkJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid_json_key")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate_json_key")
			}
			seen[key] = struct{}{}
			if err = walkJSON(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid_json_object")
		}
	case '[':
		for decoder.More() {
			if err := walkJSON(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid_json_array")
		}
	default:
		return errors.New("invalid_json_delimiter")
	}
	return nil
}
func extension(media string) string {
	if media == application.PDFMediaType {
		return ".pdf"
	}
	return ".docx"
}

type exactTransport struct{ base http.RoundTripper }

func (t *exactTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != endpoint || r.Method != http.MethodPost || r.URL.Hostname() != "api.openai.com" {
		return nil, errors.New("openai_endpoint_denied")
	}
	return t.base.RoundTrip(r)
}
