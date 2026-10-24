package eventio

import (
	"strings"
	"testing"
)

const first = `{"id":"10000000-0000-4000-8000-000000000001","sequence":1,"type":"world.set","simulatedAt":0,"actor":"system","visibility":"shared","target":"ready"}`

func TestReadFormats(t *testing.T) {
	tests := map[string]string{"array": "[" + first + "]", "state": "{\"version\":1,\"events\":[" + first + "]}", "ndjson": "# export\n" + first + "\n"}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			events, err := Read(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 || events[0].Sequence != 1 || events[0].Type != "world.set" {
				t.Fatalf("unexpected events: %#v", events)
			}
		})
	}
}

func TestReadPostgresRow(t *testing.T) {
	input := `[{"event_id":"10000000-0000-4000-8000-000000000001","sequence":1,"event_type":"world.set","simulated_at":0,"actor":"system","visibility":"shared","payload":{"target":"ready"}}]`
	events, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if events[0].ID == "" || events[0].SimulatedAt != 0 || events[0].Target != "ready" || !events[0].Present["simulatedAt"] {
		t.Fatalf("aliases not normalized: %#v", events[0])
	}
}

func TestReadRejectsEmpty(t *testing.T) {
	if _, err := Read(strings.NewReader(" \n")); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadDocumentIncludesWorldTopology(t *testing.T) {
	input := `{"locations":{"Agora":{"name":"Agora","description":"Shared space","exits":{"Archive":{"duration":2}}},"Archive":{"name":"Archive","exits":{}}},"events":[` + first + `]}`
	document, err := ReadDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Events) != 1 || document.Locations["Agora"].Description != "Shared space" {
		t.Fatalf("unexpected document: %#v", document)
	}
	if document.Locations["Agora"].Exits["Archive"].Duration != 2 {
		t.Fatalf("topology not decoded: %#v", document.Locations)
	}
}
