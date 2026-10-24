package observer

import (
	"testing"

	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

func TestBuildWorldAndChronicle(t *testing.T) {
	events := []model.Event{
		{ID: "10000000-0000-4000-8000-000000000001", Sequence: 1, Type: "activity.scheduled", SimulatedAt: 2, Actor: "lyra", Visibility: "shared", Activity: &model.Activity{ID: "a1", Actor: "lyra", Name: "weave", Location: "Workshop", StartAt: 2, EndAt: 4}, Present: map[string]bool{"simulatedAt": true}},
		{ID: "10000000-0000-4000-8000-000000000002", Sequence: 2, Type: "memory.recorded", SimulatedAt: 3, Actor: "lyra", Owner: "lyra", Content: "secret", Visibility: "private", Present: map[string]bool{"simulatedAt": true}},
	}
	view := Build(events, "test", "dev", false)
	if view.VisibleEvents != 1 || view.PrivateEvents != 1 || len(view.Actors) != 1 {
		t.Fatalf("unexpected counts: %#v", view)
	}
	if view.Actors[0].Location != "Workshop" || view.Actors[0].Activity != "weave" {
		t.Fatalf("unexpected actor: %#v", view.Actors[0])
	}
	if len(view.Chronicle) != 1 || view.Chronicle[0].Text != "lyra began weave at Workshop." {
		t.Fatalf("unexpected chronicle: %#v", view.Chronicle)
	}
}

func TestBuildCanIncludePrivateEvents(t *testing.T) {
	events := []model.Event{{ID: "10000000-0000-4000-8000-000000000001", Sequence: 1, Type: "memory.recorded", SimulatedAt: 1, Actor: "maia", Owner: "maia", Content: "secret", Visibility: "private", Present: map[string]bool{"simulatedAt": true}}}
	if view := Build(events, "test", "dev", true); view.VisibleEvents != 1 || len(view.Chronicle) != 1 {
		t.Fatalf("private event was not included: %#v", view)
	}
}

func TestCompletedMovementChangesLocation(t *testing.T) {
	events := []model.Event{
		{ID: "10000000-0000-4000-8000-000000000001", Sequence: 1, Type: "activity.scheduled", SimulatedAt: 0, Actor: "athena", Visibility: "shared", Activity: &model.Activity{ID: "move-1", Actor: "athena", Name: "go to Archive", Kind: "movement", Location: "Agora", Destination: "Archive", StartAt: 0, EndAt: 1}, Present: map[string]bool{"simulatedAt": true}},
		{ID: "10000000-0000-4000-8000-000000000002", Sequence: 2, Type: "activity.completed", SimulatedAt: 1, Actor: "athena", ActivityID: "move-1", Visibility: "shared", Present: map[string]bool{"simulatedAt": true}},
	}
	view := Build(events, "test", "dev", false)
	if view.Actors[0].Location != "Archive" {
		t.Fatalf("movement did not update location: %#v", view.Actors[0])
	}
	if view.Chronicle[1].Text != "athena arrived at Archive." {
		t.Fatalf("unexpected movement prose: %#v", view.Chronicle)
	}
}

func TestMovementUsesPreviouslyObservedOrigin(t *testing.T) {
	events := []model.Event{
		{ID: "10000000-0000-4000-8000-000000000001", Sequence: 1, Type: "activity.scheduled", SimulatedAt: 0, Actor: "pallas", Visibility: "shared", Activity: &model.Activity{ID: "work-1", Actor: "pallas", Name: "tend garden", Location: "South Garden", StartAt: 0, EndAt: 1}, Present: map[string]bool{"simulatedAt": true}},
		{ID: "10000000-0000-4000-8000-000000000002", Sequence: 2, Type: "activity.completed", SimulatedAt: 1, Actor: "pallas", ActivityID: "work-1", Visibility: "shared", Present: map[string]bool{"simulatedAt": true}},
		{ID: "10000000-0000-4000-8000-000000000003", Sequence: 3, Type: "activity.scheduled", SimulatedAt: 1, Actor: "pallas", Visibility: "shared", Activity: &model.Activity{ID: "move-1", Actor: "pallas", Name: "walk to Agora", Kind: "movement", Location: "Agora", Destination: "Agora", StartAt: 1, EndAt: 2}, Present: map[string]bool{"simulatedAt": true}},
	}
	view := Build(events, "test", "dev", false)
	if view.Chronicle[2].Text != "pallas set out from South Garden toward Agora." {
		t.Fatalf("unexpected movement origin: %#v", view.Chronicle[2])
	}
}
