package audit

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when no audit matches.
var ErrNotFound = errors.New("audit: not found")

// Store persists audits in the SEOnaut database.
type Store struct {
	DB *sql.DB
}

// NewToken returns a random URL-safe token of 43 characters (256 bits).
func NewToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Create inserts a queued audit and fills its ID and token.
func (s *Store) Create(a *Audit) error {
	now := time.Now().UTC()
	a.Token = NewToken()
	a.Status = StatusQueued
	a.Created, a.Updated = now, now
	res, err := s.DB.Exec(`
		INSERT INTO audits (token, url, email, lang, status, ip, created, updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Token, a.URL, a.Email, a.Lang, a.Status, a.IP, now, now)
	if err != nil {
		return err
	}
	a.ID, err = res.LastInsertId()
	return err
}

const selectAudit = `
	SELECT id, token, url, email, lang, ip, status, error,
		COALESCE(project_id, 0), COALESCE(crawl_id, 0), COALESCE(result, ''), created, updated
	FROM audits`

func scanAudit(row interface{ Scan(...any) error }) (*Audit, error) {
	a := &Audit{}
	var result string
	err := row.Scan(&a.ID, &a.Token, &a.URL, &a.Email, &a.Lang, &a.IP, &a.Status, &a.Error,
		&a.ProjectID, &a.CrawlID, &result, &a.Created, &a.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if result != "" {
		a.Result = &Result{}
		if err := json.Unmarshal([]byte(result), a.Result); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ByToken returns the audit with this token.
func (s *Store) ByToken(token string) (*Audit, error) {
	if len(token) != 43 {
		return nil, ErrNotFound
	}
	return scanAudit(s.DB.QueryRow(selectAudit+` WHERE token = ?`, token))
}

// NextQueued returns the oldest queued audit, or ErrNotFound.
func (s *Store) NextQueued() (*Audit, error) {
	return scanAudit(s.DB.QueryRow(selectAudit + ` WHERE status = 'queued' ORDER BY id LIMIT 1`))
}

// SetStatus records a step of the pipeline.
func (s *Store) SetStatus(a *Audit, st Status, errMsg string) error {
	a.Status, a.Error, a.Updated = st, errMsg, time.Now().UTC()
	if len(a.Error) > 1000 {
		a.Error = a.Error[:1000]
	}
	_, err := s.DB.Exec(`UPDATE audits SET status = ?, error = ?, updated = ? WHERE id = ?`,
		a.Status, a.Error, a.Updated, a.ID)
	return err
}

// SetCrawl links the audit to the project and crawl that measured the site.
func (s *Store) SetCrawl(a *Audit, projectID, crawlID int64) error {
	a.ProjectID, a.CrawlID = projectID, crawlID
	_, err := s.DB.Exec(`UPDATE audits SET project_id = ?, crawl_id = ?, updated = ? WHERE id = ?`,
		projectID, crawlID, time.Now().UTC(), a.ID)
	return err
}

// SaveResult stores the result and marks the audit done.
func (s *Store) SaveResult(a *Audit, r *Result) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	a.Result, a.Status, a.Error, a.Updated = r, StatusDone, "", time.Now().UTC()
	_, err = s.DB.Exec(`UPDATE audits SET result = ?, status = ?, error = '', updated = ? WHERE id = ?`,
		string(b), a.Status, a.Updated, a.ID)
	return err
}

// RequeueInterrupted puts back in the queue the audits a restart interrupted.
func (s *Store) RequeueInterrupted() error {
	_, err := s.DB.Exec(`UPDATE audits SET status = 'queued' WHERE status IN ('crawling', 'analysing')`)
	return err
}

// CountSince counts audits created since t, optionally for one email or host.
func (s *Store) CountSince(t time.Time, email, urlPrefix string) (int, error) {
	q := `SELECT COUNT(*) FROM audits WHERE created >= ?`
	args := []any{t.UTC()}
	if email != "" {
		q += ` AND email = ?`
		args = append(args, email)
	}
	if urlPrefix != "" {
		q += ` AND url LIKE ?`
		args = append(args, urlPrefix+"%")
	}
	var n int
	err := s.DB.QueryRow(q, args...).Scan(&n)
	return n, err
}

// LatestDoneForHost returns the most recent finished audit of a host created
// after t, so a repeated request reuses it instead of crawling again.
func (s *Store) LatestDoneForHost(host string, t time.Time) (*Audit, error) {
	return scanAudit(s.DB.QueryRow(selectAudit+`
		WHERE status = 'done' AND created >= ? AND (url LIKE ? OR url LIKE ?)
		ORDER BY id DESC LIMIT 1`, t.UTC(), "https://"+host+"/%", "http://"+host+"/%"))
}
