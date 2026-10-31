// Package smtpsecret validates externally resolved SMTP secret material.
package smtpsecret

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

type Resolver interface {
	WithSecret(context.Context, string, func([]byte) error) error
}

type Credentials struct{ Username, Password string }

func ParseCredentials(raw []byte) (Credentials, error) {
	if len(raw) == 0 || len(raw) > 4096 || !utf8.Valid(raw) {
		return Credentials{}, errors.New("smtp_credentials_invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return Credentials{}, errors.New("smtp_credentials_invalid")
	}
	values := map[string]string{}
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		key, ok := keyToken.(string)
		if keyErr != nil || !ok || (key != "username" && key != "password") {
			return Credentials{}, errors.New("smtp_credentials_invalid")
		}
		if _, duplicate := values[key]; duplicate {
			return Credentials{}, errors.New("smtp_credentials_invalid")
		}
		var value string
		if decoder.Decode(&value) != nil {
			return Credentials{}, errors.New("smtp_credentials_invalid")
		}
		values[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return Credentials{}, errors.New("smtp_credentials_invalid")
	}
	if decoder.Decode(&struct{}{}) != io.EOF || len(values) != 2 || values["username"] == "" || len(values["username"]) > 256 || strings.TrimSpace(values["username"]) != values["username"] || values["password"] == "" || len(values["password"]) > 1024 || strings.ContainsRune(values["password"], 0) {
		return Credentials{}, errors.New("smtp_credentials_invalid")
	}
	return Credentials{Username: values["username"], Password: values["password"]}, nil
}

func ParseTrustRoots(raw []byte) (*x509.CertPool, error) {
	roots := x509.NewCertPool()
	if len(raw) == 0 || len(raw) > 1<<20 || !roots.AppendCertsFromPEM(raw) {
		return nil, errors.New("smtp_trust_roots_invalid")
	}
	return roots, nil
}
