package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"jplings/internal/content"
	"jplings/internal/doctor"
	"jplings/internal/eval"
	"jplings/internal/profile"
	"jplings/internal/session"
	"jplings/internal/speech"
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
		return startSession()
	}
	switch args[0] {
	case "lint":
		return lint(args[1:])
	case "doctor":
		return runDoctor()
	case "grade":
		return gradePending(args[1:])
	case "coverage":
		return reportCoverage(args[1:])
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
  jplings                 start or resume a session (TUI)
  jplings lint [path...]  validate content YAML (default: content/)
  jplings doctor          check host prerequisites
  jplings grade --pending grade saved responses by hand
  jplings coverage        report how deeply each concept is exercised
`)
}

func profilePath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "jplings", "profile.db"), nil
}

func startSession() error {
	units, issues, err := content.Load("content")
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, issue)
		}
		return fmt.Errorf("%d content issue(s)", len(issues))
	}

	path, err := profilePath()
	if err != nil {
		return err
	}
	store, err := profile.Open(path)
	if err != nil {
		return err
	}
	defer store.Close()

	runner := &session.Runner{
		Store:  store,
		Client: speech.NewHTTPClient(),
		Voice:  "jf_alpha",
	}
	// A missing key is a warning, not a blocker: free-form responses then save
	// ungraded instead of failing the session (D3).
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		runner.Evaluator = eval.NewClient(key)
	} else {
		fmt.Fprintln(os.Stderr, "jplings: DEEPSEEK_API_KEY unset — free-form responses will be saved ungraded")
	}

	return tui.Run(runner, units)
}

func reportCoverage(paths []string) error {
	if len(paths) == 0 {
		paths = []string{"content"}
	}
	units, issues, err := content.Load(paths...)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return fmt.Errorf("content has %d issue(s); run jplings lint first", len(issues))
	}

	coverage := content.CoverageOf(units)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	currentUnit := ""
	complete, total := 0, 0
	for _, entry := range coverage {
		if entry.UnitID != currentUnit {
			currentUnit = entry.UnitID
			fmt.Fprintf(w, "\n%s\n", entry.UnitID)
		}
		total++
		state := "ok"
		if entry.Complete() {
			complete++
		} else {
			state = "THIN"
		}
		fmt.Fprintf(w, "  %s\tcomprehension %d\tproduction %d\t%s\n",
			entry.ConceptID, entry.Comprehension, entry.Production, state)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("\n%d/%d concepts deeply exercised\n", complete, total)
	return nil
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

// gradePending applies the same four-dimension rubric by hand, so a DeepSeek outage
// delays speaking evidence but never blocks it (D3).
func gradePending(args []string) error {
	if len(args) == 0 || args[0] != "--pending" {
		return fmt.Errorf("usage: jplings grade --pending")
	}

	path, err := profilePath()
	if err != nil {
		return err
	}
	store, err := profile.Open(path)
	if err != nil {
		return err
	}
	defer store.Close()

	pending, err := store.PendingNeedsManual()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Println("no responses waiting for manual grading")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	for _, item := range pending {
		fmt.Printf("\n%s\n", strings.Repeat("-", 60))
		fmt.Printf("prompt:   %s\n", item.Prompt)
		fmt.Printf("response: %s\n", item.Response)

		scores := make([]int, 0, 4)
		for _, dimension := range []string{"meaning", "grammar", "naturalness", "register"} {
			score, err := readScore(reader, dimension)
			if err != nil {
				return err
			}
			scores = append(scores, score)
		}

		fmt.Print("feedback: ")
		feedback, _ := reader.ReadString('\n')
		feedback = strings.TrimSpace(feedback)

		// Human grading is ordinary evidence, so it counts immediately: the audit
		// gate exists for model output, not for Alan's own judgement.
		for _, conceptID := range []string{item.StepID} {
			if err := store.Record(profile.Evidence{
				ConceptID: conceptID, Dimension: profile.Production,
				Correct:    scores[0] >= 1 && scores[1] >= 1,
				StimulusID: item.StepID, Source: "human", At: time.Now(),
			}); err != nil {
				return err
			}
		}
		if feedback != "" {
			if err := store.AddError(item.StepID, feedback, time.Now()); err != nil {
				return err
			}
		}
		if err := store.ResolvePending(item.ID); err != nil {
			return err
		}
		fmt.Println("graded.")
	}
	return nil
}

func readScore(reader *bufio.Reader, dimension string) (int, error) {
	for {
		fmt.Printf("%-12s (0 wrong, 1 flawed, 2 good): ", dimension)
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		value, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || value < 0 || value > 2 {
			fmt.Println("please enter 0, 1 or 2")
			continue
		}
		return value, nil
	}
}
