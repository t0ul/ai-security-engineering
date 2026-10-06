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
package cpstore

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed inventory.
type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS prompts(name TEXT, version TEXT, hash TEXT, text TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS approvals(id TEXT, proposer TEXT, approver TEXT, perm TEXT, note TEXT, version INTEGER, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS mcp_pins(server TEXT, hash TEXT, approved_by TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS eval_scores(label TEXT, f1 REAL, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS admin_audit(actor TEXT, action TEXT, detail TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);`

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
