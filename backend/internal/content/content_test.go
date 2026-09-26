package content

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmbeddedCatalog(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.Scenarios); got != 50 {
		t.Fatalf("scenario count = %d, want 50", got)
	}
	if got := len(c.Passengers); got != 44 {
		t.Fatalf("passenger count = %d, want 44", got)
	}
	for _, p := range c.Passengers {
		if p.Language != "ru" && p.Language != "en" {
			t.Fatalf("passenger %s has unsupported language %q", p.ID, p.Language)
		}
	}
}

func TestCatalogRejectsInvalidData(t *testing.T) {
	original, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*Catalog)
		want   string
	}{
		{"duplicate scenario", func(c *Catalog) { c.Scenarios[1].ID = c.Scenarios[0].ID }, "duplicate id"},
		{"bad type", func(c *Catalog) { c.Scenarios[0].Type = "unknown" }, "invalid type"},
		{"bad criticality", func(c *Catalog) { c.Scenarios[0].Criticality = "unknown" }, "invalid criticality"},
		{"missing point", func(c *Catalog) { c.Scenarios[0].CorrectCompletion.MustConvey = nil }, "must_convey"},
		{"duplicate point", func(c *Catalog) { c.Scenarios[0].CorrectCompletion.MustConvey[1].ID = "A" }, "duplicate must_convey"},
		{"bad target", func(c *Catalog) { c.Scenarios[0].CorrectCompletion.Escalation.To[0] = "station" }, "escalation target"},
		{"missing target", func(c *Catalog) { c.Scenarios[0].CorrectCompletion.Escalation.To = nil }, "needs a target"},
		{"bad time", func(c *Catalog) { c.Scenarios[0].TimeLimitSec = 0 }, "positive time_limit_sec"},
		{"duplicate passenger", func(c *Catalog) { c.Passengers[1].ID = c.Passengers[0].ID }, "duplicate id"},
		{"bad language", func(c *Catalog) { c.Passengers[0].Language = "xx" }, "invalid age, tone or language"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Copy through JSON so nested slices do not mutate the shared fixture.
			raw, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var c Catalog
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&c)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestParseRejectsMalformedJSON(t *testing.T) {
	_, err := Parse([]byte("{"), []byte("[]"))
	if err == nil || !strings.Contains(err.Error(), "scenarios.json") {
		t.Fatalf("Parse() = %v, want scenarios.json error", err)
	}
}
