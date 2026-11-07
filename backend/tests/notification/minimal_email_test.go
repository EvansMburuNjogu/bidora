package notification_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bidora/backend/internal/emailhtml"
	app "bidora/backend/internal/notification/application"
)

func TestMinimalEmailUsesAppFontFirstPartyAssetAndOneClearAction(t *testing.T) {
	html := emailhtml.Render(emailhtml.Content{Brand: "Bidora", Title: "Verify your sign-in", Body: "Enter this code to sign in.", Code: "123456", Expiry: "2030-01-01T12:00:00Z", Origin: "https://sandbox.bidora.africa", Key: "identity.login_code", SupportEmail: "support@example.test"})
	for _, want := range []string{"font-family:'Manrope',Arial,Helvetica,sans-serif", "https://sandbox.bidora.africa/fonts/manrope-latin.woff2", "Contact support", "123456"} {
		if !strings.Contains(html, want) {
			t.Fatal("missing minimal email element", want)
		}
	}
	for _, forbidden := range []string{"ZgotmplZ", "fonts.googleapis.com", "fonts.gstatic.com", "Consolas", "monospace", "Account notifications", "Button not working?", "Secure account notification"} {
		if strings.Contains(html, forbidden) {
			t.Fatal("unwanted clutter or unsafe font source", forbidden)
		}
	}
	if strings.Count(html, ">Continue sign-in</a>") != 1 || strings.Count(html, "123456") != 1 || strings.Count(html, "Expires ") != 1 {
		t.Fatal("code, expiry or action duplicated")
	}
	root := filepath.Join("..", "..", "..", "web")
	source, err := os.ReadFile(filepath.Join(root, "src", "app", "layout.tsx"))
	if err != nil || !strings.Contains(string(source), "Manrope(") {
		t.Fatal("email font no longer matches the app", err)
	}
	font, err := os.ReadFile(filepath.Join(root, "public", "fonts", "manrope-latin.woff2"))
	if err != nil || len(font) < 1000 || string(font[:4]) != "wOF2" {
		t.Fatal("first-party Manrope asset missing or invalid", err)
	}
	license, err := os.ReadFile(filepath.Join(root, "public", "fonts", "Manrope-OFL.txt"))
	if err != nil || !strings.Contains(string(license), "SIL OPEN FONT LICENSE Version 1.1") {
		t.Fatal("redistributed font license missing", err)
	}
}
func TestSeedEmailIsConciseWithoutOverwritingAdministratorCopy(t *testing.T) {
	template := app.Template{Key: "identity.login_code", Name: "Login", Subject: "Your Bidora sign-in code", Body: "Your sign-in code is {{code}}. Expires: {{expires_at}}.", Enabled: true, Version: 1}
	values := map[string]string{"platform_name": "Bidora", "code": "123456", "expires_at": "2030-01-01T12:00:00Z", "web_origin": "https://sandbox.bidora.africa"}
	rendered, err := app.RenderTemplate(template, values)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.HTML, "Enter this code to sign in.") || strings.Count(rendered.HTML, "Expires ") != 1 || strings.Contains(rendered.HTML, "the code below") || !strings.Contains(rendered.Text, "123456") {
		t.Fatal("seed copy is duplicated or code missing")
	}
	template.Body = "Our custom instructions. Code: {{code}}. Valid until {{expires_at}}."
	rendered, err = app.RenderTemplate(template, values)
	if err != nil || !strings.Contains(rendered.HTML, "Our custom instructions.") || !strings.Contains(rendered.Text, "Our custom instructions.") {
		t.Fatal("administrator copy was replaced", err)
	}
}
