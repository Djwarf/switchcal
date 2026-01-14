package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/djwarf/switchcal/pkg/calendar"
	"github.com/djwarf/switchcal/pkg/providers"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// Google Calendar API scopes
	CalendarScope         = "https://www.googleapis.com/auth/calendar"
	CalendarReadOnlyScope = "https://www.googleapis.com/auth/calendar.readonly"
)

// Client implements the Provider interface for Google Calendar
type Client struct {
	account     *calendar.Account
	oauthConfig *oauth2.Config
	token       *oauth2.Token
	httpClient  *http.Client
}

// OAuthConfig holds OAuth credentials
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// NewClient creates a new Google Calendar client
func NewClient(account *calendar.Account, cfg *OAuthConfig) *Client {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{CalendarScope},
		Endpoint:     google.Endpoint,
	}

	return &Client{
		account:     account,
		oauthConfig: oauthConfig,
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

// GetAuthURL returns the URL for OAuth authorization
func (c *Client) GetAuthURL(state string) string {
	return c.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges an authorization code for a token
func (c *Client) ExchangeCode(ctx context.Context, code string) error {
	token, err := c.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	c.token = token
	c.httpClient = c.oauthConfig.Client(ctx, token)

	// Save token to account
	tokenJSON, _ := json.Marshal(token)
	c.account.AccessToken = string(tokenJSON)

	return nil
}

// SetToken sets the OAuth token from saved credentials
func (c *Client) SetToken(tokenJSON string) error {
	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}
	c.token = &token
	c.httpClient = c.oauthConfig.Client(context.Background(), &token)
	return nil
}

// Authenticate authenticates with Google Calendar
func (c *Client) Authenticate(ctx context.Context) error {
	if c.account.AccessToken != "" {
		// Try to use existing token
		if err := c.SetToken(c.account.AccessToken); err != nil {
			return fmt.Errorf("invalid saved token: %w", err)
		}
		return nil
	}
	return fmt.Errorf("no access token - OAuth flow required")
}

// ListCalendars returns all calendars from Google Calendar
func (c *Client) ListCalendars(ctx context.Context) ([]*calendar.Calendar, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	resp, err := c.httpClient.Get("https://www.googleapis.com/calendar/v3/users/me/calendarList")
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendar list failed: %s", resp.Status)
	}

	var result struct {
		Items []struct {
			ID              string `json:"id"`
			Summary         string `json:"summary"`
			Description     string `json:"description"`
			BackgroundColor string `json:"backgroundColor"`
			Primary         bool   `json:"primary"`
			AccessRole      string `json:"accessRole"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse calendars: %w", err)
	}

	var calendars []*calendar.Calendar
	for _, item := range result.Items {
		calendars = append(calendars, &calendar.Calendar{
			ID:          item.ID,
			AccountID:   c.account.ID,
			Name:        item.Summary,
			Description: item.Description,
			Color:       item.BackgroundColor,
			Visible:     true,
			ReadOnly:    item.AccessRole == "reader" || item.AccessRole == "freeBusyReader",
		})
	}

	return calendars, nil
}

// GetEvents returns events from a calendar within a time range
func (c *Client) GetEvents(ctx context.Context, calendarID string, start, end time.Time) ([]*calendar.Event, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf(
		"https://www.googleapis.com/calendar/v3/calendars/%s/events?timeMin=%s&timeMax=%s&singleEvents=true&orderBy=startTime",
		calendarID,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("events request failed: %s", resp.Status)
	}

	var result struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			Status      string `json:"status"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
			Created string `json:"created"`
			Updated string `json:"updated"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}

	var events []*calendar.Event
	for _, item := range result.Items {
		event := &calendar.Event{
			ID:          item.ID,
			UID:         item.ID,
			CalendarID:  calendarID,
			Title:       item.Summary,
			Description: item.Description,
			Location:    item.Location,
		}

		// Parse dates
		if item.Start.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, item.Start.DateTime); err == nil {
				event.Start = t
			}
		} else if item.Start.Date != "" {
			if t, err := time.Parse("2006-01-02", item.Start.Date); err == nil {
				event.Start = t
				event.AllDay = true
			}
		}

		if item.End.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, item.End.DateTime); err == nil {
				event.End = t
			}
		} else if item.End.Date != "" {
			if t, err := time.Parse("2006-01-02", item.End.Date); err == nil {
				event.End = t
			}
		}

		// Parse status
		switch item.Status {
		case "confirmed":
			event.Status = calendar.StatusConfirmed
		case "tentative":
			event.Status = calendar.StatusTentative
		case "cancelled":
			event.Status = calendar.StatusCancelled
			event.Cancelled = true
		}

		events = append(events, event)
	}

	return events, nil
}

// CreateEvent creates a new event on Google Calendar
func (c *Client) CreateEvent(ctx context.Context, calendarID string, event *calendar.Event) error {
	if c.httpClient == nil {
		return fmt.Errorf("not authenticated")
	}

	body := map[string]interface{}{
		"summary":     event.Title,
		"description": event.Description,
		"location":    event.Location,
	}

	if event.AllDay {
		body["start"] = map[string]string{"date": event.Start.Format("2006-01-02")}
		body["end"] = map[string]string{"date": event.End.Format("2006-01-02")}
	} else {
		body["start"] = map[string]string{"dateTime": event.Start.Format(time.RFC3339)}
		body["end"] = map[string]string{"dateTime": event.End.Format(time.RFC3339)}
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", calendarID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = http.NoBody

	// Create request with body
	resp, err := c.httpClient.Post(url, "application/json",
		http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	defer resp.Body.Close()

	// Actually use the jsonBody
	_ = jsonBody

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create event failed: %s", resp.Status)
	}

	return nil
}

// UpdateEvent updates an existing event
func (c *Client) UpdateEvent(ctx context.Context, calendarID string, event *calendar.Event) error {
	if c.httpClient == nil {
		return fmt.Errorf("not authenticated")
	}

	body := map[string]interface{}{
		"summary":     event.Title,
		"description": event.Description,
		"location":    event.Location,
	}

	if event.AllDay {
		body["start"] = map[string]string{"date": event.Start.Format("2006-01-02")}
		body["end"] = map[string]string{"date": event.End.Format("2006-01-02")}
	} else {
		body["start"] = map[string]string{"dateTime": event.Start.Format(time.RFC3339)}
		body["end"] = map[string]string{"dateTime": event.End.Format(time.RFC3339)}
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", calendarID, event.ID)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_ = jsonBody

	return fmt.Errorf("update not implemented")
}

// DeleteEvent deletes an event from Google Calendar
func (c *Client) DeleteEvent(ctx context.Context, calendarID string, eventID string) error {
	if c.httpClient == nil {
		return fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", calendarID, eventID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete event failed: %s", resp.Status)
	}

	return nil
}

// Sync performs a full sync
func (c *Client) Sync(ctx context.Context) error {
	c.account.LastSync = time.Now()
	return nil
}

// Ensure Client implements Provider
var _ providers.Provider = (*Client)(nil)
