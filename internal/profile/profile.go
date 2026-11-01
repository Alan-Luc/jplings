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

const (
	Comprehension  = "comprehension"
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
CREATE TABLE IF NOT EXISTS errors (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  concept_id TEXT NOT NULL,
  detail TEXT NOT NULL,
  at INTEGER NOT NULL
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
	Source     string
	At         time.Time
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

const reviewsSchema = `
CREATE TABLE reviews (
  step_id TEXT PRIMARY KEY,
  concept_id TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'review',
  ease REAL NOT NULL,
  interval_days INTEGER NOT NULL,
  reps INTEGER NOT NULL,
  due_at INTEGER NOT NULL
);
`

// Card identity moved from concept to step; concept-keyed rows have no step to
// attach to, so they are dropped rather than guessed.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= 1 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`PRAGMA table_info(reviews)`)
	if err != nil {
		return err
	}
	exists, hasStep := false, false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		exists = true
		hasStep = hasStep || name == "step_id"
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if exists && hasStep {
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, 1)); err != nil {
			return err
		}
		return tx.Commit()
	}
	if exists {
		if _, err := tx.Exec(`DROP TABLE reviews`); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(reviewsSchema); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, 1)); err != nil {
		return err
	}
	return tx.Commit()
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

func (s *Store) Mastered(conceptID string) (bool, []string, error) {
	var missing []string

	production, err := s.productionDemonstrated(conceptID)
	if err != nil {
		return false, nil, err
	}
	if !production {
		missing = append(missing, Production)
	}

	retention, err := s.dimensionSatisfied(conceptID, Retention)
	if err != nil {
		return false, nil, err
	}
	if !retention {
		missing = append(missing, Retention)
	}

	return len(missing) == 0, missing, nil
}

// Mastered and CanSkip share this predicate: two distinct production stimuli,
// at least one of them credited by something other than the LLM.
func (s *Store) productionDemonstrated(conceptID string) (bool, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT stimulus_id, source FROM evidence
		 WHERE concept_id = ? AND dimension = ? AND correct = 1 AND stimulus_id <> ''`,
		conceptID, Production)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	stimuli := map[string]bool{}
	nonLLM := false
	for rows.Next() {
		var stimulus, source string
		if err := rows.Scan(&stimulus, &source); err != nil {
			return false, err
		}
		if source == "guided" {
			continue
		}
		stimuli[stimulus] = true
		if source != "llm" {
			nonLLM = true
		}
	}
	return len(stimuli) >= 2 && nonLLM, rows.Err()
}

func (s *Store) CanSkip(conceptID string) (bool, error) {
	return s.productionDemonstrated(conceptID)
}

func (s *Store) dimensionSatisfied(conceptID, dimension string) (bool, error) {
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
		if source != "guided" {
			return true, nil
		}
	}
	return false, rows.Err()
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

func (s *Store) EnsureCard(card schedule.Card) error {
	kind := card.Kind
	if kind == "" {
		kind = schedule.KindReview
	}
	_, err := s.db.Exec(
		`INSERT INTO reviews (step_id, concept_id, kind, ease, interval_days, reps, due_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(step_id) DO NOTHING`,
		card.StepID, card.ConceptID, kind, card.Ease, card.IntervalDays, card.Reps, card.Due.Unix(),
	)
	return err
}

func (s *Store) UpdateCard(card schedule.Card) error {
	kind := card.Kind
	if kind == "" {
		kind = schedule.KindReview
	}
	_, err := s.db.Exec(
		`UPDATE reviews SET concept_id = ?, kind = ?, ease = ?, interval_days = ?, reps = ?, due_at = ?
		 WHERE step_id = ?`,
		card.ConceptID, kind, card.Ease, card.IntervalDays, card.Reps, card.Due.Unix(), card.StepID,
	)
	return err
}

func (s *Store) Reviews() ([]schedule.Card, error) {
	return s.cardsWhere(`1 = 1 ORDER BY step_id`)
}

func (s *Store) DueCards(now time.Time, limit int) ([]schedule.Card, error) {
	return s.cardsWhere(fmt.Sprintf(`due_at <= %d ORDER BY due_at, step_id LIMIT %d`, now.Unix(), limit))
}

func (s *Store) cardsWhere(condition string) ([]schedule.Card, error) {
	rows, err := s.db.Query(
		`SELECT step_id, concept_id, ease, interval_days, reps, due_at, kind FROM reviews WHERE ` + condition)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []schedule.Card
	for rows.Next() {
		var card schedule.Card
		var due int64
		if err := rows.Scan(&card.StepID, &card.ConceptID, &card.Ease, &card.IntervalDays, &card.Reps, &due, &card.Kind); err != nil {
			return nil, err
		}
		card.Due = time.Unix(due, 0)
		out = append(out, card)
	}
	return out, rows.Err()
}

type Pending struct {
	ID       int64
	StepID   string
	Prompt   string
	Response string
	Retries  int
}

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

func (s *Store) Backup(dir string, keep int) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("jplings-%s.db", time.Now().UTC().Format("20060102-150405")))

	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("backup %s already exists", path)
	}
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
