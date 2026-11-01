package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"jplings/internal/content"
	"jplings/internal/corpus"
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
	case "content":
		return contentCommand(args[1:])
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
  jplings content import  fetch JMdict + Tatoeba reference data
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
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		runner.Evaluator = eval.NewClient(key)
	} else {
		fmt.Fprintln(os.Stderr, "jplings: DEEPSEEK_API_KEY unset — free-form responses will be saved ungraded")
	}

	return tui.Run(runner, units)
}

func contentCommand(args []string) error {
	if len(args) == 0 || args[0] != "import" {
		return fmt.Errorf("usage: jplings content import [--out dir] [--words file] [--sentences n]")
	}

	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	outDir := flags.String("out", "corpus", "where to write vocab.yaml and sentences.yaml")
	wordsPath := flags.String("words", filepath.Join("content", "words.txt"), "word list to look up in JMdict")
	limit := flags.Int("sentences", 200, "how many sentence pairs to export")
	minRunes := flags.Int("min-length", 6, "reject sentences shorter than this many characters")
	onlyKnown := flags.Bool("require-known", true, "keep only sentences containing curated vocabulary")
	cacheDir := flags.String("cache", "", "download cache (default: user cache dir)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	cache := *cacheDir
	if cache == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		cache = filepath.Join(base, "jplings")
	}

	fmt.Fprintf(os.Stderr, "fetching sources into %s\n", cache)
	files, err := corpus.Fetch(cache)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	want, err := readWordList(*wordsPath)
	if err != nil {
		return err
	}
	require := want
	if !*onlyKnown {
		require = nil
	}
	if len(want) > 0 {
		jmdict, err := corpus.OpenJMdict(files.JMdict)
		if err != nil {
			return err
		}
		lookup, err := corpus.LoadJMdict(jmdict, want)
		jmdict.Close()
		if err != nil {
			return err
		}
		words := lookup.Words()
		if err := writeFile(filepath.Join(*outDir, "vocab.yaml"), func(w io.Writer) error {
			return corpus.WriteVocab(w, words)
		}); err != nil {
			return err
		}
		matched := map[string]bool{}
		for _, word := range words {
			for _, form := range append(append([]string{}, word.Kanji...), word.Kana...) {
				if want[form] {
					matched[form] = true
				}
			}
		}
		fmt.Printf("vocab: %d entries covering %d of %d requested words\n", len(words), len(matched), len(want))
		for _, form := range sortedKeys(want) {
			if !matched[form] {
				fmt.Fprintf(os.Stderr, "  not in JMdict: %s\n", form)
			}
		}
	}

	pairs, err := importSentences(files, *limit, *minRunes, require)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(*outDir, "sentences.yaml"), func(w io.Writer) error {
		return corpus.WriteSentences(w, pairs)
	}); err != nil {
		return err
	}
	fmt.Printf("sentences: %d pairs\n", len(pairs))
	fmt.Printf("\nlicences: JMdict %s, Tatoeba %s — see ATTRIBUTION.md\n", corpus.JMdictLic, corpus.TatoebaLic)
	return nil
}

func importSentences(files corpus.Files, limit int, minRunes int, require map[string]bool) ([]corpus.Sentence, error) {
	japanese, err := corpus.OpenBzip2(files.Japanese)
	if err != nil {
		return nil, err
	}
	jpnSentences, err := corpus.ReadSentences(japanese)
	japanese.Close()
	if err != nil {
		return nil, err
	}

	english, err := corpus.OpenBzip2(files.English)
	if err != nil {
		return nil, err
	}
	engSentences, err := corpus.ReadSentences(english)
	english.Close()
	if err != nil {
		return nil, err
	}

	links, err := corpus.OpenBzip2(files.Links)
	if err != nil {
		return nil, err
	}
	linksPairs, err := corpus.ReadLinks(links)
	links.Close()
	if err != nil {
		return nil, err
	}

	joined := corpus.Pairs(jpnSentences, engSentences, linksPairs)
	if err := corpus.Validate(joined); err != nil {
		return nil, err
	}
	usable := corpus.Filter(joined, corpus.FilterOptions{MinRunes: minRunes, Require: require})
	return corpus.SelectShort(usable, limit), nil
}

func readWordList(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	want := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		want[line] = true
	}
	return want, nil
}

func writeFile(path string, write func(io.Writer) error) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(file); err != nil {
		file.Close()
		return err
	}
	return file.Close()
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

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
