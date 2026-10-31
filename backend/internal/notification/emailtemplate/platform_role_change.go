package emailtemplate

import (
	"errors"
	"html/template"
	"strings"
	"time"

	"bidora/backend/internal/platformadmin/domain"
)

type PlatformRoleChangeInput struct {
	Role       domain.Role
	Assigned   bool
	OccurredAt time.Time
}

func RenderPlatformRoleChange(input PlatformRoleChangeInput) (Message, error) {
	if _, err := domain.Capabilities(input.Role); err != nil || input.OccurredAt.IsZero() {
		return Message{}, errors.New("platform_role_change_template_invalid")
	}
	action, past := "Role assigned", "was assigned to"
	if !input.Assigned {
		action, past = "Role removed", "was removed from"
	}
	role := roleLabel(input.Role)
	occurred := input.OccurredAt.UTC().Format("2 Jan 2006, 15:04 UTC")
	text := action + "\n\nThe " + role + " role " + past + " your Bidora Platform Administrator account.\n\nTime: " + occurred + "\n\nFor your security, existing sessions were revoked. Sign in again and complete TOTP. If you did not expect this change, contact an authorized Platform Owner through your established support channel.\n"
	var html strings.Builder
	if err := platformRoleChangeHTML.Execute(&html, struct{ Action, Past, Role, Occurred string }{action, past, role, occurred}); err != nil {
		return Message{}, errors.New("platform_role_change_template_unavailable")
	}
	return Message{Subject: "Bidora Platform Administrator role changed", Text: text, HTML: html.String()}, nil
}

var platformRoleChangeHTML = template.Must(template.New("platform-role-change").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Action}}</title></head><body style="margin:0;background:#f5f7fa;color:#172033;font-family:Arial,sans-serif"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="padding:32px 12px"><tr><td align="center"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#fff;border:1px solid #e3e8ef;border-radius:12px"><tr><td style="padding:28px 32px 12px;font-size:20px;font-weight:700;color:#111827">Bidora</td></tr><tr><td style="padding:8px 32px 32px"><h1 style="margin:0 0 16px;font-size:24px;color:#111827">{{.Action}}</h1><p style="font-size:15px;line-height:1.6">The <strong>{{.Role}}</strong> role {{.Past}} your Platform Administrator account.</p><p style="font-size:14px;line-height:1.6"><strong>Time:</strong> {{.Occurred}}</p><p style="font-size:14px;line-height:1.6">For your security, existing sessions were revoked. Sign in again and complete TOTP.</p><p style="margin:0;font-size:13px;line-height:1.6;color:#64748b">If you did not expect this change, contact an authorized Platform Owner through your established support channel.</p></td></tr></table></td></tr></table></body></html>`))
