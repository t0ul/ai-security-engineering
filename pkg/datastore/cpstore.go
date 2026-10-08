// Package cpstore is the control-plane console's durable inventory: an embedded
// SQLite store (pure-Go, no CGO) that persists the governed state the operator
// console shows across restarts — versioned prompts, four-eyes approvals, MCP
// manifest pins, eval scores, and the admin-action audit. The fleet libraries
// (goverlord, gustoms) stay pure/in-memory; the course records their decisions
// here as they happen, so history survives a restart and the console reads real
// data, not a mock.
//
// Tables are append-only history (an inventory of versioned artifacts and the
// decisions about them), distinct from gledger's runtime trace log.
package datastore

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed inventory.
type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS prompts(name TEXT, version TEXT, hash TEXT, text TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS grammars(name TEXT, version TEXT, hash TEXT, text TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS models(name TEXT, version TEXT, hash TEXT, model TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS approvals(id TEXT, proposer TEXT, approver TEXT, perm TEXT, note TEXT, version INTEGER, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS mcp_pins(server TEXT, hash TEXT, approved_by TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS eval_scores(label TEXT, f1 REAL, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS admin_audit(actor TEXT, action TEXT, detail TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS policies(name TEXT, version TEXT, hash TEXT, items TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS config(key TEXT PRIMARY KEY, value TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS sampling(name TEXT, version TEXT, hash TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS retrieval(name TEXT, version TEXT, hash TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS bundles(label TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS budgets(name TEXT, version TEXT, hash TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS feedback(source TEXT, title TEXT, decision TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS skill_pins(name TEXT, version TEXT, hash TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);`

// Open creates/opens the inventory at path (":memory:" for ephemeral).
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// RecordPrompt stores a versioned, hashed system-prompt artifact.
func (s *Store) RecordPrompt(name, version, hash, text string) error {
	_, err := s.db.Exec(`INSERT INTO prompts(name,version,hash,text) VALUES(?,?,?,?)`, name, version, hash, text)
	return err
}

// SetConfig upserts a single configuration value by key (JSON or scalar). The DB
// is the source of truth for all runtime config; shipped consts are only the
// fail-closed default when a key is absent.
func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO config(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, at=CURRENT_TIMESTAMP`, key, value)
	return err
}

// GetConfig reads a configuration value by key.
func (s *Store) GetConfig(key string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT value FROM config WHERE key=?`, key)
	var v string
	switch err := row.Scan(&v); err {
	case nil:
		return v, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, err
	}
}

// RecordFeedback stores an operator accept/reject decision on an extracted item —
// the data-flywheel's ground-truth signal: real use teaches the system which
// extractions were right (M19 data governance / the improvement loop).
func (s *Store) RecordFeedback(source, title, decision string) error {
	_, err := s.db.Exec(`INSERT INTO feedback(source,title,decision) VALUES(?,?,?)`, source, title, decision)
	return err
}

// FeedbackStats returns the accept/reject tallies accumulated from real use.
func (s *Store) FeedbackStats() (accepts, rejects int, err error) {
	row := s.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN decision='accept' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN decision='reject' THEN 1 ELSE 0 END),0) FROM feedback`)
	err = row.Scan(&accepts, &rejects)
	return accepts, rejects, err
}

// RecordBudget stores a versioned, hashed budget config (JSON) for a name.
func (s *Store) RecordBudget(name, version, hash, config string) error {
	_, err := s.db.Exec(`INSERT INTO budgets(name,version,hash,config) VALUES(?,?,?,?)`, name, version, hash, config)
	return err
}

// LatestBudget returns the most recent persisted budget config JSON for name, so
// the resolver can rehydrate on boot.
func (s *Store) LatestBudget(name string) (config string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT config FROM budgets WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&config); e {
	case nil:
		return config, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// SaveBundle stores a known-good configuration snapshot (JSON) under a label — a
// point the whole governed plane can be rolled back to in one step (C10).
func (s *Store) SaveBundle(label, config string) error {
	_, err := s.db.Exec(`INSERT INTO bundles(label,config) VALUES(?,?)`, label, config)
	return err
}

// GetBundle returns the most recent snapshot stored under label.
func (s *Store) GetBundle(label string) (config string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT config FROM bundles WHERE label=? ORDER BY at DESC, rowid DESC LIMIT 1`, label)
	switch e := row.Scan(&config); e {
	case nil:
		return config, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// BundleRow is a stored snapshot's label + time (config omitted from the list).
type BundleRow struct {
	Label string
	At    time.Time
}

// ListBundles returns the most-recent snapshot per label, newest first. It selects
// the real `at` column (not MAX(at), which some drivers return as a string that
// won't scan into time.Time) via a per-label latest-row subquery.
func (s *Store) ListBundles(limit int) ([]BundleRow, error) {
	rows, err := s.db.Query(`SELECT label, at FROM bundles b WHERE at = (SELECT MAX(at) FROM bundles WHERE label = b.label) GROUP BY label ORDER BY at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BundleRow
	for rows.Next() {
		var b BundleRow
		if err := rows.Scan(&b.Label, &b.At); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// RecordSampling stores a versioned, hashed sampling config (JSON) for a model.
func (s *Store) RecordSampling(name, version, hash, config string) error {
	_, err := s.db.Exec(`INSERT INTO sampling(name,version,hash,config) VALUES(?,?,?,?)`, name, version, hash, config)
	return err
}

// LatestSampling returns the most recent persisted sampling config JSON for name,
// so the resolver can rehydrate the active sampling on boot.
func (s *Store) LatestSampling(name string) (config string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT config FROM sampling WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&config); e {
	case nil:
		return config, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// RecordRetrieval persists a governed retrieval-config activation.
func (s *Store) RecordRetrieval(name, version, hash, config string) error {
	_, err := s.db.Exec(`INSERT INTO retrieval(name,version,hash,config) VALUES(?,?,?,?)`, name, version, hash, config)
	return err
}

// LatestRetrieval returns the most recent persisted retrieval config JSON for
// name, so the resolver can rehydrate the active retrieval on boot.
func (s *Store) LatestRetrieval(name string) (config string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT config FROM retrieval WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&config); e {
	case nil:
		return config, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// LatestPromptText returns the most recent persisted prompt text for name, so the
// resolver can rehydrate the active prompt on boot (the DB is the source of truth).
func (s *Store) LatestPromptText(name string) (text string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT text FROM prompts WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&text); e {
	case nil:
		return text, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// RecordGrammar persists a governed output-grammar activation.
func (s *Store) RecordGrammar(name, version, hash, text string) error {
	_, err := s.db.Exec(`INSERT INTO grammars(name,version,hash,text) VALUES(?,?,?,?)`, name, version, hash, text)
	return err
}

// LatestGrammarText returns the most recent persisted grammar text for name, so
// the resolver can rehydrate the active grammar on boot.
func (s *Store) LatestGrammarText(name string) (text string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT text FROM grammars WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&text); e {
	case nil:
		return text, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// RecordModel persists a governed model-binding activation.
func (s *Store) RecordModel(name, version, hash, model string) error {
	_, err := s.db.Exec(`INSERT INTO models(name,version,hash,model) VALUES(?,?,?,?)`, name, version, hash, model)
	return err
}

// LatestModel returns the most recent persisted model name for a role, so the
// resolver can rehydrate the active binding on boot.
func (s *Store) LatestModel(name string) (model string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT model FROM models WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&model); e {
	case nil:
		return model, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// RecordSkillPin persists a governed skill approval: the operator-approved content
// hash (the pin) for a skill name. The approved hash IS the pin — no hash-of-a-hash.
func (s *Store) RecordSkillPin(name, version, hash string) error {
	_, err := s.db.Exec(`INSERT INTO skill_pins(name,version,hash) VALUES(?,?,?)`, name, version, hash)
	return err
}

// LatestSkillPin returns the most recent approved content hash for a skill name, so
// the resolver can rehydrate the active pin on boot.
func (s *Store) LatestSkillPin(name string) (hash string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT hash FROM skill_pins WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	switch e := row.Scan(&hash); e {
	case nil:
		return hash, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

// LatestPolicyItems returns the most recent persisted allowlist for name (stored
// newline-joined), so the resolver can rehydrate an active policy on boot.
func (s *Store) LatestPolicyItems(name string) (items []string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT items FROM policies WHERE name=? ORDER BY at DESC, rowid DESC LIMIT 1`, name)
	var joined string
	switch e := row.Scan(&joined); e {
	case nil:
		for _, it := range strings.Split(joined, "\n") {
			if it != "" {
				items = append(items, it)
			}
		}
		return items, true, nil
	case sql.ErrNoRows:
		return nil, false, nil
	default:
		return nil, false, e
	}
}

// RecordPolicy stores a versioned, hashed policy-allowlist artifact (items is the
// newline-joined allowlist: egress hosts, exec argv[0]s, guardrail topics, ...).
func (s *Store) RecordPolicy(name, version, hash, items string) error {
	_, err := s.db.Exec(`INSERT INTO policies(name,version,hash,items) VALUES(?,?,?,?)`, name, version, hash, items)
	return err
}

// RecordApproval stores a committed four-eyes decision.
func (s *Store) RecordApproval(id, proposer, approver, perm, note string, version int) error {
	_, err := s.db.Exec(`INSERT INTO approvals(id,proposer,approver,perm,note,version) VALUES(?,?,?,?,?,?)`,
		id, proposer, approver, perm, note, version)
	return err
}

// RecordPin stores an approved MCP manifest pin.
func (s *Store) RecordPin(server, hash, approvedBy string) error {
	_, err := s.db.Exec(`INSERT INTO mcp_pins(server,hash,approved_by) VALUES(?,?,?)`, server, hash, approvedBy)
	return err
}

// RecordEval stores an eval score (feeds the MLOps promotion gate).
func (s *Store) RecordEval(label string, f1 float64) error {
	_, err := s.db.Exec(`INSERT INTO eval_scores(label,f1) VALUES(?,?)`, label, f1)
	return err
}

// RecordAdmin stores an operator-action audit row.
func (s *Store) RecordAdmin(actor, action, detail string) error {
	_, err := s.db.Exec(`INSERT INTO admin_audit(actor,action,detail) VALUES(?,?,?)`, actor, action, detail)
	return err
}

// EvalScore is one recorded score.
type EvalScore struct {
	Label string
	F1    float64
	At    time.Time
}

// LatestEval returns the most recent score for a label. A promotion gate can
// read this so "ship to prod" is backed by a persisted, auditable score.
func (s *Store) LatestEval(label string) (EvalScore, bool, error) {
	row := s.db.QueryRow(`SELECT label,f1,at FROM eval_scores WHERE label=? ORDER BY at DESC, rowid DESC LIMIT 1`, label)
	var e EvalScore
	switch err := row.Scan(&e.Label, &e.F1, &e.At); err {
	case nil:
		return e, true, nil
	case sql.ErrNoRows:
		return EvalScore{}, false, nil
	default:
		return EvalScore{}, false, err
	}
}

// ListEvals returns recent eval scores (newest first) — the F1 trend for the
// console's Eval card.
func (s *Store) ListEvals(limit int) ([]EvalScore, error) {
	rows, err := s.db.Query(`SELECT label,f1,at FROM eval_scores ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EvalScore
	for rows.Next() {
		var e EvalScore
		if err := rows.Scan(&e.Label, &e.F1, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PromptRow is a recorded prompt activation (text omitted from the list view).
type PromptRow struct {
	Name    string
	Version string
	Hash    string
	At      time.Time
}

// ListPrompts returns recent prompt activations, newest first.
func (s *Store) ListPrompts(limit int) ([]PromptRow, error) {
	rows, err := s.db.Query(`SELECT name,version,hash,at FROM prompts ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PromptRow
	for rows.Next() {
		var p PromptRow
		if err := rows.Scan(&p.Name, &p.Version, &p.Hash, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PolicyRow is a recorded policy activation (items omitted from the list view).
type PolicyRow struct {
	Name    string
	Version string
	Hash    string
	At      time.Time
}

// ListPolicies returns recent policy activations, newest first.
func (s *Store) ListPolicies(limit int) ([]PolicyRow, error) {
	rows, err := s.db.Query(`SELECT name,version,hash,at FROM policies ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PolicyRow
	for rows.Next() {
		var p PolicyRow
		if err := rows.Scan(&p.Name, &p.Version, &p.Hash, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PinRow is a recorded MCP manifest pin.
type PinRow struct {
	Server     string
	Hash       string
	ApprovedBy string
	At         time.Time
}

// ListPins returns recent MCP manifest pins, newest first.
func (s *Store) ListPins(limit int) ([]PinRow, error) {
	rows, err := s.db.Query(`SELECT server,hash,approved_by,at FROM mcp_pins ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PinRow
	for rows.Next() {
		var p PinRow
		if err := rows.Scan(&p.Server, &p.Hash, &p.ApprovedBy, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Approval is one recorded decision.
type Approval struct {
	ID       string
	Proposer string
	Approver string
	Perm     string
	Note     string
	Version  int
	At       time.Time
}

// ListApprovals returns recent approvals, newest first.
func (s *Store) ListApprovals(limit int) ([]Approval, error) {
	rows, err := s.db.Query(`SELECT id,proposer,approver,perm,note,version,at FROM approvals ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		var a Approval
		if err := rows.Scan(&a.ID, &a.Proposer, &a.Approver, &a.Perm, &a.Note, &a.Version, &a.At); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
