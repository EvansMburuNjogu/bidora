package httpapi

import (
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"net/http/httptest"
	"testing"
)

func TestPlatformSingleRecordAndFinalPageAlwaysIncludeNullableCursor(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "fixture")
		return platformJSON(c, 200, map[string]any{"administrators": []any{}}, nil)
	})
	r, e := app.Test(httptest.NewRequest("GET", "/test", nil))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var root struct{ Meta map[string]any }
	if e = json.NewDecoder(r.Body).Decode(&root); e != nil {
		t.Fatal(e)
	}
	if v, ok := root.Meta["next_cursor"]; !ok || v != nil {
		t.Fatal("terminal cursor omitted: clients would reject valid users and invitation receipts")
	}
}
