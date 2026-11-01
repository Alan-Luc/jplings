// Package profile is the persistent learner record (R5, D5).
//
// Progress must survive a crash mid-session, so it lives in SQLite rather than a
// JSON file. The honest-gating rules from R6 are implemented here, in code, so that
// neither the learner nor a future feature can skip past them.
package profile

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"jplings/internal/schedule"
)

// Evidence dimensions. A concept is only mastered when production, generalization
// and retention have each been demonstrated.
const (
	Recognition    = "recognition"
	Comprehension  = "comprehension"
	Explicit       = "explicit"
	Production     = "production"
	Generalization = "generalization"
	Retention      = "retention"
)

const schema = `
CREATE TABLE IF NOT EXISTS concepts (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS evidence (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  concept_id TEXT NOT NULL,
  dimension TEXT NOT NULL,
  correct INTEGER NOT NULL,
  stimulus_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT 'typed',
  at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS evidence_concept ON evidence(concept_id, dimension);
CREATE TABLE IF NOT EXISTS llm_trust (
  concept_id TEXT NOT NULL,
  dimension TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'provisional',
  PRIMARY KEY (concept_id, dimension)
);
CREATE TABLE IF NOT EXISTS errors (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  concept_id TEXT NOT NULL,
  detail TEXT NOT NULL,
  at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS reviews (
  concept_id TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'review',
  ease REAL NOT NULL,
  interval_days INTEGER NOT NULL,
  reps INTEGER NOT NULL,
  due_at INTEGER NOT NULL,
  PRIMARY KEY (concept_id, kind)
);
CREATE TABLE IF NOT EXISTS pending (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  step_id TEXT NOT NULL,
  prompt TEXT NOT NULL,
  response TEXT NOT NULL,
  retries INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS session_cursor (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  unit_id TEXT NOT NULL,
  lesson_id TEXT NOT NULL,
  step_index INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
`

type Store struct {
	db   *sql.DB
	path string
}

type Evidence struct {
	ConceptID  string
	Dimension  string
	Correct    bool
	StimulusID string
	Source     string // typed, spoken, llm, human
	At         time.Time
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// WAL with a busy timeout keeps a mid-session crash from corrupting the record.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Record(e Evidence) error {
	correct := 0
	if e.Correct {
		correct = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO evidence (concept_id, dimension, correct, stimulus_id, source, at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.ConceptID, e.Dimension, correct, e.StimulusID, e.Source, e.At.Unix(),
	)
	return err
}

func (s *Store) Evidence(conceptID string) ([]Evidence, error) {
	rows, err := s.db.Query(
		`SELECT concept_id, dimension, correct, stimulus_id, source, at
		 FROM evidence WHERE concept_id = ? ORDER BY at, id`, conceptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Evidence
	for rows.Next() {
		var e Evidence
		var correct int
		var at int64
		if err := rows.Scan(&e.ConceptID, &e.Dimension, &correct, &e.StimulusID, &e.Source, &at); err != nil {
			return nil, err
		}
		e.Correct = correct == 1
		e.At = time.Unix(at, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetLLMTrust records whether LLM-graded evidence for a dimension may count toward
// mastery: provisional, trusted, or withdrawn (D3 audit gate).
func (s *Store) SetLLMTrust(conceptID, dimension, status string) error {
	_, err := s.db.Exec(
		`INSERT INTO llm_trust (concept_id, dimension, status) VALUES (?, ?, ?)
		 ON CONFLICT(concept_id, dimension) DO UPDATE SET status = excluded.status`,
		conceptID, dimension, status,
	)
	return err
}

func (s *Store) llmTrusted(conceptID, dimension string) (bool, error) {
	var status string
	err := s.db.QueryRow(
		`SELECT status FROM llm_trust WHERE concept_id = ? AND dimension = ?`,
		conceptID, dimension,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return status == "trusted", nil
}

// Mastered applies the R6 gate. A concept is mastered only with production,
// generalization and delayed retention evidence, and LLM-graded evidence counts only
// once the audit has promoted that dimension to trusted.
func (s *Store) Mastered(conceptID string) (bool, []string, error) {
	satisfied := map[string]bool{}
	for _, dimension := range []string{Production, Generalization, Retention} {
		ok, err := s.dimensionSatisfied(conceptID, dimension)
		if err != nil {
			return false, nil, err
		}
		satisfied[dimension] = ok
	}

	var missing []string
	for _, dimension := range []string{Production, Generalization, Retention} {
		if !satisfied[dimension] {
			missing = append(missing, dimension)
		}
	}
	return len(missing) == 0, missing, nil
}

func (s *Store) dimensionSatisfied(conceptID, dimension string) (bool, error) {
	trusted, err := s.llmTrusted(conceptID, dimension)
	if err != nil {
		return false, err
	}

	rows, err := s.db.Query(
		`SELECT source FROM evidence
		 WHERE concept_id = ? AND dimension = ? AND correct = 1`,
		conceptID, dimension)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			return false, err
		}
		// Provisional LLM evidence is visible but cannot alone clear a mastery gate.
		if source != "llm" || trusted {
			return true, nil
		}
	}
	return false, rows.Err()
}

// CanSkip applies the R6 pre-knowledge gate: skipping requires two production
// demonstrations on different stimuli, so repeating one sentence never counts.
func (s *Store) CanSkip(conceptID string) (bool, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT stimulus_id FROM evidence
		 WHERE concept_id = ? AND dimension = ? AND correct = 1 AND stimulus_id <> ''`,
		conceptID, Production)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var stimuli []string
	for rows.Next() {
		var stimulus string
		if err := rows.Scan(&stimulus); err != nil {
			return false, err
		}
		stimuli = append(stimuli, stimulus)
	}
	return len(stimuli) >= 2, rows.Err()
}

func (s *Store) AddError(conceptID, detail string, at time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO errors (concept_id, detail, at) VALUES (?, ?, ?)`,
		conceptID, detail, at.Unix(),
	)
	return err
}

func (s *Store) Errors(conceptID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT detail FROM errors WHERE concept_id = ? ORDER BY at, id`, conceptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var detail string
		if err := rows.Scan(&detail); err != nil {
			return nil, err
		}
		out = append(out, detail)
	}
	return out, rows.Err()
}

func (s *Store) ScheduleReview(card schedule.Card) error {
	kind := card.Kind
	if kind == "" {
		kind = schedule.KindReview
	}
	_, err := s.db.Exec(
		`INSERT INTO reviews (concept_id, kind, ease, interval_days, reps, due_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(concept_id, kind) DO UPDATE SET
		   ease = excluded.ease,
		   interval_days = excluded.interval_days,
		   reps = excluded.reps,
		   due_at = excluded.due_at`,
		card.ConceptID, kind, card.Ease, card.IntervalDays, card.Reps, card.Due.Unix(),
	)
	return err
}

func (s *Store) Reviews() ([]schedule.Card, error) {
	return s.cardsWhere(`1 = 1`)
}

func (s *Store) DueRetention(now time.Time) ([]schedule.Card, error) {
	return s.cardsWhere(fmt.Sprintf(`kind = '%s' AND due_at <= %d`, schedule.KindRetention, now.Unix()))
}

func (s *Store) HasRetentionCheck(conceptID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM reviews WHERE concept_id = ? AND kind = ?`,
		conceptID, schedule.KindRetention,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) cardsWhere(condition string) ([]schedule.Card, error) {
	rows, err := s.db.Query(
		`SELECT concept_id, ease, interval_days, reps, due_at, kind FROM reviews WHERE ` + condition)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []schedule.Card
	for rows.Next() {
		var card schedule.Card
		var due int64
		if err := rows.Scan(&card.ConceptID, &card.Ease, &card.IntervalDays, &card.Reps, &due, &card.Kind); err != nil {
			return nil, err
		}
		card.Due = time.Unix(due, 0)
		out = append(out, card)
	}
	return out, rows.Err()
}

// Pending is a response saved for grading when the evaluator was unavailable (D3).
type Pending struct {
	ID       int64
	StepID   string
	Prompt   string
	Response string
	Retries  int
}

// EnqueuePending saves a response that could not be graded. It is never auto-passed.
func (s *Store) EnqueuePending(stepID, prompt, response string, at time.Time) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO pending (step_id, prompt, response, at) VALUES (?, ?, ?, ?)`,
		stepID, prompt, response, at.Unix(),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) PendingToRetry() ([]Pending, error) {
	return s.pendingWhere(`status = 'pending' AND retries < 2`)
}

func (s *Store) PendingNeedsManual() ([]Pending, error) {
	return s.pendingWhere(`status = 'pending' AND retries >= 2`)
}

func (s *Store) pendingWhere(condition string) ([]Pending, error) {
	rows, err := s.db.Query(
		`SELECT id, step_id, prompt, response, retries FROM pending
		 WHERE ` + condition + ` ORDER BY at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Pending
	for rows.Next() {
		var p Pending
		if err := rows.Scan(&p.ID, &p.StepID, &p.Prompt, &p.Response, &p.Retries); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) RetryPending(id int64) error {
	_, err := s.db.Exec(`UPDATE pending SET retries = retries + 1 WHERE id = ?`, id)
	return err
}

func (s *Store) ResolvePending(id int64) error {
	_, err := s.db.Exec(`UPDATE pending SET status = 'graded' WHERE id = ?`, id)
	return err
}

// SaveCursor persists the exact position of a session so it resumes mid-unit.
func (s *Store) SaveCursor(unitID, lessonID string, stepIndex int, at time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO session_cursor (id, unit_id, lesson_id, step_index, updated_at)
		 VALUES (1, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   unit_id = excluded.unit_id,
		   lesson_id = excluded.lesson_id,
		   step_index = excluded.step_index,
		   updated_at = excluded.updated_at`,
		unitID, lessonID, stepIndex, at.Unix(),
	)
	return err
}

func (s *Store) Cursor() (unitID, lessonID string, stepIndex int, ok bool, err error) {
	err = s.db.QueryRow(
		`SELECT unit_id, lesson_id, step_index FROM session_cursor WHERE id = 1`,
	).Scan(&unitID, &lessonID, &stepIndex)
	if err == sql.ErrNoRows {
		return "", "", 0, false, nil
	}
	if err != nil {
		return "", "", 0, false, err
	}
	return unitID, lessonID, stepIndex, true, nil
}

// Backup writes a consistent snapshot beside the live database and verifies it.
// A corruption is therefore found at backup time rather than weeks later.
func (s *Store) Backup(dir string, keep int) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("jplings-%s.db", time.Now().UTC().Format("20060102-150405")))

	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("backup %s already exists", path)
	}
	// VACUUM INTO produces a single consistent file even while WAL is active.
	if _, err := s.db.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", err
	}
	if err := verify(path); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("backup failed integrity check: %w", err)
	}
	if err := prune(dir, keep); err != nil {
		return "", err
	}
	return path, nil
}

func verify(path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()

	var status string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&status); err != nil {
		return err
	}
	if status != "ok" {
		return fmt.Errorf("integrity_check reported %q", status)
	}
	return nil
}

func prune(dir string, keep int) error {
	entries, err := filepath.Glob(filepath.Join(dir, "jplings-*.db"))
	if err != nil {
		return err
	}
	if keep <= 0 || len(entries) <= keep {
		return nil
	}
	sort.Strings(entries)
	for _, old := range entries[:len(entries)-keep] {
		if err := os.Remove(old); err != nil {
			return err
		}
	}
	return nil
}
