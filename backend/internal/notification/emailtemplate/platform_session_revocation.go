package emailtemplate

import (
	"errors"
	"html/template"
	"strings"
	"time"
)

type PlatformSessionRevocationInput struct{ OccurredAt time.Time }

func RenderPlatformSessionRevocation(input PlatformSessionRevocationInput) (Message, error) {
	if input.OccurredAt.IsZero() {
		return Message{}, errors.New("platform_session_revocation_template_invalid")
	}
	occurred := input.OccurredAt.UTC().Format("2 Jan 2006, 15:04 UTC")
	text := "Administrator sessions revoked\n\nAll existing sessions for your Bidora Platform Administrator account were revoked. Sign in again and complete email verification.\n\nTime: " + occurred + "\n\nIf you did not expect this action, contact an authorized Platform Owner through your established support channel.\n"
	var html strings.Builder
	if err := platformSessionRevocationHTML.Execute(&html, occurred); err != nil {
		return Message{}, errors.New("platform_session_revocation_template_unavailable")
	}
	return Message{Subject: "Bidora Platform Administrator sessions revoked", Text: text, HTML: html.String()}, nil
}

var platformSessionRevocationHTML = template.Must(template.New("platform-session-revocation").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Administrator sessions revoked</title></head><body style="margin:0;background:#f5f7fa;color:#172033;font-family:Arial,sans-serif"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="padding:32px 12px"><tr><td align="center"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#fff;border:1px solid #e3e8ef;border-radius:12px"><tr><td style="padding:28px 32px 12px;font-size:20px;font-weight:700;color:#111827">Bidora</td></tr><tr><td style="padding:8px 32px 32px"><h1 style="margin:0 0 16px;font-size:24px;color:#111827">Administrator sessions revoked</h1><p style="font-size:15px;line-height:1.6">All existing sessions for your Platform Administrator account were revoked. Sign in again and complete email verification.</p><p style="font-size:14px;line-height:1.6"><strong>Time:</strong> {{.}}</p><p style="margin:0;font-size:13px;line-height:1.6;color:#64748b">If you did not expect this action, contact an authorized Platform Owner through your established support channel.</p></td></tr></table></td></tr></table></body></html>`))
