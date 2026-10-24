package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

func sample() model.Report {
	return model.Report{SchemaVersion: "1.0", ToolVersion: "dev", Source: "events.ndjson", Summary: model.Summary{Status: "fail", Events: 2, Errors: 1, Warnings: 1, FirstSequence: 1, LastSequence: 2, EventTypes: map[string]int{"world.set": 2}}, Findings: []model.Finding{{Severity: model.SeverityError, Code: "sequence.gap", Message: "missing sequence", Sequence: 2}, {Severity: model.SeverityWarning, Code: "actor.note", Message: "note", Sequence: 2, Actor: "athena"}}, Actors: []model.ActorSummary{{Actor: "athena", Events: 1}}}
}
func TestHumanReadable(t *testing.T) {
	var out bytes.Buffer
	if err := Human(&out, sample(), Options{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"STOA EVENT AUDIT", "Result:", "FAIL", "sequence.gap", "ACTORS"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out.String())
		}
	}
}
func TestJSONAndFiltering(t *testing.T) {
	var out bytes.Buffer
	if err := JSON(&out, sample(), Options{MinimumSeverity: model.SeverityError, Actor: "pallas"}); err != nil {
		t.Fatal(err)
	}
	var got model.Report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 1 || got.Findings[0].Code != "sequence.gap" || len(got.Actors) != 0 {
		t.Fatalf("unexpected filtered report: %#v", got)
	}
}
