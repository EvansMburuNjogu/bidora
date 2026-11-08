package emailhtml

import (
	"net/url"
	"strings"
	"testing"
)

func TestInvitationLinkUsesFragmentAndNeverDisplaysTheBearerAsACode(t *testing.T) {
	token := strings.Repeat("A", 43)
	action := InvitationAction("https://sandbox.example.test", token)
	u, e := url.Parse(action.URL)
	if e != nil || u.Path != "/administrator-invitation" || u.RawQuery != "" || u.Fragment != "code="+token {
		t.Fatal("invitation URL leaks selector into request URL or has wrong destination")
	}
	html := Render(Content{Origin: "https://sandbox.example.test", Key: "platform.invitation.issued", Code: token, Title: "Your invitation", Body: "Set your password and verify your email."})
	if !strings.Contains(html, action.URL) || strings.Contains(html, "word-break:break-all;color:#173b3d") {
		t.Fatal("setup link missing or bearer rendered as visible verification code")
	}
	if InvitationAction("https://sandbox.example.test/unsafe", token).URL != "" || InvitationAction("http://sandbox.example.test", token).URL != "" || InvitationAction("https://sandbox.example.test", "bad").URL != "" {
		t.Fatal("invalid origin or token created action")
	}
}
