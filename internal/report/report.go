package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

type Options struct {
	MinimumSeverity model.Severity
	Actor           string
}

func Human(writer io.Writer, source model.Report, options Options) error {
	r := Filter(source, options)
	w := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "STOA EVENT AUDIT")
	fmt.Fprintf(w, "Source:\t%s\n", r.Source)
	fmt.Fprintf(w, "Result:\t%s\n", strings.ToUpper(strings.ReplaceAll(r.Summary.Status, "_", " ")))
	fmt.Fprintf(w, "Events:\t%d\n", r.Summary.Events)
	if r.Summary.Events > 0 {
		fmt.Fprintf(w, "Sequence:\t%d-%d\n", r.Summary.FirstSequence, r.Summary.LastSequence)
		fmt.Fprintf(w, "Final clock:\tt=%d\n", r.Summary.FinalClock)
	}
	fmt.Fprintf(w, "Findings:\t%d errors, %d warnings, %d information\n", r.Summary.Errors, r.Summary.Warnings, r.Summary.Information)
	if r.Filters != nil && (r.Filters.Actor != "" || r.Filters.MinimumSeverity != model.SeverityInfo) {
		fmt.Fprintf(w, "View:\tseverity >= %s", r.Filters.MinimumSeverity)
		if r.Filters.Actor != "" {
			fmt.Fprintf(w, ", actor = %s (global findings retained)", r.Filters.Actor)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FINDINGS")
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "No findings match this view.")
	} else {
		for _, f := range r.Findings {
			location := "global"
			if f.Sequence > 0 {
				location = fmt.Sprintf("seq %d", f.Sequence)
			}
			if f.Actor != "" {
				location += ", " + f.Actor
			}
			fmt.Fprintf(w, "%s\t[%s]\t%s\t%s\n", strings.ToUpper(string(f.Severity)), f.Code, location, f.Message)
		}
	}
	if len(r.Actors) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ACTORS")
		fmt.Fprintln(w, "Actor\tEvents\tScheduled\tCompleted\tInterrupted\tActive\tPaused")
		for _, a := range r.Actors {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n", a.Actor, a.Events, a.Scheduled, a.Completed, a.Interrupted, a.Active, a.Paused)
		}
	}
	return w.Flush()
}

func JSON(writer io.Writer, source model.Report, options Options) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(Filter(source, options))
}

func Filter(source model.Report, options Options) model.Report {
	if options.MinimumSeverity == "" {
		options.MinimumSeverity = model.SeverityInfo
	}
	result := source
	result.Filters = &model.Filters{MinimumSeverity: options.MinimumSeverity, Actor: options.Actor}
	result.Findings = make([]model.Finding, 0, len(source.Findings))
	for _, f := range source.Findings {
		if f.Severity.Rank() < options.MinimumSeverity.Rank() {
			continue
		}
		if options.Actor != "" && f.Actor != "" && !strings.EqualFold(f.Actor, options.Actor) {
			continue
		}
		result.Findings = append(result.Findings, f)
	}
	sort.SliceStable(result.Findings, func(i, j int) bool {
		if result.Findings[i].Sequence == result.Findings[j].Sequence {
			return result.Findings[i].Severity.Rank() > result.Findings[j].Severity.Rank()
		}
		return result.Findings[i].Sequence < result.Findings[j].Sequence
	})
	result.Actors = make([]model.ActorSummary, 0, len(source.Actors))
	for _, a := range source.Actors {
		if options.Actor == "" || strings.EqualFold(a.Actor, options.Actor) {
			result.Actors = append(result.Actors, a)
		}
	}
	return result
}
