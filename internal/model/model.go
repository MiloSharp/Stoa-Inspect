package model

import (
	"fmt"
	"strings"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

func ParseSeverity(value string) (Severity, error) {
	switch Severity(strings.ToLower(value)) {
	case SeverityInfo:
		return SeverityInfo, nil
	case SeverityWarning:
		return SeverityWarning, nil
	case SeverityError:
		return SeverityError, nil
	default:
		return "", fmt.Errorf("unknown severity %q: use info, warning, or error", value)
	}
}

func (s Severity) Rank() int {
	switch s {
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	default:
		return 1
	}
}

type Activity struct {
	ID               string         `json:"id"`
	Actor            string         `json:"actor"`
	Name             string         `json:"name"`
	StartAt          int64          `json:"startAt"`
	EndAt            int64          `json:"endAt"`
	Reads            []string       `json:"reads,omitempty"`
	Writes           []string       `json:"writes,omitempty"`
	InterruptSignals []string       `json:"interruptSignals,omitempty"`
	Location         string         `json:"location,omitempty"`
	Kind             string         `json:"kind,omitempty"`
	Destination      string         `json:"destination,omitempty"`
	TargetActor      string         `json:"targetActor,omitempty"`
	Status           string         `json:"status,omitempty"`
	Effects          map[string]any `json:"effects,omitempty"`
}

type Proposal struct {
	ID     string `json:"id"`
	Actor  string `json:"actor"`
	Status string `json:"status"`
}

type Event struct {
	ID          string          `json:"id"`
	Sequence    int64           `json:"sequence"`
	Type        string          `json:"type"`
	SimulatedAt int64           `json:"simulatedAt"`
	Actor       string          `json:"actor,omitempty"`
	Visibility  string          `json:"visibility,omitempty"`
	Activity    *Activity       `json:"activity,omitempty"`
	ActivityID  string          `json:"activityId,omitempty"`
	Mode        string          `json:"mode,omitempty"`
	Remaining   int64           `json:"remaining,omitempty"`
	Proposal    *Proposal       `json:"proposal,omitempty"`
	ProposalID  string          `json:"proposalId,omitempty"`
	Resolution  string          `json:"resolution,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Target      string          `json:"target,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Content     string          `json:"content,omitempty"`
	From        int64           `json:"from,omitempty"`
	To          int64           `json:"to,omitempty"`
	Present     map[string]bool `json:"-"`
}

type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Sequence int64    `json:"sequence,omitempty"`
	EventID  string   `json:"eventId,omitempty"`
	Actor    string   `json:"actor,omitempty"`
}

type ActorSummary struct {
	Actor       string `json:"actor"`
	Events      int    `json:"events"`
	Scheduled   int    `json:"scheduled"`
	Completed   int    `json:"completed"`
	Interrupted int    `json:"interrupted"`
	Active      int    `json:"active"`
	Paused      int    `json:"paused"`
}

type Summary struct {
	Status        string         `json:"status"`
	Events        int            `json:"events"`
	Errors        int            `json:"errors"`
	Warnings      int            `json:"warnings"`
	Information   int            `json:"information"`
	FirstSequence int64          `json:"firstSequence,omitempty"`
	LastSequence  int64          `json:"lastSequence,omitempty"`
	FinalClock    int64          `json:"finalClock,omitempty"`
	EventTypes    map[string]int `json:"eventTypes"`
}

type Filters struct {
	MinimumSeverity Severity `json:"minimumSeverity,omitempty"`
	Actor           string   `json:"actor,omitempty"`
}

type Report struct {
	SchemaVersion string         `json:"schemaVersion"`
	ToolVersion   string         `json:"toolVersion"`
	Source        string         `json:"source"`
	Summary       Summary        `json:"summary"`
	Findings      []Finding      `json:"findings"`
	Actors        []ActorSummary `json:"actors"`
	Filters       *Filters       `json:"filters,omitempty"`
}
