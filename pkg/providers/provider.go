package providers

import (
	"context"
	"time"

	"github.com/djwarf/switchcal/pkg/calendar"
)

// Provider defines the interface for calendar providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// Type returns the account type
	Type() calendar.AccountType

	// Authenticate authenticates with the provider
	Authenticate(ctx context.Context) error

	// ListCalendars returns all calendars from the provider
	ListCalendars(ctx context.Context) ([]*calendar.Calendar, error)

	// GetEvents returns events from a calendar within a time range
	GetEvents(ctx context.Context, calendarID string, start, end time.Time) ([]*calendar.Event, error)

	// CreateEvent creates a new event
	CreateEvent(ctx context.Context, calendarID string, event *calendar.Event) error

	// UpdateEvent updates an existing event
	UpdateEvent(ctx context.Context, calendarID string, event *calendar.Event) error

	// DeleteEvent deletes an event
	DeleteEvent(ctx context.Context, calendarID string, eventID string) error

	// Sync performs a full sync with the provider
	Sync(ctx context.Context) error

	// GetAccount returns the provider's account
	GetAccount() *calendar.Account

	// SetAccount sets the provider's account
	SetAccount(account *calendar.Account)
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	EventsCreated int
	EventsUpdated int
	EventsDeleted int
	Errors        []error
	SyncTime      time.Time
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	// OAuth
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string

	// CalDAV
	ServerURL string
	Username  string
	Password  string
}

// CalDAV server URLs for common providers
var CalDAVServers = map[calendar.AccountType]string{
	calendar.AccountTypeGoogle:  "https://apidata.googleusercontent.com/caldav/v2/",
	calendar.AccountTypeApple:   "https://caldav.icloud.com/",
	calendar.AccountTypeOutlook: "https://outlook.office365.com/caldav/",
	calendar.AccountTypeSamsung: "https://caldav.samsung.com/",
}

// OAuth configurations for common providers
var OAuthConfigs = map[calendar.AccountType]struct {
	AuthURL  string
	TokenURL string
	Scopes   []string
}{
	calendar.AccountTypeGoogle: {
		AuthURL:  "https://accounts.google.com/o/oauth2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
		Scopes:   []string{"https://www.googleapis.com/auth/calendar"},
	},
	calendar.AccountTypeOutlook: {
		AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:   []string{"https://outlook.office.com/Calendars.ReadWrite"},
	},
}
