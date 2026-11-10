package emailhtml

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"html/template"
)

//go:embed assets/bidora-aperture-192.png
var logoPNG []byte

const LogoContentID = "bidora-logo@bidora"

// This trusted data URL renders in template previews without a public asset host.
// The SMTP transport substitutes a CID attachment for email-client compatibility.
var logoDataURL = template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG))

func LogoDataURL() template.URL { return logoDataURL }
func LogoPNG() []byte           { return bytes.Clone(logoPNG) }
