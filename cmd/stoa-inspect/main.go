package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MiloSharp/Stoa-Inspect/internal/audit"
	"github.com/MiloSharp/Stoa-Inspect/internal/eventio"
	"github.com/MiloSharp/Stoa-Inspect/internal/model"
	"github.com/MiloSharp/Stoa-Inspect/internal/report"
)

const version = "0.1.0"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		usage(stdout)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintf(stdout, "stoa-inspect %s\n", version)
		return 0
	}
	if args[0] != "audit" {
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
	return runAudit(args[1:], stdout, stderr)
}
func runAudit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "human", "output format: human or json")
	severityText := flags.String("severity", "info", "minimum displayed severity")
	actor := flags.String("actor", "", "show one actor's findings")
	from := flags.Int64("from-sequence", 0, "first sequence to include")
	to := flags.Int64("to-sequence", 0, "last sequence to include")
	output := flags.String("output", "", "write report to file")
	flags.Usage = func() { auditUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "audit requires exactly one input file, or - for stdin")
		auditUsage(stderr)
		return 2
	}
	if *from < 0 || *to < 0 || (*to > 0 && *from > *to) {
		fmt.Fprintln(stderr, "invalid sequence range")
		return 2
	}
	severity, err := model.ParseSeverity(*severityText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *format != "human" && *format != "json" {
		fmt.Fprintln(stderr, "unknown format: use human or json")
		return 2
	}
	source := flags.Arg(0)
	reader, closeInput, err := openInput(source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer closeInput()
	events, err := eventio.Read(reader)
	if err != nil {
		fmt.Fprintf(stderr, "cannot read events: %v\n", err)
		return 2
	}
	destination, closeOutput, err := openOutput(*output, stdout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer closeOutput()
	result := audit.Run(events, source, version, audit.Options{FromSequence: *from, ToSequence: *to})
	view := report.Options{MinimumSeverity: severity, Actor: strings.TrimSpace(*actor)}
	if *format == "json" {
		err = report.JSON(destination, result, view)
	} else {
		err = report.Human(destination, result, view)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 2
	}
	if result.Summary.Errors > 0 {
		return 1
	}
	return 0
}
func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open input: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}
func openOutput(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create output: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}
func usage(w io.Writer) {
	fmt.Fprintln(w, "Stoa-Inspect audits exported Stoa event streams without changing them.\n\nUsage:\n  stoa-inspect audit [options] <events.json|events.ndjson|->\n  stoa-inspect version\n\nHuman-readable output is the default. Use --format json for automation.")
}
func auditUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: stoa-inspect audit [options] <events.json|events.ndjson|->\n\nOptions:\n  --format human|json\n  --severity info|warning|error\n  --actor NAME\n  --from-sequence N\n  --to-sequence N\n  --output FILE")
}
