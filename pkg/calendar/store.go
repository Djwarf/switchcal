package calendar

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages calendar data persistence
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// NewStore creates a new calendar store
func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Use DELETE journal mode for immediate writes (no WAL)
	connStr := dbPath + "?_foreign_keys=on&_journal_mode=DELETE&_synchronous=FULL"
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Force single connection to avoid pooling issues
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Force connection and set pragmas
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs database migrations
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		email TEXT,
		enabled INTEGER DEFAULT 1,
		server_url TEXT,
		username TEXT,
		access_token TEXT,
		refresh_token TEXT,
		token_expiry DATETIME,
		app_password TEXT,
		last_sync DATETIME
	);

	CREATE TABLE IF NOT EXISTS calendars (
		id TEXT PRIMARY KEY,
		account_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		color TEXT DEFAULT '#4285f4',
		visible INTEGER DEFAULT 1,
		read_only INTEGER DEFAULT 0,
		sync_token TEXT,
		last_sync DATETIME,
		FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		calendar_id TEXT NOT NULL,
		uid TEXT,
		title TEXT NOT NULL,
		description TEXT,
		location TEXT,
		start_time DATETIME NOT NULL,
		end_time DATETIME NOT NULL,
		all_day INTEGER DEFAULT 0,
		color TEXT,
		recurrence TEXT,
		reminders TEXT,
		created DATETIME,
		modified DATETIME,
		etag TEXT,
		status TEXT DEFAULT 'confirmed',
		cancelled INTEGER DEFAULT 0,
		FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_events_calendar ON events(calendar_id);
	CREATE INDEX IF NOT EXISTS idx_events_start ON events(start_time);
	CREATE INDEX IF NOT EXISTS idx_events_end ON events(end_time);
	CREATE INDEX IF NOT EXISTS idx_calendars_account ON calendars(account_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// --- Account Operations ---

// SaveAccount saves an account to the database
func (s *Store) SaveAccount(a *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use ON CONFLICT DO UPDATE instead of REPLACE to avoid triggering CASCADE deletes
	_, err := s.db.Exec(`
		INSERT INTO accounts (id, name, type, email, enabled, server_url, username, access_token, refresh_token, token_expiry, app_password, last_sync)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			email = excluded.email,
			enabled = excluded.enabled,
			server_url = excluded.server_url,
			username = excluded.username,
			access_token = excluded.access_token,
			refresh_token = excluded.refresh_token,
			token_expiry = excluded.token_expiry,
			app_password = excluded.app_password,
			last_sync = excluded.last_sync`,
		a.ID, a.Name, a.Type, a.Email, a.Enabled, a.ServerURL, a.Username,
		a.AccessToken, a.RefreshToken, a.TokenExpiry, a.AppPassword, a.LastSync)
	return err
}

// GetAccount retrieves an account by ID
func (s *Store) GetAccount(id string) (*Account, error) {
	row := s.db.QueryRow(`SELECT * FROM accounts WHERE id = ?`, id)
	return scanAccount(row)
}

// GetAllAccounts retrieves all accounts
func (s *Store) GetAllAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`SELECT * FROM accounts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a, err := scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// DeleteAccount deletes an account and its calendars/events
func (s *Store) DeleteAccount(id string) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	return err
}

// --- Calendar Operations ---

// SaveCalendar saves a calendar to the database
func (s *Store) SaveCalendar(c *Calendar) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use ON CONFLICT DO UPDATE instead of REPLACE to avoid triggering CASCADE deletes
	_, err := s.db.Exec(`
		INSERT INTO calendars (id, account_id, name, description, color, visible, read_only, sync_token, last_sync)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			account_id = excluded.account_id,
			name = excluded.name,
			description = excluded.description,
			color = excluded.color,
			visible = excluded.visible,
			read_only = excluded.read_only,
			sync_token = excluded.sync_token,
			last_sync = excluded.last_sync`,
		c.ID, c.AccountID, c.Name, c.Description, c.Color, c.Visible, c.ReadOnly, c.SyncToken, c.LastSync)
	return err
}

// GetCalendar retrieves a calendar by ID
func (s *Store) GetCalendar(id string) (*Calendar, error) {
	row := s.db.QueryRow(`SELECT * FROM calendars WHERE id = ?`, id)
	return scanCalendar(row)
}

// GetCalendarsByAccount retrieves all calendars for an account
func (s *Store) GetCalendarsByAccount(accountID string) ([]*Calendar, error) {
	rows, err := s.db.Query(`SELECT * FROM calendars WHERE account_id = ? ORDER BY name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calendars []*Calendar
	for rows.Next() {
		c, err := scanCalendarRows(rows)
		if err != nil {
			return nil, err
		}
		calendars = append(calendars, c)
	}
	return calendars, rows.Err()
}

// GetAllCalendars retrieves all calendars
func (s *Store) GetAllCalendars() ([]*Calendar, error) {
	rows, err := s.db.Query(`SELECT * FROM calendars ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calendars []*Calendar
	for rows.Next() {
		c, err := scanCalendarRows(rows)
		if err != nil {
			return nil, err
		}
		calendars = append(calendars, c)
	}
	return calendars, rows.Err()
}

// GetVisibleCalendars retrieves all visible calendars
func (s *Store) GetVisibleCalendars() ([]*Calendar, error) {
	rows, err := s.db.Query(`SELECT * FROM calendars WHERE visible = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calendars []*Calendar
	for rows.Next() {
		c, err := scanCalendarRows(rows)
		if err != nil {
			return nil, err
		}
		calendars = append(calendars, c)
	}
	return calendars, rows.Err()
}

// DeleteCalendar deletes a calendar and its events
func (s *Store) DeleteCalendar(id string) error {
	_, err := s.db.Exec(`DELETE FROM calendars WHERE id = ?`, id)
	return err
}

// --- Event Operations ---

// SaveEvent saves an event to the database
func (s *Store) SaveEvent(e *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recurrence, _ := json.Marshal(e.Recurrence)
	reminders, _ := json.Marshal(e.Reminders)

	// Use ON CONFLICT DO UPDATE instead of REPLACE to avoid issues with foreign keys
	_, err := s.db.Exec(`
		INSERT INTO events (id, calendar_id, uid, title, description, location, start_time, end_time, all_day, color, recurrence, reminders, created, modified, etag, status, cancelled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			calendar_id = excluded.calendar_id,
			uid = excluded.uid,
			title = excluded.title,
			description = excluded.description,
			location = excluded.location,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			all_day = excluded.all_day,
			color = excluded.color,
			recurrence = excluded.recurrence,
			reminders = excluded.reminders,
			created = excluded.created,
			modified = excluded.modified,
			etag = excluded.etag,
			status = excluded.status,
			cancelled = excluded.cancelled`,
		e.ID, e.CalendarID, e.UID, e.Title, e.Description, e.Location,
		e.Start, e.End, e.AllDay, e.Color, string(recurrence), string(reminders),
		e.Created, e.Modified, e.ETag, e.Status, e.Cancelled)
	return err
}

// GetEvent retrieves an event by ID
func (s *Store) GetEvent(id string) (*Event, error) {
	row := s.db.QueryRow(`SELECT * FROM events WHERE id = ?`, id)
	return scanEvent(row)
}

// GetEventsByCalendar retrieves all events for a calendar
func (s *Store) GetEventsByCalendar(calendarID string) ([]*Event, error) {
	rows, err := s.db.Query(`SELECT * FROM events WHERE calendar_id = ? AND cancelled = 0 ORDER BY start_time`, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetEventsInRange retrieves all events within a time range
func (s *Store) GetEventsInRange(start, end time.Time) ([]*Event, error) {
	rows, err := s.db.Query(`
		SELECT e.* FROM events e
		JOIN calendars c ON e.calendar_id = c.id
		WHERE c.visible = 1 AND e.cancelled = 0
		AND e.start_time < ? AND e.end_time > ?
		ORDER BY e.start_time`,
		end, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetEventsForDate retrieves all events for a specific date
func (s *Store) GetEventsForDate(date time.Time) ([]*Event, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)
	return s.GetEventsInRange(start, end)
}

// DeleteEvent deletes an event
func (s *Store) DeleteEvent(id string) error {
	_, err := s.db.Exec(`DELETE FROM events WHERE id = ?`, id)
	return err
}

// DeleteEventsByCalendar deletes all events for a calendar
func (s *Store) DeleteEventsByCalendar(calendarID string) error {
	_, err := s.db.Exec(`DELETE FROM events WHERE calendar_id = ?`, calendarID)
	return err
}

// --- Scan helpers ---

func scanAccount(row *sql.Row) (*Account, error) {
	a := &Account{}
	var email, serverURL, username, accessToken, refreshToken, appPassword sql.NullString
	var tokenExpiry, lastSync sql.NullTime
	err := row.Scan(&a.ID, &a.Name, &a.Type, &email, &a.Enabled, &serverURL, &username,
		&accessToken, &refreshToken, &tokenExpiry, &appPassword, &lastSync)
	if err != nil {
		return nil, err
	}
	a.Email = email.String
	a.ServerURL = serverURL.String
	a.Username = username.String
	a.AccessToken = accessToken.String
	a.RefreshToken = refreshToken.String
	a.AppPassword = appPassword.String
	if tokenExpiry.Valid {
		a.TokenExpiry = tokenExpiry.Time
	}
	if lastSync.Valid {
		a.LastSync = lastSync.Time
	}
	return a, nil
}

func scanAccountRows(rows *sql.Rows) (*Account, error) {
	a := &Account{}
	var email, serverURL, username, accessToken, refreshToken, appPassword sql.NullString
	var tokenExpiry, lastSync sql.NullTime
	err := rows.Scan(&a.ID, &a.Name, &a.Type, &email, &a.Enabled, &serverURL, &username,
		&accessToken, &refreshToken, &tokenExpiry, &appPassword, &lastSync)
	if err != nil {
		return nil, err
	}
	a.Email = email.String
	a.ServerURL = serverURL.String
	a.Username = username.String
	a.AccessToken = accessToken.String
	a.RefreshToken = refreshToken.String
	a.AppPassword = appPassword.String
	if tokenExpiry.Valid {
		a.TokenExpiry = tokenExpiry.Time
	}
	if lastSync.Valid {
		a.LastSync = lastSync.Time
	}
	return a, nil
}

func scanCalendar(row *sql.Row) (*Calendar, error) {
	c := &Calendar{}
	var description, color, syncToken sql.NullString
	var lastSync sql.NullTime
	err := row.Scan(&c.ID, &c.AccountID, &c.Name, &description, &color, &c.Visible, &c.ReadOnly, &syncToken, &lastSync)
	if err != nil {
		return nil, err
	}
	c.Description = description.String
	c.Color = color.String
	c.SyncToken = syncToken.String
	if lastSync.Valid {
		c.LastSync = lastSync.Time
	}
	if c.Color == "" {
		c.Color = "#4285f4"
	}
	return c, nil
}

func scanCalendarRows(rows *sql.Rows) (*Calendar, error) {
	c := &Calendar{}
	var description, color, syncToken sql.NullString
	var lastSync sql.NullTime
	err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &description, &color, &c.Visible, &c.ReadOnly, &syncToken, &lastSync)
	if err != nil {
		return nil, err
	}
	c.Description = description.String
	c.Color = color.String
	c.SyncToken = syncToken.String
	if lastSync.Valid {
		c.LastSync = lastSync.Time
	}
	if c.Color == "" {
		c.Color = "#4285f4"
	}
	return c, nil
}

func scanEvent(row *sql.Row) (*Event, error) {
	e := &Event{}
	var uid, description, location, color, recurrence, reminders, etag, status sql.NullString
	var created, modified sql.NullTime
	err := row.Scan(&e.ID, &e.CalendarID, &uid, &e.Title, &description, &location,
		&e.Start, &e.End, &e.AllDay, &color, &recurrence, &reminders,
		&created, &modified, &etag, &status, &e.Cancelled)
	if err != nil {
		return nil, err
	}

	e.UID = uid.String
	e.Description = description.String
	e.Location = location.String
	e.Color = color.String
	e.ETag = etag.String
	e.Status = EventStatus(status.String)
	if e.Status == "" {
		e.Status = StatusConfirmed
	}
	if created.Valid {
		e.Created = created.Time
	}
	if modified.Valid {
		e.Modified = modified.Time
	}

	if recurrence.String != "" && recurrence.String != "null" {
		json.Unmarshal([]byte(recurrence.String), &e.Recurrence)
	}
	if reminders.String != "" && reminders.String != "null" {
		json.Unmarshal([]byte(reminders.String), &e.Reminders)
	}

	return e, nil
}

func scanEvents(rows *sql.Rows) ([]*Event, error) {
	var events []*Event
	for rows.Next() {
		e := &Event{}
		var uid, description, location, color, recurrence, reminders, etag, status sql.NullString
		var created, modified sql.NullTime
		err := rows.Scan(&e.ID, &e.CalendarID, &uid, &e.Title, &description, &location,
			&e.Start, &e.End, &e.AllDay, &color, &recurrence, &reminders,
			&created, &modified, &etag, &status, &e.Cancelled)
		if err != nil {
			return nil, err
		}

		e.UID = uid.String
		e.Description = description.String
		e.Location = location.String
		e.Color = color.String
		e.ETag = etag.String
		e.Status = EventStatus(status.String)
		if e.Status == "" {
			e.Status = StatusConfirmed
		}
		if created.Valid {
			e.Created = created.Time
		}
		if modified.Valid {
			e.Modified = modified.Time
		}

		if recurrence.String != "" && recurrence.String != "null" {
			json.Unmarshal([]byte(recurrence.String), &e.Recurrence)
		}
		if reminders.String != "" && reminders.String != "null" {
			json.Unmarshal([]byte(reminders.String), &e.Reminders)
		}

		events = append(events, e)
	}
	return events, rows.Err()
}
