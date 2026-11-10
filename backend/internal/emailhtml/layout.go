// Package emailhtml provides a provider-independent, escaped email presentation.
package emailhtml

import (
	"bidora/backend/internal/invitationcode"
	"bytes"
	"html/template"
	"net/url"
	"strings"
	"time"
)

type Content struct {
	Brand, Title, Body, Code, Expiry, Origin, Key, SupportEmail string
	ChallengeRef                                                string
	RecoveryUserID                                              string
}

type Action struct{ Label, URL string }

// QuickAction uses fixed token-free paths on an explicit HTTPS origin, or a
// loopback HTTP origin for local development. Runtime configuration owns the origin.
func QuickAction(origin, key string) Action {
	u, err := url.Parse(origin)
	localHTTP := err == nil && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" || strings.EqualFold(u.Hostname(), "localhost"))
	if err != nil || (u.Scheme != "https" && !localHTTP) || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return Action{}
	}
	path, label := "/account/security", "Review account security"
	switch key {
	case "identity.login_code":
		path, label = "/auth/login-code", "Continue sign-in"
	case "identity.email_verification":
		path, label = "/verify-email", "Verify email"
	case "identity.password_recovery":
		path, label = "/reset-password", "Reset password"
	case "tenant.invitation":
		path, label = "/invitations/tenant", "Open invitation"
	case "platform.invitation.issued", "platform.invitation.reminder":
		path, label = "/invitations/platform", "Open invitation"
	}
	u.Path = path
	return Action{label, u.String()}
}

func Render(c Content) string {
	if c.Brand == "" {
		c.Brand = "Bidora"
	}
	if expiry, err := time.Parse(time.RFC3339, c.Expiry); err == nil {
		c.Expiry = expiry.UTC().Format("2 Jan 2006, 15:04 UTC")
	}
	logoURL, fontURL := LogoDataURL(), ""
	if action := QuickAction(c.Origin, c.Key); action.URL != "" {
		u, _ := url.Parse(action.URL)
		u.Path = "/fonts/manrope-latin.woff2"
		fontURL = u.String()
	}
	action := ActionFor(c)
	if c.Key == "platform.invitation.issued" && action.URL != "" {
		c.Code = ""
	} else if c.Key == "tenant.invitation" {
		c.Code = invitationcode.Display(c.Code)
	}
	data := struct {
		Content
		Lines   []string
		Action  Action
		LogoURL template.URL
		FontURL string
		Safety  string
	}{c, strings.Split(c.Body, "\n"), action, logoURL, fontURL, SafetyNote(c.Key)}
	var out bytes.Buffer
	if err := layout.Execute(&out, data); err != nil {
		return ""
	}
	return out.String()
}

// SafetyNote is shared by HTML and plain-text emails without repeated boilerplate.
func SafetyNote(key string) string {
	switch key {
	case "platform.invitation.issued", "platform.invitation.reminder":
		return "Keep your invitation link private. If you did not expect this invitation, ignore this email."
	case "identity.login_code", "identity.email_verification", "identity.password_recovery", "tenant.invitation":
		return "Never share your code. Didn't request this? Ignore this email."
	default:
		return "If this wasn't you, contact support."
	}
}

var layout = template.Must(template.New("notification").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><title>{{.Title}}</title>
{{if .FontURL}}<style>@font-face{font-family:'Manrope';font-style:normal;font-weight:200 800;font-display:swap;src:url('{{.FontURL}}') format('woff2')}</style>{{end}}
</head><body style="margin:0;padding:0;background:#f7f8f5;color:#17212b;font-family:'Manrope',Arial,Helvetica,sans-serif;-webkit-text-size-adjust:100%">
<div style="display:none;max-height:0;overflow:hidden;opacity:0">{{.Title}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f7f8f5;font-family:'Manrope',Arial,Helvetica,sans-serif"><tr><td align="center" style="padding:40px 16px">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;background:#ffffff;border:1px solid #d8dfdc;border-radius:12px">
<tr><td style="padding:28px 28px 0"><table role="presentation" cellpadding="0" cellspacing="0"><tr>{{if .LogoURL}}<td style="padding-right:10px;vertical-align:middle"><img src="{{.LogoURL}}" width="36" height="36" alt="Bidora logo" style="display:block;border:0;width:36px;height:36px"></td>{{end}}<td style="vertical-align:middle;font-family:'Manrope',Arial,Helvetica,sans-serif;font-size:22px;line-height:30px;font-weight:700;letter-spacing:-.6px;color:#173b3d">{{.Brand}}</td></tr></table></td></tr>
<tr><td style="padding:28px;font-family:'Manrope',Arial,Helvetica,sans-serif"><h1 style="margin:0 0 10px;font-size:22px;line-height:30px;font-weight:650;letter-spacing:-.5px;color:#17212b">{{.Title}}</h1>
{{range .Lines}}{{if .}}<p style="margin:0 0 12px;font-size:14px;line-height:23px;color:#52636b;overflow-wrap:anywhere;word-break:break-word">{{.}}</p>{{end}}{{end}}
{{if .Code}}<div style="margin:24px 0;padding:20px 16px;background:#e4f0ed;border-radius:8px;text-align:center"><div style="font-family:'Manrope',Arial,Helvetica,sans-serif;font-size:{{if le (len .Code) 6}}32{{else}}14{{end}}px;line-height:{{if le (len .Code) 6}}44{{else}}24{{end}}px;font-weight:650;letter-spacing:{{if le (len .Code) 6}}6{{else}}.4{{end}}px;font-variant-numeric:tabular-nums;word-break:break-all;color:#173b3d">{{.Code}}</div>{{if .Expiry}}<p style="margin:10px 0 0;font-size:12px;line-height:20px;color:#52636b">Expires {{.Expiry}}</p>{{end}}</div>{{end}}
{{if and .Expiry (not .Code)}}<p style="margin:16px 0;font-size:12px;line-height:20px;color:#52636b">Expires {{.Expiry}}</p>{{end}}
{{if .Action.URL}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:20px 0 0"><tr><td bgcolor="#0f766e" style="border-radius:6px;text-align:center"><a href="{{.Action.URL}}" style="display:inline-block;padding:12px 20px;font-family:'Manrope',Arial,Helvetica,sans-serif;color:#ffffff;text-decoration:none;font-size:14px;line-height:22px;font-weight:600">{{.Action.Label}}</a></td></tr></table>{{end}}
</td></tr></table>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px"><tr><td style="padding:18px 12px 0;text-align:center;font-family:'Manrope',Arial,Helvetica,sans-serif;font-size:11px;line-height:19px;color:#6b777b">{{.Safety}}{{if .SupportEmail}}<br><a href="mailto:{{.SupportEmail}}" style="color:#52636b;text-decoration:underline">Contact support</a>{{end}}</td></tr></table>
</td></tr></table></body></html>`))
