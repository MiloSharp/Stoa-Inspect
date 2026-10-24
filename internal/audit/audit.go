package audit

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

type Options struct{ FromSequence, ToSequence int64 }
type activityState struct {
	activity model.Activity
	status   string
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
var knownTypes = map[string]bool{
	"scenario.loaded": true, "activity.scheduled": true, "activity.completed": true,
	"activity.interrupted": true, "activity.resumed": true, "activity.cancelled": true,
	"time.advanced": true, "simulation.pause": true, "simulation.resume": true,
	"world.set": true, "memory.recorded": true, "autonomy.decided": true,
	"proposal.created": true, "proposal.resolved": true,
}

func Run(events []model.Event, source, toolVersion string, options Options) model.Report {
	selected := make([]model.Event, 0, len(events))
	for _, event := range events {
		if options.FromSequence > 0 && event.Sequence < options.FromSequence {
			continue
		}
		if options.ToSequence > 0 && event.Sequence > options.ToSequence {
			continue
		}
		selected = append(selected, event)
	}
	report := model.Report{SchemaVersion: "1.0", ToolVersion: toolVersion, Source: source,
		Findings: []model.Finding{}, Actors: []model.ActorSummary{}, Summary: model.Summary{Events: len(selected), EventTypes: map[string]int{}}}
	if len(selected) == 0 {
		add(&report, model.SeverityError, "input.no_events", "no events remain after applying the sequence range", model.Event{})
		finalize(&report, nil)
		return report
	}
	for _, event := range selected {
		report.Summary.EventTypes[event.Type]++
	}
	sorted := append([]model.Event(nil), selected...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Sequence < sorted[j].Sequence })
	report.Summary.FirstSequence, report.Summary.LastSequence = sorted[0].Sequence, sorted[len(sorted)-1].Sequence
	validateInputOrder(&report, selected)
	validateIdentityAndSequence(&report, sorted, options.FromSequence > 0)
	actors, activities := validateLifecycles(&report, sorted)
	finalizeActorActivityCounts(actors, activities)
	finalize(&report, actors)
	return report
}

func validateInputOrder(report *model.Report, events []model.Event) {
	for i := 1; i < len(events); i++ {
		if events[i].Sequence < events[i-1].Sequence {
			add(report, model.SeverityError, "sequence.out_of_order", fmt.Sprintf("sequence %d appears after sequence %d in the input", events[i].Sequence, events[i-1].Sequence), events[i])
		}
	}
}

func validateIdentityAndSequence(report *model.Report, events []model.Event, partial bool) {
	sequences := map[int64]model.Event{}
	ids := map[string]model.Event{}
	if !partial && events[0].Sequence != 1 {
		add(report, model.SeverityWarning, "sequence.partial_start", fmt.Sprintf("stream starts at sequence %d rather than 1; this may be a partial export", events[0].Sequence), events[0])
	}
	var previous *model.Event
	for _, event := range events {
		if event.Sequence <= 0 {
			add(report, model.SeverityError, "event.sequence_missing", "event sequence must be a positive integer", event)
		}
		if earlier, exists := sequences[event.Sequence]; exists {
			add(report, model.SeverityError, "sequence.duplicate", fmt.Sprintf("sequence %d is also used by event %s", event.Sequence, displayID(earlier.ID)), event)
		} else {
			sequences[event.Sequence] = event
			if previous != nil {
				if event.Sequence > previous.Sequence+1 {
					add(report, model.SeverityError, "sequence.gap", fmt.Sprintf("missing sequences %s", sequenceRange(previous.Sequence+1, event.Sequence-1)), event)
				}
				if event.SimulatedAt < previous.SimulatedAt {
					add(report, model.SeverityError, "time.regression", fmt.Sprintf("simulated time moves backwards from %d to %d", previous.SimulatedAt, event.SimulatedAt), event)
				}
			}
			copy := event
			previous = &copy
		}
		if event.ID == "" {
			add(report, model.SeverityError, "event.id_missing", "event id is required", event)
		} else {
			if earlier, exists := ids[event.ID]; exists {
				add(report, model.SeverityError, "event.id_duplicate", fmt.Sprintf("event id is also used at sequence %d", earlier.Sequence), event)
			} else {
				ids[event.ID] = event
			}
			if !uuidPattern.MatchString(event.ID) {
				add(report, model.SeverityWarning, "event.id_not_uuid", "event id is not a canonical UUID", event)
			}
		}
		if event.Type == "" {
			add(report, model.SeverityError, "event.type_missing", "event type is required", event)
		} else if !knownTypes[event.Type] {
			add(report, model.SeverityWarning, "event.type_unknown", fmt.Sprintf("unknown event type %q", event.Type), event)
		}
		if !event.Present["simulatedAt"] {
			add(report, model.SeverityError, "event.simulated_time_missing", "simulatedAt is required", event)
		} else if event.SimulatedAt < 0 {
			add(report, model.SeverityError, "event.simulated_time_negative", "simulatedAt cannot be negative", event)
		}
		if event.Visibility == "" {
			add(report, model.SeverityError, "event.visibility_missing", "visibility is required", event)
		} else if event.Visibility != "shared" && event.Visibility != "private" {
			add(report, model.SeverityWarning, "event.visibility_unknown", fmt.Sprintf("unknown visibility %q", event.Visibility), event)
		}
	}
}

func validateLifecycles(report *model.Report, events []model.Event) (map[string]*model.ActorSummary, map[string]*activityState) {
	actors := map[string]*model.ActorSummary{}
	activities := map[string]*activityState{}
	proposals := map[string]string{}
	paused := false
	for _, event := range events {
		if event.SimulatedAt > report.Summary.FinalClock {
			report.Summary.FinalClock = event.SimulatedAt
		}
		if event.Actor != "" {
			actorSummary(actors, event.Actor).Events++
		}
		switch event.Type {
		case "activity.scheduled":
			validateScheduled(report, event, actors, activities)
		case "activity.completed":
			validateTransition(report, event, activities, "completed", []string{"active"})
			if event.Actor != "" {
				actorSummary(actors, event.Actor).Completed++
			}
		case "activity.interrupted":
			validateInterrupted(report, event, activities)
			if event.Actor != "" {
				actorSummary(actors, event.Actor).Interrupted++
			}
		case "activity.resumed":
			validateTransition(report, event, activities, "active", []string{"paused"})
		case "activity.cancelled":
			validateTransition(report, event, activities, "cancelled", []string{"paused"})
		case "proposal.created":
			if event.Proposal == nil || event.Proposal.ID == "" {
				add(report, model.SeverityError, "proposal.invalid", "proposal.created requires a proposal with an id", event)
				break
			}
			if _, ok := proposals[event.Proposal.ID]; ok {
				add(report, model.SeverityError, "proposal.duplicate", fmt.Sprintf("proposal %s was already created", event.Proposal.ID), event)
			}
			proposals[event.Proposal.ID] = "pending"
		case "proposal.resolved":
			status, ok := proposals[event.ProposalID]
			if event.ProposalID == "" || !ok {
				add(report, model.SeverityError, "proposal.unknown", fmt.Sprintf("resolution references unknown proposal %q", event.ProposalID), event)
			} else if status != "pending" {
				add(report, model.SeverityError, "proposal.already_resolved", fmt.Sprintf("proposal %s was already resolved", event.ProposalID), event)
			} else if event.Resolution != "approved" && event.Resolution != "rejected" && event.Resolution != "failed" {
				add(report, model.SeverityError, "proposal.resolution_invalid", fmt.Sprintf("invalid resolution %q", event.Resolution), event)
			} else {
				proposals[event.ProposalID] = event.Resolution
			}
		case "simulation.pause":
			if paused {
				add(report, model.SeverityWarning, "simulation.already_paused", "simulation is paused twice without a resume", event)
			}
			paused = true
		case "simulation.resume":
			if !paused {
				add(report, model.SeverityWarning, "simulation.not_paused", "simulation resumes while it is not paused", event)
			}
			paused = false
		case "time.advanced":
			if !event.Present["from"] || !event.Present["to"] || event.To < event.From {
				add(report, model.SeverityError, "time.advance_invalid", "time.advanced requires from and to with to >= from", event)
			}
		case "world.set":
			if event.Target == "" {
				add(report, model.SeverityError, "world.target_missing", "world.set requires a target", event)
			}
		case "memory.recorded":
			if event.Owner == "" || event.Content == "" {
				add(report, model.SeverityError, "memory.invalid", "memory.recorded requires owner and content", event)
			}
			if event.Visibility != "private" {
				add(report, model.SeverityWarning, "memory.not_private", "recorded memory is expected to have private visibility", event)
			}
		case "scenario.loaded":
			if !event.Present["scenario"] {
				add(report, model.SeverityError, "scenario.missing", "scenario.loaded requires a scenario payload", event)
			}
		}
	}
	return actors, activities
}

func validateScheduled(report *model.Report, event model.Event, actors map[string]*model.ActorSummary, activities map[string]*activityState) {
	if event.Activity == nil {
		add(report, model.SeverityError, "activity.missing", "activity.scheduled requires an activity payload", event)
		return
	}
	a := *event.Activity
	if a.ID == "" || a.Name == "" || event.Actor == "" {
		add(report, model.SeverityError, "activity.invalid", "scheduled activity requires id, name, and actor", event)
	}
	if a.Actor != "" && a.Actor != event.Actor {
		add(report, model.SeverityWarning, "activity.actor_mismatch", fmt.Sprintf("event actor %q differs from activity actor %q", event.Actor, a.Actor), event)
	}
	if a.Actor == "" {
		a.Actor = event.Actor
	}
	if a.EndAt <= a.StartAt {
		add(report, model.SeverityError, "activity.duration_invalid", "activity endAt must be greater than startAt", event)
	}
	if _, exists := activities[a.ID]; exists {
		add(report, model.SeverityError, "activity.duplicate", fmt.Sprintf("activity %s was already scheduled", a.ID), event)
		return
	}
	for id, existing := range activities {
		if existing.status != "active" {
			continue
		}
		if existing.activity.Actor == event.Actor {
			add(report, model.SeverityError, "activity.actor_busy", fmt.Sprintf("actor %s already has active activity %s", event.Actor, id), event)
		}
		if resources := conflictingResources(a, existing.activity); len(resources) > 0 {
			add(report, model.SeverityError, "activity.resource_conflict", fmt.Sprintf("activity %s conflicts with active activity %s on %s", a.ID, id, strings.Join(resources, ", ")), event)
		}
	}
	activities[a.ID] = &activityState{activity: a, status: "active"}
	actorSummary(actors, event.Actor).Scheduled++
}

func validateTransition(report *model.Report, event model.Event, activities map[string]*activityState, target string, allowed []string) {
	a, exists := activities[event.ActivityID]
	if event.ActivityID == "" || !exists {
		add(report, model.SeverityError, "activity.unknown", fmt.Sprintf("%s references unknown activity %q", event.Type, event.ActivityID), event)
		return
	}
	if !contains(allowed, a.status) {
		add(report, model.SeverityError, "activity.transition_invalid", fmt.Sprintf("cannot move activity %s from %s to %s", event.ActivityID, a.status, target), event)
		return
	}
	if event.Actor != "" && a.activity.Actor != "" && event.Actor != a.activity.Actor {
		add(report, model.SeverityWarning, "activity.actor_mismatch", fmt.Sprintf("event actor %q differs from scheduled actor %q", event.Actor, a.activity.Actor), event)
	}
	a.status = target
}

func validateInterrupted(report *model.Report, event model.Event, activities map[string]*activityState) {
	a, exists := activities[event.ActivityID]
	if event.ActivityID == "" || !exists {
		add(report, model.SeverityError, "activity.unknown", fmt.Sprintf("activity.interrupted references unknown activity %q", event.ActivityID), event)
		return
	}
	if a.status != "active" {
		add(report, model.SeverityError, "activity.transition_invalid", fmt.Sprintf("cannot interrupt activity %s while it is %s", event.ActivityID, a.status), event)
		return
	}
	switch event.Mode {
	case "pause":
		a.status = "paused"
	case "cancel":
		a.status = "cancelled"
	case "reschedule":
		a.status = "active"
	default:
		add(report, model.SeverityError, "activity.interrupt_mode_invalid", fmt.Sprintf("unknown interrupt mode %q", event.Mode), event)
	}
}

func conflictingResources(left, right model.Activity) []string {
	lr, lw, rr, rw := set(left.Reads), set(left.Writes), set(right.Reads), set(right.Writes)
	conflicts := map[string]bool{}
	for r := range lw {
		if rw[r] || rr[r] {
			conflicts[r] = true
		}
	}
	for r := range lr {
		if rw[r] {
			conflicts[r] = true
		}
	}
	result := make([]string, 0, len(conflicts))
	for r := range conflicts {
		result = append(result, r)
	}
	sort.Strings(result)
	return result
}

func finalizeActorActivityCounts(actors map[string]*model.ActorSummary, activities map[string]*activityState) {
	for _, a := range activities {
		if a.activity.Actor == "" {
			continue
		}
		s := actorSummary(actors, a.activity.Actor)
		if a.status == "active" {
			s.Active++
		} else if a.status == "paused" {
			s.Paused++
		}
	}
}
func finalize(report *model.Report, actors map[string]*model.ActorSummary) {
	for _, f := range report.Findings {
		switch f.Severity {
		case model.SeverityError:
			report.Summary.Errors++
		case model.SeverityWarning:
			report.Summary.Warnings++
		default:
			report.Summary.Information++
		}
	}
	if report.Summary.Errors > 0 {
		report.Summary.Status = "fail"
	} else if report.Summary.Warnings > 0 {
		report.Summary.Status = "pass_with_warnings"
	} else {
		report.Summary.Status = "pass"
	}
	names := make([]string, 0, len(actors))
	for name := range actors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		report.Actors = append(report.Actors, *actors[name])
	}
}

func add(r *model.Report, s model.Severity, code, message string, e model.Event) {
	r.Findings = append(r.Findings, model.Finding{Severity: s, Code: code, Message: message, Sequence: e.Sequence, EventID: e.ID, Actor: e.Actor})
}
func actorSummary(actors map[string]*model.ActorSummary, actor string) *model.ActorSummary {
	if actors[actor] == nil {
		actors[actor] = &model.ActorSummary{Actor: actor}
	}
	return actors[actor]
}
func set(values []string) map[string]bool {
	r := map[string]bool{}
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			r[v] = true
		}
	}
	return r
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func sequenceRange(from, to int64) string {
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d-%d", from, to)
}
func displayID(id string) string {
	if id == "" {
		return "<missing id>"
	}
	return id
}
