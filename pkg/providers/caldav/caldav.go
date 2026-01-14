package caldav

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/djwarf/switchcal/pkg/calendar"
	"github.com/djwarf/switchcal/pkg/providers"
	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/google/uuid"
)

// OAuthHTTPClient wraps http.Client with OAuth Bearer token
type OAuthHTTPClient struct {
	token string
}

func (c *OAuthHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	return http.DefaultClient.Do(req)
}

// NewOAuthHTTPClient creates an HTTP client with OAuth Bearer auth
func NewOAuthHTTPClient(accessToken string) webdav.HTTPClient {
	return &OAuthHTTPClient{token: accessToken}
}

// Client implements the Provider interface for CalDAV servers
type Client struct {
	account      *calendar.Account
	httpClient   webdav.HTTPClient
	caldavClient *caldav.Client
}

// NewClient creates a new CalDAV client
func NewClient(account *calendar.Account) *Client {
	return &Client{
		account: account,
	}
}

// Name returns the provider name
func (c *Client) Name() string {
	return c.account.Name
}

// Type returns the account type
func (c *Client) Type() calendar.AccountType {
	return c.account.Type
}

// GetAccount returns the account
func (c *Client) GetAccount() *calendar.Account {
	return c.account
}

// SetAccount sets the account
func (c *Client) SetAccount(account *calendar.Account) {
	c.account = account
}

// Authenticate authenticates with the CalDAV server
func (c *Client) Authenticate(ctx context.Context) error {
	var httpClient webdav.HTTPClient

	// Use OAuth for Google accounts, Basic auth otherwise
	if c.account.Type == calendar.AccountTypeGoogle && c.account.AccessToken != "" {
		httpClient = NewOAuthHTTPClient(c.account.AccessToken)
	} else {
		httpClient = webdav.HTTPClientWithBasicAuth(nil, c.account.Username, c.account.AppPassword)
	}
	c.httpClient = httpClient

	client, err := caldav.NewClient(httpClient, c.account.ServerURL)
	if err != nil {
		return fmt.Errorf("failed to create CalDAV client: %w", err)
	}
	c.caldavClient = client

	// Skip home set discovery for Google (uses direct URL pattern)
	if c.account.Type == calendar.AccountTypeGoogle {
		return nil
	}

	// Test connection by finding the calendar home
	_, err = client.FindCalendarHomeSet(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	return nil
}

// ListCalendars returns all calendars from the CalDAV server
func (c *Client) ListCalendars(ctx context.Context) ([]*calendar.Calendar, error) {
	if c.caldavClient == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	// For Google, use the direct URL pattern
	if c.account.Type == calendar.AccountTypeGoogle {
		// Google CalDAV returns calendars at the server URL directly
		cals, err := c.caldavClient.FindCalendars(ctx, c.account.ServerURL)
		if err != nil {
			return nil, fmt.Errorf("failed to list Google calendars: %w", err)
		}

		var calendars []*calendar.Calendar
		for _, cal := range cals {
			name := cal.Name
			if name == "" {
				name = "Primary Calendar"
			}
			calendars = append(calendars, &calendar.Calendar{
				ID:          cal.Path,
				AccountID:   c.account.ID,
				Name:        name,
				Description: cal.Description,
				Color:       "#4285f4",
				Visible:     true,
				ReadOnly:    false,
			})
		}
		return calendars, nil
	}

	// Standard CalDAV discovery
	homeSet, err := c.caldavClient.FindCalendarHomeSet(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to find calendar home: %w", err)
	}

	cals, err := c.caldavClient.FindCalendars(ctx, homeSet)
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	var calendars []*calendar.Calendar
	for _, cal := range cals {
		calendars = append(calendars, &calendar.Calendar{
			ID:          cal.Path,
			AccountID:   c.account.ID,
			Name:        cal.Name,
			Description: cal.Description,
			Color:       "#4285f4", // Default color
			Visible:     true,
			ReadOnly:    false,
		})
	}

	return calendars, nil
}

// GetEvents returns events from a calendar within a time range
func (c *Client) GetEvents(ctx context.Context, calendarID string, start, end time.Time) ([]*calendar.Event, error) {
	if c.caldavClient == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	query := &caldav.CalendarQuery{
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name:  "VEVENT",
				Start: start,
				End:   end,
			}},
		},
	}

	objects, err := c.caldavClient.QueryCalendar(ctx, calendarID, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	var events []*calendar.Event
	for _, obj := range objects {
		event, err := parseICalEvent(&obj, calendarID)
		if err != nil {
			continue // Skip invalid events
		}
		events = append(events, event)
	}

	return events, nil
}

// CreateEvent creates a new event on the CalDAV server
func (c *Client) CreateEvent(ctx context.Context, calendarID string, event *calendar.Event) error {
	if c.caldavClient == nil {
		return fmt.Errorf("not authenticated")
	}

	// Generate UID if not set
	if event.UID == "" {
		event.UID = uuid.New().String()
	}

	icalData := eventToICal(event)
	path := fmt.Sprintf("%s/%s.ics", calendarID, event.UID)

	_, err := c.caldavClient.PutCalendarObject(ctx, path, icalData)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

// UpdateEvent updates an existing event
func (c *Client) UpdateEvent(ctx context.Context, calendarID string, event *calendar.Event) error {
	if c.caldavClient == nil {
		return fmt.Errorf("not authenticated")
	}

	icalData := eventToICal(event)
	path := fmt.Sprintf("%s/%s.ics", calendarID, event.UID)

	_, err := c.caldavClient.PutCalendarObject(ctx, path, icalData)
	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

// DeleteEvent deletes an event from the CalDAV server
func (c *Client) DeleteEvent(ctx context.Context, calendarID string, eventID string) error {
	if c.caldavClient == nil {
		return fmt.Errorf("not authenticated")
	}

	path := fmt.Sprintf("%s/%s.ics", calendarID, eventID)
	err := c.caldavClient.RemoveAll(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// Sync performs a full sync (placeholder for now)
func (c *Client) Sync(ctx context.Context) error {
	c.account.LastSync = time.Now()
	return nil
}

// parseICalEvent parses a CalDAV object into an Event
func parseICalEvent(obj *caldav.CalendarObject, calendarID string) (*calendar.Event, error) {
	if obj.Data == nil {
		return nil, fmt.Errorf("no data in calendar object")
	}

	for _, component := range obj.Data.Children {
		if component.Name != ical.CompEvent {
			continue
		}

		event := &calendar.Event{
			CalendarID: calendarID,
			ETag:       obj.ETag,
			Status:     calendar.StatusConfirmed,
		}

		// Get UID
		if prop := component.Props.Get(ical.PropUID); prop != nil {
			event.UID = prop.Value
			event.ID = prop.Value
		}

		// Get summary/title
		if prop := component.Props.Get(ical.PropSummary); prop != nil {
			event.Title = prop.Value
		}

		// Get description
		if prop := component.Props.Get(ical.PropDescription); prop != nil {
			event.Description = prop.Value
		}

		// Get location
		if prop := component.Props.Get(ical.PropLocation); prop != nil {
			event.Location = prop.Value
		}

		// Get start time
		if prop := component.Props.Get(ical.PropDateTimeStart); prop != nil {
			if t, err := prop.DateTime(nil); err == nil {
				event.Start = t
			}
			// Check if all-day event
			if val := prop.Params.Get("VALUE"); val == "DATE" {
				event.AllDay = true
			}
		}

		// Get end time
		if prop := component.Props.Get(ical.PropDateTimeEnd); prop != nil {
			if t, err := prop.DateTime(nil); err == nil {
				event.End = t
			}
		}

		// Get created time
		if prop := component.Props.Get(ical.PropCreated); prop != nil {
			if t, err := prop.DateTime(nil); err == nil {
				event.Created = t
			}
		}

		// Get last modified time
		if prop := component.Props.Get(ical.PropLastModified); prop != nil {
			if t, err := prop.DateTime(nil); err == nil {
				event.Modified = t
			}
		}

		// Get status
		if prop := component.Props.Get(ical.PropStatus); prop != nil {
			switch prop.Value {
			case "CONFIRMED":
				event.Status = calendar.StatusConfirmed
			case "TENTATIVE":
				event.Status = calendar.StatusTentative
			case "CANCELLED":
				event.Status = calendar.StatusCancelled
				event.Cancelled = true
			}
		}

		return event, nil
	}

	return nil, fmt.Errorf("no VEVENT found")
}

// eventToICal converts an Event to iCal format
func eventToICal(event *calendar.Event) *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//UniCal//EN")

	vevent := ical.NewComponent(ical.CompEvent)
	vevent.Props.SetText(ical.PropUID, event.UID)
	vevent.Props.SetText(ical.PropSummary, event.Title)

	if event.Description != "" {
		vevent.Props.SetText(ical.PropDescription, event.Description)
	}
	if event.Location != "" {
		vevent.Props.SetText(ical.PropLocation, event.Location)
	}

	if event.AllDay {
		vevent.Props.SetDate(ical.PropDateTimeStart, event.Start)
		vevent.Props.SetDate(ical.PropDateTimeEnd, event.End)
	} else {
		vevent.Props.SetDateTime(ical.PropDateTimeStart, event.Start)
		vevent.Props.SetDateTime(ical.PropDateTimeEnd, event.End)
	}

	vevent.Props.SetDateTime(ical.PropDateTimeStamp, time.Now())

	switch event.Status {
	case calendar.StatusConfirmed:
		vevent.Props.SetText(ical.PropStatus, "CONFIRMED")
	case calendar.StatusTentative:
		vevent.Props.SetText(ical.PropStatus, "TENTATIVE")
	case calendar.StatusCancelled:
		vevent.Props.SetText(ical.PropStatus, "CANCELLED")
	}

	cal.Children = append(cal.Children, vevent)
	return cal
}

// Ensure Client implements Provider
var _ providers.Provider = (*Client)(nil)
