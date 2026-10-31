// Package emailtemplate renders provider-neutral, multipart-ready Bidora email
// bodies. Templates use no remote assets, tracking pixels, scripts or unsafe HTML.
package emailtemplate

import (
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"bidora/backend/internal/platformadmin/domain"
)

type PlatformInvitationKind string

const (
	PlatformInvitationIssued   PlatformInvitationKind = "ISSUED"
	PlatformInvitationReminder PlatformInvitationKind = "REMINDER"
)

type PlatformInvitationInput struct {
	Kind      PlatformInvitationKind
	Role      domain.Role
	Token     string
	ExpiresAt time.Time
}

type Message struct{ Subject, Text, HTML string }

func (Message) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED email message]") }

func RenderPlatformInvitation(input PlatformInvitationInput) (Message, error) {
	if input.Kind != PlatformInvitationIssued && input.Kind != PlatformInvitationReminder {
		return Message{}, errors.New("platform_invitation_template_invalid")
	}
	if _, err := domain.Capabilities(input.Role); err != nil || input.ExpiresAt.IsZero() || strings.ContainsAny(input.Token, "\r\n<>") {
		return Message{}, errors.New("platform_invitation_template_invalid")
	}
	if input.Kind == PlatformInvitationIssued && len(input.Token) != 43 || input.Kind == PlatformInvitationReminder && input.Token != "" {
		return Message{}, errors.New("platform_invitation_template_invalid")
	}
	roleLabel := roleLabel(input.Role)
	prefix := "You’re invited"
	intro := "A Bidora Platform Owner invited you to help administer Bidora."
	if input.Kind == PlatformInvitationReminder {
		prefix = "Reminder: your invitation is waiting"
		intro = "Your invitation to help administer Bidora has not yet been accepted."
	}
	expires := input.ExpiresAt.UTC().Format("2 Jan 2006, 15:04 UTC")
	details := "Role: " + roleLabel + "\nExpires: " + expires
	instruction := "Sign in using the exact invited email address, confirm TOTP, then enter this code."
	if input.Token != "" {
		details = "Role: " + roleLabel + "\nInvitation code: " + input.Token + "\nExpires: " + expires
	} else {
		instruction = "Use the code in your original invitation email. If it is unavailable, ask the Platform Owner to resend the invitation; a resend invalidates the previous code."
	}
	text := prefix + "\n\n" + intro + "\n\n" + details + "\n\n" + instruction + " Bidora support will never ask you to send an invitation code by email or chat.\n\nIf you did not expect this invitation, you can ignore this message.\n"
	data := struct{ Prefix, Intro, Role, Token, Expires string }{prefix, intro, roleLabel, input.Token, expires}
	var html strings.Builder
	if err := platformInvitationHTML.Execute(&html, data); err != nil {
		return Message{}, errors.New("platform_invitation_template_unavailable")
	}
	subject := "Your Bidora Platform Administrator invitation"
	if input.Kind == PlatformInvitationReminder {
		subject = "Reminder: Bidora Platform Administrator invitation"
	}
	return Message{Subject: subject, Text: text, HTML: html.String()}, nil
}

func roleLabel(role domain.Role) string {
	switch role {
	case domain.PlatformOwner:
		return "Platform Owner"
	case domain.OperationsAdministrator:
		return "Operations Administrator"
	case domain.BillingAdministrator:
		return "Billing Administrator"
	case domain.IntegrationAdministrator:
		return "Integration Administrator"
	case domain.SupportAdministrator:
		return "Support Administrator"
	case domain.SecurityAuditAdministrator:
		return "Security and Audit Administrator"
	default:
		return "Platform Administrator"
	}
}

var platformInvitationHTML = template.Must(template.New("platform-invitation").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Prefix}}</title></head>
<body style="margin:0;background:#f5f7fa;color:#172033;font-family:Arial,sans-serif">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f5f7fa;padding:32px 12px"><tr><td align="center">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#ffffff;border:1px solid #e3e8ef;border-radius:12px">
<tr><td style="padding:28px 32px 12px"><div style="font-size:20px;font-weight:700;color:#111827">Bidora</div></td></tr>
<tr><td style="padding:8px 32px 32px"><h1 style="margin:0 0 16px;font-size:24px;line-height:1.3;color:#111827">{{.Prefix}}</h1>
<p style="margin:0 0 20px;font-size:15px;line-height:1.6">{{.Intro}}</p>
<div style="margin:0 0 20px;padding:16px;background:#f8fafc;border:1px solid #e3e8ef;border-radius:8px">
<div style="font-size:12px;color:#64748b;text-transform:uppercase;letter-spacing:.05em">Role</div><div style="margin-top:4px;font-size:16px;font-weight:600">{{.Role}}</div>
{{if .Token}}<div style="margin-top:16px;font-size:12px;color:#64748b;text-transform:uppercase;letter-spacing:.05em">Invitation code</div><div style="margin-top:6px;font:600 15px/1.5 monospace;word-break:break-all">{{.Token}}</div>{{end}}</div>
<p style="margin:0 0 16px;font-size:14px;line-height:1.6"><strong>Expires:</strong> {{.Expires}}</p>
{{if .Token}}<p style="margin:0 0 16px;font-size:14px;line-height:1.6">Sign in using the exact invited email address, confirm TOTP, then enter the invitation code.</p>{{else}}<p style="margin:0 0 16px;font-size:14px;line-height:1.6">Use the code in your original invitation email. If it is unavailable, ask the Platform Owner to resend the invitation; a resend invalidates the previous code.</p>{{end}}
<p style="margin:0;font-size:13px;line-height:1.6;color:#64748b">Bidora support will never ask you to send this code by email or chat. If you did not expect this invitation, ignore this message.</p></td></tr>
</table></td></tr></table></body></html>`))
