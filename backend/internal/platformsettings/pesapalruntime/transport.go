package pesapalruntime

import (
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

var errProvider = errors.New("pesapal_provider_unavailable")

func wipe(raw []byte) {
	for i := range raw {
		raw[i] = 0
	}
}
func (r *Runtime) send(ctx context.Context, c domain.PesaPalConfiguration, path, token string, body []byte) ([]byte, error) {
	method := "GET"
	if body != nil {
		method = "POST"
	}
	req, e := http.NewRequestWithContext(ctx, method, c.API()+path, bytes.NewReader(body))
	if e != nil {
		return nil, errProvider
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, e := r.client.Do(req)
	if e != nil {
		return nil, errProvider
	}
	if response == nil || response.Body == nil {
		return nil, errProvider
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || strings.Split(response.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return nil, errProvider
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, 65537))
	if e != nil || len(raw) > 65536 {
		return nil, errProvider
	}
	if !uniqueKeys(raw) {
		wipe(raw)
		return nil, errProvider
	}
	return raw, nil
}
func uniqueKeys(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, e := d.Token()
	if e != nil || first != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for d.More() {
		key, e := d.Token()
		s, ok := key.(string)
		if e != nil || !ok || seen[s] {
			return false
		}
		seen[s] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return false
		}
	}
	if _, e = d.Token(); e != nil {
		return false
	}
	var trailing any
	return d.Decode(&trailing) == io.EOF
}
func (r *Runtime) authenticate(ctx context.Context, c domain.PesaPalConfiguration) (token string, err error) {
	if r.secrets == nil {
		return "", errProvider
	}
	err = r.secrets.WithSecret(ctx, c.CredentialsSecretRef, func(raw []byte) error {
		credentials, e := smtpsecret.ParsePesaPalCredentials(raw)
		if e != nil {
			return errProvider
		}
		payload, e := json.Marshal(credentials)
		credentials = smtpsecret.PesaPalCredentials{}
		if e != nil {
			return errProvider
		}
		defer wipe(payload)
		response, e := r.send(ctx, c, "/Auth/RequestToken", "", payload)
		if e != nil {
			return e
		}
		defer wipe(response)
		var result struct {
			Token  string `json:"token"`
			Status string `json:"status"`
		}
		if json.Unmarshal(response, &result) != nil || result.Status != "200" || len(result.Token) < 8 || len(result.Token) > 8192 || strings.TrimSpace(result.Token) != result.Token || strings.ContainsAny(result.Token, "\r\n\x00") {
			return errProvider
		}
		token = result.Token
		return nil
	})
	if err != nil {
		return "", errProvider
	}
	return token, nil
}
