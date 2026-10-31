package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"jplings/internal/content"
	"jplings/internal/doctor"
	"jplings/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "jplings: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return tui.Run()
	}
	switch args[0] {
	case "lint":
		return lint(args[1:])
	case "doctor":
		return runDoctor()
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Print(`jplings — Japanese learning system

Usage:
  jplings                 start a session (TUI)
  jplings lint [path...]  validate content YAML (default: content/)
  jplings doctor          check host prerequisites
`)
}

func lint(paths []string) error {
	if len(paths) == 0 {
		paths = []string{"content"}
	}
	units, issues, err := content.Load(paths...)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, issue)
		}
		return fmt.Errorf("%d content issue(s)", len(issues))
	}
	fmt.Printf("%d unit(s) OK\n", len(units))
	return nil
}

func runDoctor() error {
	checks := doctor.Run(doctor.Default())

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, c := range checks {
		required := ""
		if c.Required {
			required = "required"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", status(c), c.Name, c.Detail, required)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if failed := doctor.Failures(checks); len(failed) > 0 {
		return fmt.Errorf("%d required prerequisite(s) missing", len(failed))
	}
	return nil
}

func status(c doctor.Check) string {
	switch {
	case c.Status == doctor.OK:
		return "ok"
	case c.Required:
		return "missing"
	default:
		return "warn"
	}
}
