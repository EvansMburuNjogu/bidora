package emailtemplate

import (
	"errors"
	"html/template"
	"strings"
	"time"
)

type PlatformIdentityStateInput struct {
	Disabled   bool
	OccurredAt time.Time
}

func RenderPlatformIdentityState(input PlatformIdentityStateInput) (Message, error) {
	if input.OccurredAt.IsZero() {
		return Message{}, errors.New("platform_identity_state_template_invalid")
	}
	action, detail := "Administrator access restored", "Your Bidora Platform Administrator account was reactivated. Sign in again and complete TOTP."
	if input.Disabled {
		action, detail = "Administrator access disabled", "Your Bidora Platform Administrator account was disabled and its existing sessions were revoked."
	}
	occurred := input.OccurredAt.UTC().Format("2 Jan 2006, 15:04 UTC")
	text := action + "\n\n" + detail + "\n\nTime: " + occurred + "\n\nIf you did not expect this change, contact an authorized Platform Owner through your established support channel.\n"
	var html strings.Builder
	if err := platformIdentityStateHTML.Execute(&html, struct{ Action, Detail, Occurred string }{action, detail, occurred}); err != nil {
		return Message{}, errors.New("platform_identity_state_template_unavailable")
	}
	return Message{Subject: "Bidora Platform Administrator access changed", Text: text, HTML: html.String()}, nil
}

var platformIdentityStateHTML = template.Must(template.New("platform-identity-state").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Action}}</title></head><body style="margin:0;background:#f5f7fa;color:#172033;font-family:Arial,sans-serif"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="padding:32px 12px"><tr><td align="center"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#fff;border:1px solid #e3e8ef;border-radius:12px"><tr><td style="padding:28px 32px 12px;font-size:20px;font-weight:700;color:#111827">Bidora</td></tr><tr><td style="padding:8px 32px 32px"><h1 style="margin:0 0 16px;font-size:24px;color:#111827">{{.Action}}</h1><p style="font-size:15px;line-height:1.6">{{.Detail}}</p><p style="font-size:14px;line-height:1.6"><strong>Time:</strong> {{.Occurred}}</p><p style="margin:0;font-size:13px;line-height:1.6;color:#64748b">If you did not expect this change, contact an authorized Platform Owner through your established support channel.</p></td></tr></table></td></tr></table></body></html>`))
