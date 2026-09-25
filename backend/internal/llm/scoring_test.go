package llm

import "testing"

func TestParseScoreResultWrappedJSON(t *testing.T) {
	raw := "```json\n{\"conveyed\":[\"A\"],\"missed\":[\"B\"],\"tone\":\"neutral\",\"escalation_done\":[],\"escalation_ok\":false,\"reasoning\":\"ok\"}\n```"
	result, err := ParseScoreResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conveyed) != 1 || result.Conveyed[0] != "A" || result.Tone != "neutral" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := ParseScoreResult("not json"); err == nil {
		t.Fatal("malformed response accepted")
	}
}
