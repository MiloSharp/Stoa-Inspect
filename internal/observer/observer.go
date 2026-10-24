package observer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MiloSharp/Stoa-Inspect/internal/audit"
	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

type View struct {
	Source        string          `json:"source"`
	Clock         int64           `json:"clock"`
	Paused        bool            `json:"paused"`
	VisibleEvents int             `json:"visibleEvents"`
	TotalEvents   int             `json:"totalEvents"`
	PrivateEvents int             `json:"privateEvents"`
	Actors        []Actor         `json:"actors"`
	Locations     []Location      `json:"locations"`
	Proposals     []Proposal      `json:"proposals"`
	Chronicle     []ChronicleItem `json:"chronicle"`
	Findings      []model.Finding `json:"findings"`
	AuditStatus   string          `json:"auditStatus"`
}

type Actor struct {
	Name       string `json:"name"`
	Location   string `json:"location"`
	Activity   string `json:"activity,omitempty"`
	Status     string `json:"status"`
	LastSeenAt int64  `json:"lastSeenAt"`
}

type Location struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Actors      []string `json:"actors"`
	Exits       []Exit   `json:"exits"`
}

type Exit struct {
	Destination string `json:"destination"`
	Duration    int64  `json:"duration"`
}

type Proposal struct {
	ID         string `json:"id"`
	Actor      string `json:"actor,omitempty"`
	Status     string `json:"status"`
	Resolution string `json:"resolution,omitempty"`
}

type ChronicleItem struct {
	Sequence   int64  `json:"sequence"`
	Time       int64  `json:"time"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	Actor      string `json:"actor,omitempty"`
	Location   string `json:"location,omitempty"`
	Visibility string `json:"visibility"`
}

type activityState struct {
	Activity model.Activity
	Status   string
	Origin   string
}

func Build(events []model.Event, source, toolVersion string, includePrivate bool) View {
	return BuildWithWorld(events, source, toolVersion, includePrivate, nil)
}

func BuildWithWorld(events []model.Event, source, toolVersion string, includePrivate bool, world map[string]model.WorldLocation) View {
	result := audit.Run(events, source, toolVersion, audit.Options{})
	view := View{
		Source: source, TotalEvents: len(events), Findings: result.Findings, AuditStatus: result.Summary.Status,
		Actors: []Actor{}, Locations: []Location{}, Proposals: []Proposal{}, Chronicle: []ChronicleItem{},
	}
	ordered := append([]model.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Sequence < ordered[j].Sequence })

	actors := map[string]*Actor{}
	activities := map[string]*activityState{}
	proposals := map[string]*Proposal{}

	for _, event := range ordered {
		if event.SimulatedAt > view.Clock {
			view.Clock = event.SimulatedAt
		}
		if event.Visibility == "private" {
			view.PrivateEvents++
			if !includePrivate {
				continue
			}
		}
		view.VisibleEvents++
		switch event.Type {
		case "simulation.pause":
			view.Paused = true
		case "simulation.resume":
			view.Paused = false
		case "activity.scheduled":
			if event.Activity != nil {
				activity := *event.Activity
				if activity.Actor == "" {
					activity.Actor = event.Actor
				}
				actor := ensureActor(actors, activity.Actor, event.SimulatedAt)
				origin := actor.Location
				if origin == "" {
					origin = activity.Location
				}
				activities[activity.ID] = &activityState{Activity: activity, Status: "active", Origin: origin}
				actor.Activity, actor.Status = activity.Name, "active"
				if activity.Destination == "" && activity.Location != "" {
					actor.Location = activity.Location
				} else if actor.Location == "" {
					actor.Location = origin
				}
			}
		case "activity.completed", "activity.cancelled":
			if activity := activities[event.ActivityID]; activity != nil {
				activity.Status = strings.TrimPrefix(event.Type, "activity.")
				actor := ensureActor(actors, activity.Activity.Actor, event.SimulatedAt)
				actor.Activity, actor.Status = "", "idle"
				if event.Type == "activity.completed" && activity.Activity.Destination != "" {
					actor.Location = activity.Activity.Destination
				}
			}
		case "activity.interrupted":
			if activity := activities[event.ActivityID]; activity != nil {
				activity.Status = event.Mode
				actor := ensureActor(actors, activity.Activity.Actor, event.SimulatedAt)
				if event.Mode == "pause" {
					actor.Status = "paused"
				} else {
					actor.Activity, actor.Status = "", "idle"
				}
			}
		case "activity.resumed":
			if activity := activities[event.ActivityID]; activity != nil {
				activity.Status = "active"
				actor := ensureActor(actors, activity.Activity.Actor, event.SimulatedAt)
				actor.Activity, actor.Status = activity.Activity.Name, "active"
			}
		case "proposal.created":
			if event.Proposal != nil {
				proposals[event.Proposal.ID] = &Proposal{ID: event.Proposal.ID, Actor: firstNonEmpty(event.Proposal.Actor, event.Actor), Status: "pending"}
			}
		case "proposal.resolved":
			if proposal := proposals[event.ProposalID]; proposal != nil {
				proposal.Status, proposal.Resolution = "resolved", event.Resolution
			}
		}
		if event.Actor != "" && event.Actor != "system" {
			ensureActor(actors, event.Actor, event.SimulatedAt)
		}
		view.Chronicle = append(view.Chronicle, describe(event, activities))
	}

	for _, actor := range actors {
		if actor.Location == "" {
			actor.Location = "Unlocated"
		}
		if actor.Status == "" {
			actor.Status = "idle"
		}
		view.Actors = append(view.Actors, *actor)
	}
	sort.Slice(view.Actors, func(i, j int) bool {
		return strings.ToLower(view.Actors[i].Name) < strings.ToLower(view.Actors[j].Name)
	})
	locations := map[string][]string{}
	for _, actor := range view.Actors {
		locations[actor.Location] = append(locations[actor.Location], actor.Name)
	}
	for name := range world {
		if _, exists := locations[name]; !exists {
			locations[name] = []string{}
		}
	}
	for location, names := range locations {
		sort.Strings(names)
		item := Location{Name: location, Actors: names, Exits: []Exit{}}
		if known, exists := world[location]; exists {
			item.Description = known.Description
			for destination, exit := range known.Exits {
				item.Exits = append(item.Exits, Exit{Destination: destination, Duration: exit.Duration})
			}
			sort.Slice(item.Exits, func(i, j int) bool {
				return strings.ToLower(item.Exits[i].Destination) < strings.ToLower(item.Exits[j].Destination)
			})
		}
		view.Locations = append(view.Locations, item)
	}
	sort.Slice(view.Locations, func(i, j int) bool {
		return strings.ToLower(view.Locations[i].Name) < strings.ToLower(view.Locations[j].Name)
	})
	for _, proposal := range proposals {
		view.Proposals = append(view.Proposals, *proposal)
	}
	sort.Slice(view.Proposals, func(i, j int) bool { return view.Proposals[i].ID < view.Proposals[j].ID })
	return view
}

func ensureActor(actors map[string]*Actor, name string, at int64) *Actor {
	if name == "" {
		name = "Unknown"
	}
	if actors[name] == nil {
		actors[name] = &Actor{Name: name, Status: "idle"}
	}
	if at >= actors[name].LastSeenAt {
		actors[name].LastSeenAt = at
	}
	return actors[name]
}

func describe(event model.Event, activities map[string]*activityState) ChronicleItem {
	item := ChronicleItem{Sequence: event.Sequence, Time: event.SimulatedAt, Actor: event.Actor, Visibility: event.Visibility, Title: humanType(event.Type)}
	switch event.Type {
	case "activity.scheduled":
		if event.Activity != nil {
			item.Location = event.Activity.Location
			if event.Activity.Destination != "" {
				origin := event.Activity.Location
				if activity := activities[event.Activity.ID]; activity != nil {
					origin = activity.Origin
				}
				item.Text = fmt.Sprintf("%s set out from %s toward %s.", displayName(event.Actor), firstNonEmpty(origin, "an unknown place"), event.Activity.Destination)
			} else {
				item.Text = fmt.Sprintf("%s began %s%s.", displayName(event.Actor), event.Activity.Name, atLocation(event.Activity.Location))
			}
		}
	case "activity.completed":
		if activity := activities[event.ActivityID]; activity != nil {
			if activity.Activity.Destination != "" {
				item.Location = activity.Activity.Destination
				item.Text = fmt.Sprintf("%s arrived at %s.", displayName(event.Actor), activity.Activity.Destination)
			} else {
				item.Location = activity.Activity.Location
				item.Text = fmt.Sprintf("%s completed %s%s.", displayName(event.Actor), activity.Activity.Name, atLocation(activity.Activity.Location))
			}
		}
	case "activity.interrupted":
		if activity := activities[event.ActivityID]; activity != nil {
			item.Location = activity.Activity.Location
			item.Text = fmt.Sprintf("%s's work on %s was interrupted (%s).", displayName(event.Actor), activity.Activity.Name, firstNonEmpty(event.Mode, "unspecified"))
		}
	case "activity.resumed":
		if activity := activities[event.ActivityID]; activity != nil {
			item.Location = activity.Activity.Location
			item.Text = fmt.Sprintf("%s resumed %s%s.", displayName(event.Actor), activity.Activity.Name, atLocation(activity.Activity.Location))
		}
	case "activity.cancelled":
		if activity := activities[event.ActivityID]; activity != nil {
			item.Location = activity.Activity.Location
			item.Text = fmt.Sprintf("%s stopped %s%s.", displayName(event.Actor), activity.Activity.Name, atLocation(activity.Activity.Location))
		}
	case "proposal.created":
		if event.Proposal != nil {
			item.Text = fmt.Sprintf("%s created proposal %s.", displayName(event.Actor), event.Proposal.ID)
		}
	case "proposal.resolved":
		if event.Resolution == "failed" {
			item.Text = fmt.Sprintf("Proposal %s failed.", event.ProposalID)
		} else {
			item.Text = fmt.Sprintf("Proposal %s was %s.", event.ProposalID, firstNonEmpty(event.Resolution, "resolved"))
		}
	case "simulation.pause":
		item.Text = "The simulation paused."
	case "simulation.resume":
		item.Text = "The simulation resumed."
	case "time.advanced":
		item.Text = fmt.Sprintf("Time advanced from t=%d to t=%d.", event.From, event.To)
	case "world.set":
		item.Text = fmt.Sprintf("The world state changed: %s.", firstNonEmpty(event.Target, "unspecified target"))
	case "memory.recorded":
		item.Text = fmt.Sprintf("%s recorded a private memory.", displayName(firstNonEmpty(event.Owner, event.Actor)))
	case "scenario.loaded":
		item.Text = "A scenario was loaded."
	default:
		item.Text = fmt.Sprintf("%s produced %s.", displayName(event.Actor), humanType(event.Type))
	}
	if item.Text == "" {
		item.Text = fmt.Sprintf("%s occurred.", humanType(event.Type))
	}
	return item
}

func humanType(value string) string {
	value = strings.ReplaceAll(value, ".", " ")
	if value == "" {
		return "Unknown event"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
func displayName(value string) string {
	if value == "" || value == "system" {
		return "The system"
	}
	return value
}
func atLocation(location string) string {
	if location == "" {
		return ""
	}
	return " at " + location
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
