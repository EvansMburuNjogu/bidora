package domain

import (
	_ "embed"
	"encoding/json"
	"slices"
	"time"
)

// One generated ISO/tzdb reference snapshot, shared with the installed UI data.
// These are selectable references, never automatically enabled markets.
//
//go:embed country-reference.generated.json
var countryReferenceJSON []byte

type CountryReference struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Currency  string   `json:"currency"`
	Language  string   `json:"language"`
	Languages []string `json:"languages"`
	Zone      string   `json:"zone"`
	Zones     []string `json:"zones"`
}

func CountryReferences() []CountryReference {
	var rows []CountryReference
	if json.Unmarshal(countryReferenceJSON, &rows) != nil {
		return nil
	}
	return rows
}

type CountryConfiguration struct {
	Code      string    `json:"country_code"`
	Name      string    `json:"name"`
	Currency  string    `json:"currency"`
	Language  string    `json:"language"`
	Timezone  string    `json:"timezone"`
	Locale    string    `json:"locale"`
	Enabled   bool      `json:"enabled"`
	Default   bool      `json:"is_default"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c CountryConfiguration) Valid() bool {
	if c.Version < 0 || c.Default && !c.Enabled || c.Code == "KE" && c.Currency != "KES" || c.Locale != c.Language+"-"+c.Code {
		return false
	}
	code, currency, language, zone := false, false, false, false
	for _, r := range CountryReferences() {
		code = code || r.Code == c.Code
		currency = currency || r.Currency == c.Currency
		language = language || slices.Contains(r.Languages, c.Language)
		zone = zone || slices.Contains(r.Zones, c.Timezone)
	}
	if !code || !currency || !language || !zone {
		return false
	}
	_, err := time.LoadLocation(c.Timezone)
	return err == nil
}
func (c CountryConfiguration) WithName() CountryConfiguration {
	for _, r := range CountryReferences() {
		if r.Code == c.Code {
			c.Name = r.Name
			break
		}
	}
	return c
}
