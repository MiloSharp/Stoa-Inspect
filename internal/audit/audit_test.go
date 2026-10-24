package audit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MiloSharp/Stoa-Inspect/internal/eventio"
	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

func parse(t testing.TB, text string) []model.Event {
	t.Helper()
	events, err := eventio.Read(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestValidStreamPasses(t *testing.T) {
	input := `{"id":"10000000-0000-4000-8000-000000000001","sequence":1,"type":"activity.scheduled","simulatedAt":0,"actor":"athena","visibility":"shared","activity":{"id":"20000000-0000-4000-8000-000000000001","actor":"athena","name":"study","startAt":0,"endAt":2,"reads":["archive"],"writes":["notes"]}}
{"id":"10000000-0000-4000-8000-000000000002","sequence":2,"type":"activity.completed","simulatedAt":2,"actor":"athena","visibility":"shared","activityId":"20000000-0000-4000-8000-000000000001"}`
	r := Run(parse(t, input), "test", "dev", Options{})
	if r.Summary.Status != "pass" || r.Summary.Errors != 0 {
		t.Fatalf("unexpected report: %#v", r)
	}
	if len(r.Actors) != 1 || r.Actors[0].Completed != 1 {
		t.Fatalf("unexpected actor summary: %#v", r.Actors)
	}
}

func TestDetectsGapConflictAndUnknownActivity(t *testing.T) {
	input := `{"id":"10000000-0000-4000-8000-000000000001","sequence":1,"type":"activity.scheduled","simulatedAt":0,"actor":"athena","visibility":"shared","activity":{"id":"20000000-0000-4000-8000-000000000001","actor":"athena","name":"study","startAt":0,"endAt":2,"writes":["archive"]}}
{"id":"10000000-0000-4000-8000-000000000003","sequence":3,"type":"activity.scheduled","simulatedAt":0,"actor":"pallas","visibility":"shared","activity":{"id":"20000000-0000-4000-8000-000000000002","actor":"pallas","name":"catalogue","startAt":0,"endAt":2,"reads":["archive"]}}
{"id":"10000000-0000-4000-8000-000000000004","sequence":4,"type":"activity.completed","simulatedAt":1,"actor":"minerva","visibility":"shared","activityId":"missing"}`
	r := Run(parse(t, input), "test", "dev", Options{})
	for _, code := range []string{"sequence.gap", "activity.resource_conflict", "activity.unknown"} {
		if !hasCode(r, code) {
			t.Errorf("missing %s: %#v", code, r.Findings)
		}
	}
}

func TestProposalAndPauseLifecycle(t *testing.T) {
	input := `{"id":"10000000-0000-4000-8000-000000000001","sequence":1,"type":"simulation.resume","simulatedAt":0,"actor":"system","visibility":"shared"}
{"id":"10000000-0000-4000-8000-000000000002","sequence":2,"type":"proposal.resolved","simulatedAt":0,"actor":"athena","visibility":"shared","proposalId":"missing","resolution":"approved"}`
	r := Run(parse(t, input), "test", "dev", Options{})
	if !hasCode(r, "simulation.not_paused") || !hasCode(r, "proposal.unknown") {
		t.Fatalf("unexpected findings: %#v", r.Findings)
	}
}

func TestSequenceRange(t *testing.T) {
	input := `{"id":"10000000-0000-4000-8000-000000000001","sequence":1,"type":"world.set","simulatedAt":0,"actor":"system","visibility":"shared","target":"a"}
{"id":"10000000-0000-4000-8000-000000000002","sequence":2,"type":"world.set","simulatedAt":1,"actor":"system","visibility":"shared","target":"b"}`
	r := Run(parse(t, input), "test", "dev", Options{FromSequence: 2, ToSequence: 2})
	if r.Summary.Events != 1 || r.Summary.FirstSequence != 2 {
		t.Fatalf("range not applied: %#v", r.Summary)
	}
}

func hasCode(r model.Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func BenchmarkAuditTenThousand(b *testing.B) {
	var text strings.Builder
	for i := 1; i <= 10000; i++ {
		fmt.Fprintf(&text, "{\"id\":\"%08x-0000-4000-8000-%012x\",\"sequence\":%d,\"type\":\"world.set\",\"simulatedAt\":%d,\"actor\":\"system\",\"visibility\":\"shared\",\"target\":\"value-%d\"}\n", i, i, i, i, i)
	}
	events := parse(b, text.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run(events, "benchmark", "dev", Options{})
	}
}
