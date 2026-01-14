package calendar

import (
	"time"
)

// Event represents a calendar event
type Event struct {
	ID          string    `json:"id"`
	CalendarID  string    `json:"calendar_id"`
	UID         string    `json:"uid"` // iCal UID
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Color       string    `json:"color"`

	// Recurrence
	Recurrence *RecurrenceRule `json:"recurrence,omitempty"`

	// Reminders (minutes before event)
	Reminders []int `json:"reminders,omitempty"`

	// Metadata
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	ETag     string    `json:"etag"` // For sync

	// Status
	Status    EventStatus `json:"status"`
	Cancelled bool        `json:"cancelled"`
}

// EventStatus represents the status of an event
type EventStatus string

const (
	StatusConfirmed EventStatus = "confirmed"
	StatusTentative EventStatus = "tentative"
	StatusCancelled EventStatus = "cancelled"
)

// RecurrenceRule defines how an event repeats
type RecurrenceRule struct {
	Frequency  Frequency `json:"frequency"`
	Interval   int       `json:"interval"`   // Every N days/weeks/months/years
	Count      int       `json:"count"`      // Number of occurrences (0 = infinite)
	Until      time.Time `json:"until"`      // End date (zero = no end)
	ByDay      []Weekday `json:"by_day"`     // For weekly: which days
	ByMonthDay []int     `json:"by_monthday"` // For monthly: which days of month
	ByMonth    []int     `json:"by_month"`   // For yearly: which months
}

// Frequency represents recurrence frequency
type Frequency string

const (
	FrequencyDaily   Frequency = "daily"
	FrequencyWeekly  Frequency = "weekly"
	FrequencyMonthly Frequency = "monthly"
	FrequencyYearly  Frequency = "yearly"
)

// Weekday represents a day of the week
type Weekday string

const (
	Monday    Weekday = "MO"
	Tuesday   Weekday = "TU"
	Wednesday Weekday = "WE"
	Thursday  Weekday = "TH"
	Friday    Weekday = "FR"
	Saturday  Weekday = "SA"
	Sunday    Weekday = "SU"
)

// Calendar represents a calendar (container for events)
type Calendar struct {
	ID          string `json:"id"`
	AccountID   string `json:"account_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Visible     bool   `json:"visible"`
	ReadOnly    bool   `json:"read_only"`

	// Sync metadata
	SyncToken string    `json:"sync_token"`
	LastSync  time.Time `json:"last_sync"`
}

// Account represents a calendar account (Google, Apple, etc.)
type Account struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     AccountType `json:"type"`
	Email    string      `json:"email"`
	Enabled  bool        `json:"enabled"`

	// Connection details
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`

	// OAuth tokens (encrypted at rest)
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`

	// App password (for non-OAuth providers)
	AppPassword string `json:"app_password,omitempty"`

	LastSync time.Time `json:"last_sync"`
}

// AccountType represents the type of calendar account
type AccountType string

const (
	AccountTypeGoogle    AccountType = "google"
	AccountTypeApple     AccountType = "apple"
	AccountTypeOutlook   AccountType = "outlook"
	AccountTypeSamsung   AccountType = "samsung"
	AccountTypeCalDAV    AccountType = "caldav"
	AccountTypeLocal     AccountType = "local"
)

// Duration returns the duration of the event
func (e *Event) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// IsRecurring returns true if the event has a recurrence rule
func (e *Event) IsRecurring() bool {
	return e.Recurrence != nil
}

// Overlaps returns true if this event overlaps with another
func (e *Event) Overlaps(other *Event) bool {
	return e.Start.Before(other.End) && e.End.After(other.Start)
}

// ContainsTime returns true if the given time falls within the event
func (e *Event) ContainsTime(t time.Time) bool {
	return !t.Before(e.Start) && t.Before(e.End)
}

// IsOnDate returns true if the event occurs on the given date
func (e *Event) IsOnDate(date time.Time) bool {
	dateStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dateEnd := dateStart.Add(24 * time.Hour)
	return e.Start.Before(dateEnd) && e.End.After(dateStart)
}
