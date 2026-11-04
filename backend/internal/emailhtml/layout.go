// Package emailhtml provides a provider-independent, escaped email presentation.
package emailhtml

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"
	"time"
)

type Content struct {
	Brand, Title, Body, Code, Expiry, Origin, Key string
}

type Action struct{ Label, URL string }

// QuickAction never accepts a user-supplied path or includes secrets in URLs.
func QuickAction(origin, key string) Action {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
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
	case "tenant.invitation", "platform.invitation.issued", "platform.invitation.reminder":
		path, label = "/login", "Open Bidora"
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
	data := struct {
		Content
		Lines  []string
		Action Action
	}{c, strings.Split(c.Body, "\n"), QuickAction(c.Origin, c.Key)}
	var out bytes.Buffer
	if err := layout.Execute(&out, data); err != nil {
		return ""
	}
	return out.String()
}

var layout = template.Must(template.New("notification").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:#f4f6f8;color:#18232d;font-family:Arial,Helvetica,sans-serif;-webkit-text-size-adjust:100%">
<div style="display:none;max-height:0;overflow:hidden;opacity:0">{{.Title}} · {{.Brand}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f6f8"><tr><td align="center" style="padding:28px 12px">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border:1px solid #e2e7eb;border-radius:16px">
<tr><td style="padding:28px 24px 20px;border-bottom:1px solid #edf0f2"><span style="font-size:23px;line-height:30px;font-weight:700;letter-spacing:-.5px;color:#18232d">{{.Brand}}</span><div style="margin-top:5px;color:#65727e;font-size:12px">Account notifications</div></td></tr>
<tr><td style="padding:26px 24px"><h1 style="margin:0 0 18px;font-size:24px;line-height:32px;letter-spacing:-.4px">{{.Title}}</h1>
{{range .Lines}}{{if .}}<p style="margin:0 0 14px;font-size:15px;line-height:24px;overflow-wrap:anywhere;word-break:break-word">{{.}}</p>{{end}}{{end}}
{{if .Code}}<div style="margin:22px 0;padding:20px 12px;background:#f4f7f7;border:1px solid #dce6e5;border-radius:10px;text-align:center"><div style="margin-bottom:10px;font-size:12px;color:#536660">Your code</div><div style="font-family:Consolas,monospace;font-size:{{if le (len .Code) 6}}28{{else}}15{{end}}px;line-height:38px;font-weight:700;letter-spacing:{{if le (len .Code) 6}}5{{else}}1{{end}}px;word-break:break-all;color:#18232d">{{.Code}}</div>{{if .Expiry}}<div style="margin-top:12px;font-size:12px;line-height:20px;color:#536660">Expires {{.Expiry}}</div>{{end}}</div>{{end}}
{{if .Action.URL}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px 0"><tr><td bgcolor="#185c50" style="border-radius:8px;text-align:center"><a href="{{.Action.URL}}" style="display:inline-block;padding:14px 22px;color:#ffffff;text-decoration:none;font-size:15px;line-height:22px;font-weight:700">{{.Action.Label}}</a></td></tr></table><p style="font-size:12px;line-height:20px;color:#65727e;word-break:break-all">Button not working? Open<br><a href="{{.Action.URL}}" style="color:#185c50;text-decoration:underline">{{.Action.URL}}</a></p>{{end}}
</td></tr><tr><td style="padding:20px 24px;background:#fafbfc;border-top:1px solid #edf0f2;border-radius:0 0 16px 16px"><p style="margin:0;font-size:12px;line-height:20px;color:#65727e">Never share your password or verification codes. If you did not request this, ignore this message or contact your established support channel.</p></td></tr></table>
<p style="margin:18px 0 0;font-size:12px;color:#7a8791">{{.Brand}} · Secure account notification</p></td></tr></table></body></html>`))
