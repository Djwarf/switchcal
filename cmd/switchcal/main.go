package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/djwarf/switchcal/internal/config"
	"github.com/djwarf/switchcal/pkg/calendar"
	"github.com/djwarf/switchcal/pkg/providers/caldav"
)

// Custom CSS theme for SwitchCal
const switchcalCSS = `
/* ── Accent palette ── */
@define-color sc_accent #7c6ef6;
@define-color sc_accent_dim alpha(@sc_accent, 0.25);
@define-color sc_accent_hover alpha(@sc_accent, 0.12);
@define-color sc_surface alpha(@theme_fg_color, 0.04);
@define-color sc_surface_hover alpha(@theme_fg_color, 0.08);
@define-color sc_border alpha(@theme_fg_color, 0.08);
@define-color sc_muted alpha(@theme_fg_color, 0.45);

/* ── Sidebar ── */
.sc-sidebar {
    background: alpha(@theme_bg_color, 0.6);
    padding: 12px;
}
.sc-sidebar-list {
    background: none;
    background-color: transparent;
}
.sc-sidebar-list > row {
    background: none;
    background-color: transparent;
}
.sc-sidebar scrolledwindow {
    background: none;
    background-color: transparent;
}
.sc-sidebar scrolledwindow > viewport {
    background: none;
    background-color: transparent;
}
.sc-sidebar-title {
    font-weight: 800;
    font-size: 11px;
    letter-spacing: 2px;
    color: @sc_muted;
    padding: 4px 4px 8px 4px;
}
.sc-add-account {
    border-radius: 8px;
    padding: 6px 12px;
    font-size: 13px;
}
.sc-cal-color {
    border-radius: 50%;
    min-width: 10px;
    min-height: 10px;
}
.sc-cal-name {
    font-size: 13px;
}

/* ── Month header ── */
.sc-header {
    padding: 4px 8px;
}
.sc-month-label {
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.5px;
}
.sc-nav-btn {
    border-radius: 8px;
    min-width: 34px;
    min-height: 34px;
    padding: 0;
}
.sc-today-btn {
    border-radius: 8px;
    padding: 4px 16px;
    font-weight: 600;
    font-size: 13px;
}
.sc-add-event-btn {
    border-radius: 8px;
    padding: 4px 14px;
    font-weight: 600;
    font-size: 13px;
    background: @sc_accent;
    color: white;
}
.sc-add-event-btn:hover {
    background: lighter(@sc_accent);
}
.sc-toggle-btn {
    border-radius: 50%;
    min-width: 34px;
    min-height: 34px;
    padding: 0;
}

/* ── Day headers ── */
.sc-day-header {
    font-size: 11px;
    font-weight: 700;
    color: @sc_muted;
    padding: 8px 0;
    letter-spacing: 1px;
}

/* ── Day cells ── */
.sc-day-cell {
    border-radius: 8px;
    background: @sc_surface;
    transition: background 150ms ease-in-out;
    padding: 0;
    min-width: 0;
    min-height: 0;
}
.sc-day-cell:hover {
    background: @sc_surface_hover;
}
.sc-day-cell.sc-selected {
    background: @sc_accent_dim;
    border: 1px solid alpha(@sc_accent, 0.4);
}
.sc-day-cell.sc-today {
    border: 2px solid @sc_accent;
}
.sc-day-cell.sc-today.sc-selected {
    border: 2px solid @sc_accent;
    background: @sc_accent_dim;
}
.sc-day-number {
    font-size: 13px;
    font-weight: 500;
    color: @sc_muted;
}
.sc-day-number.sc-today {
    color: @sc_accent;
    font-weight: 800;
}

/* ── Event pills in day cells ── */
.sc-event-pill {
    border-radius: 3px;
    padding: 0px 4px;
    font-size: 9px;
    margin-top: 1px;
    font-weight: 500;
}
.sc-more-events {
    font-size: 9px;
    font-weight: 600;
    color: @sc_muted;
    padding: 0px 4px;
}

/* ── Day detail panel ── */
.sc-detail {
    background: alpha(@theme_bg_color, 0.6);
    padding: 16px;
}
.sc-detail-date {
    font-size: 16px;
    font-weight: 700;
    padding-bottom: 8px;
}
.sc-no-events {
    font-size: 13px;
    color: @sc_muted;
    padding-top: 24px;
}

/* ── Event cards ── */
.sc-event-card {
    border-radius: 10px;
    padding: 12px 14px;
    margin-top: 6px;
    background: @sc_surface;
    transition: background 150ms ease-in-out;
}
.sc-event-card:hover {
    background: @sc_surface_hover;
}
.sc-event-title {
    font-size: 14px;
    font-weight: 600;
}
.sc-event-time {
    font-size: 12px;
    color: @sc_muted;
    margin-top: 2px;
}
.sc-event-location {
    font-size: 12px;
    color: @sc_muted;
    margin-top: 2px;
}
.sc-add-event-day {
    border-radius: 8px;
    padding: 8px;
    margin-top: 12px;
    font-size: 13px;
}

/* ── Calendar grid spacing ── */
.sc-grid {
    margin-top: 8px;
}

/* ── Color stripe on event cards ── */
.sc-color-stripe {
    border-radius: 3px;
    min-width: 4px;
    margin-right: 10px;
}
`

// Google OAuth configuration
const (
	googleClientID     = "707683257072-qhapb7fq21cc2too73ovopdobrpigdr9.apps.googleusercontent.com"
	googleClientSecret = "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"
	googleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	googleCalDAVURL    = "https://apidata.googleusercontent.com/caldav/v2/"
	googleScope        = "https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/userinfo.email"
	redirectPort       = 8085
)

// App holds the application state
type App struct {
	config    *config.Config
	store     *calendar.Store
	window    *gtk.ApplicationWindow

	// UI components
	sidebar       *gtk.Box
	sidebarSep    *gtk.Separator
	calendarList  *gtk.ListBox
	monthBox      *gtk.Box
	monthView     *gtk.Grid
	monthLabel    *gtk.Label
	dayDetail     *gtk.Box
	contentPaned  *gtk.Paned
	sidebarBtn    *gtk.ToggleButton
	detailBtn     *gtk.ToggleButton

	// Day cell tracking for efficient updates
	dayWidgets        map[int]*gtk.Button // day number -> button widget
	selectedDayWidget *gtk.Button

	// Debounce timer for UI refreshes
	refreshTimer *time.Timer
	refreshMu    sync.Mutex

	// Responsive layout state
	sidebarVisible   bool
	dayDetailVisible bool

	// Current state
	currentDate  time.Time
	selectedDate time.Time

	// Calendar color cache (calendarID → hex color)
	calendarColors map[string]string

	// Background sync
	syncTicker *time.Ticker
	syncDone   chan struct{}
}

// WaybarOutput is the JSON structure for waybar custom modules
type WaybarOutput struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

func main() {
	// Check for waybar mode
	if len(os.Args) > 1 && (os.Args[1] == "--waybar" || os.Args[1] == "waybar") {
		runWaybarMode()
		return
	}

	app := gtk.NewApplication("com.djwarf.switchcal", 0)
	app.ConnectActivate(func() { activate(app) })

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}

// runWaybarMode outputs calendar info as JSON for waybar
func runWaybarMode() {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	store, err := calendar.NewStore(cfg.DatabasePath())
	if err != nil {
		// Output empty state on error
		output := WaybarOutput{
			Text:    time.Now().Format("02/01"),
			Tooltip: "Calendar unavailable",
			Class:   "error",
		}
		json.NewEncoder(os.Stdout).Encode(output)
		return
	}
	defer store.Close()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	events, err := store.GetEventsInRange(today, tomorrow)
	if err != nil {
		events = []*calendar.Event{}
	}

	// Build tooltip with today's events
	var tooltip strings.Builder
	tooltip.WriteString(now.Format("Monday, 2 January 2006"))

	if len(events) > 0 {
		tooltip.WriteString("\n\n")
		for i, event := range events {
			if i > 0 {
				tooltip.WriteString("\n")
			}
			eventTime := event.Start.Format(cfg.TimeFormat())
			if event.AllDay {
				eventTime = "All day"
			}
			tooltip.WriteString(fmt.Sprintf("• %s - %s", eventTime, event.Title))
		}
	} else {
		tooltip.WriteString("\n\nNo events today")
	}

	// Determine class based on event count
	class := "no-events"
	if len(events) > 0 {
		class = "has-events"
	}

	// Format text: date and event count
	text := now.Format("02/01")
	if len(events) > 0 {
		text = fmt.Sprintf("%s (%d)", text, len(events))
	}

	output := WaybarOutput{
		Text:    text,
		Tooltip: tooltip.String(),
		Class:   class,
	}
	json.NewEncoder(os.Stdout).Encode(output)
}

func activate(gtkApp *gtk.Application) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config: %v", err)
		cfg = config.DefaultConfig()
	}

	// Open store
	store, err := calendar.NewStore(cfg.DatabasePath())
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	app := &App{
		config:           cfg,
		store:            store,
		currentDate:      time.Now(),
		selectedDate:     time.Now(),
		dayWidgets:       make(map[int]*gtk.Button),
		calendarColors:   make(map[string]string),
		sidebarVisible:   false,
		dayDetailVisible: false,
	}

	// Close database and stop background sync on app shutdown
	gtkApp.ConnectShutdown(func() {
		log.Printf("App shutting down, closing database...")
		if app.syncTicker != nil {
			app.syncTicker.Stop()
			close(app.syncDone)
		}
		store.Close()
	})

	// Ensure a default local calendar exists
	app.ensureDefaultCalendar()

	app.buildUI(gtkApp)
	app.loadCalendars()
	app.refreshMonthView()

	// Sync all Google accounts on startup
	go app.syncAllAccounts()

	// Start periodic background sync
	if app.config.SyncIntervalSeconds > 0 {
		interval := time.Duration(app.config.SyncIntervalSeconds) * time.Second
		app.syncTicker = time.NewTicker(interval)
		app.syncDone = make(chan struct{})
		go func() {
			for {
				select {
				case <-app.syncTicker.C:
					app.syncAllAccounts()
				case <-app.syncDone:
					return
				}
			}
		}()
		log.Printf("Background sync enabled: every %ds", app.config.SyncIntervalSeconds)
	}
}

// syncAllAccounts syncs calendars from all configured accounts
func (app *App) syncAllAccounts() {
	accounts, err := app.store.GetAllAccounts()
	if err != nil {
		log.Printf("Error loading accounts for sync: %v", err)
		return
	}

	for _, account := range accounts {
		if account.Type == calendar.AccountTypeGoogle && account.AccessToken != "" {
			app.syncCalDAVAccount(account)
		}
	}
}

// ensureDefaultCalendar creates a default local calendar if none exist
func (app *App) ensureDefaultCalendar() {
	calendars, err := app.store.GetAllCalendars()
	if err != nil {
		log.Printf("Error checking calendars: %v", err)
		return
	}

	if len(calendars) == 0 {
		// Create a default local account
		account := &calendar.Account{
			ID:      fmt.Sprintf("acc-%d", time.Now().UnixNano()),
			Name:    "Local",
			Type:    calendar.AccountTypeLocal,
			Enabled: true,
		}
		if err := app.store.SaveAccount(account); err != nil {
			log.Printf("Error creating default account: %v", err)
			return
		}

		// Create a default calendar
		cal := &calendar.Calendar{
			ID:        fmt.Sprintf("cal-%d", time.Now().UnixNano()),
			AccountID: account.ID,
			Name:      "My Calendar",
			Color:     "#4285f4",
			Visible:   true,
		}
		if err := app.store.SaveCalendar(cal); err != nil {
			log.Printf("Error creating default calendar: %v", err)
			return
		}
		log.Println("Created default local calendar")
	}
}

func (app *App) buildUI(gtkApp *gtk.Application) {
	// Load custom CSS
	cssProvider := gtk.NewCSSProvider()
	cssProvider.LoadFromString(switchcalCSS)
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		cssProvider,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)

	app.window = gtk.NewApplicationWindow(gtkApp)
	app.window.SetTitle("SwitchCal")
	app.window.SetDefaultSize(app.config.WindowWidth, app.config.WindowHeight)
	app.window.SetSizeRequest(420, 350) // Minimum size

	// Main layout - needs to expand to fill window
	mainBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	mainBox.SetHExpand(true)
	mainBox.SetVExpand(true)

	// Sidebar - fixed width
	app.sidebar = app.buildSidebar()
	mainBox.Append(app.sidebar)

	// Separator between sidebar and content
	app.sidebarSep = gtk.NewSeparator(gtk.OrientationVertical)
	mainBox.Append(app.sidebarSep)

	// Month view - expands to fill available space
	app.monthBox = app.buildMonthView()
	app.monthBox.SetHExpand(true)
	app.monthBox.SetVExpand(true)

	// Day detail panel
	app.dayDetail = app.buildDayDetail()
	app.dayDetail.SetHExpand(true)
	app.dayDetail.SetVExpand(true)

	// Paned for resizable calendar + day detail
	app.contentPaned = gtk.NewPaned(gtk.OrientationHorizontal)
	app.contentPaned.SetHExpand(true)
	app.contentPaned.SetVExpand(true)
	app.contentPaned.SetStartChild(app.monthBox)
	app.contentPaned.SetEndChild(app.dayDetail)
	app.contentPaned.SetResizeStartChild(true)
	app.contentPaned.SetResizeEndChild(true)
	app.contentPaned.SetShrinkStartChild(true)
	app.contentPaned.SetShrinkEndChild(true)
	app.contentPaned.SetWideHandle(true)
	app.contentPaned.SetPosition(app.config.WindowWidth * 6 / 10)

	mainBox.Append(app.contentPaned)

	app.window.SetChild(mainBox)

	// Handle window resize for responsive layout
	app.window.Object.NotifyProperty("default-width", func() {
		app.updateLayoutForWidth()
	})

	app.window.Show()

	// Set initial layout based on window size
	app.updateLayoutForWidth()
}

func (app *App) buildSidebar() *gtk.Box {
	sidebar := gtk.NewBox(gtk.OrientationVertical, 6)
	sidebar.AddCSSClass("sc-sidebar")
	sidebar.SetSizeRequest(200, -1)

	// Header
	header := gtk.NewLabel("CALENDARS")
	header.AddCSSClass("sc-sidebar-title")
	header.SetXAlign(0)
	sidebar.Append(header)

	// Add account button
	addBtn := gtk.NewButtonWithLabel("+ Add Account")
	addBtn.AddCSSClass("sc-add-account")
	addBtn.ConnectClicked(func() {
		app.showAddAccountDialog()
	})
	sidebar.Append(addBtn)

	// Calendar list
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrolled.SetMarginTop(8)

	app.calendarList = gtk.NewListBox()
	app.calendarList.SetSelectionMode(gtk.SelectionNone)
	app.calendarList.AddCSSClass("sc-sidebar-list")
	scrolled.SetChild(app.calendarList)
	sidebar.Append(scrolled)

	return sidebar
}

func (app *App) buildMonthView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	view.SetHExpand(true)
	view.SetVExpand(true)
	view.SetMarginTop(8)
	view.SetMarginBottom(8)
	view.SetMarginStart(12)
	view.SetMarginEnd(12)

	// Header with navigation
	headerBox := gtk.NewBox(gtk.OrientationHorizontal, 6)
	headerBox.AddCSSClass("sc-header")

	// Sidebar toggle button
	app.sidebarBtn = gtk.NewToggleButton()
	app.sidebarBtn.SetIconName("open-menu-symbolic")
	app.sidebarBtn.SetActive(app.sidebarVisible)
	app.sidebarBtn.SetTooltipText("Toggle sidebar")
	app.sidebarBtn.AddCSSClass("sc-toggle-btn")
	app.sidebarBtn.ConnectToggled(func() {
		app.setSidebarVisible(app.sidebarBtn.Active())
	})
	headerBox.Append(app.sidebarBtn)

	prevBtn := gtk.NewButtonFromIconName("go-previous-symbolic")
	prevBtn.AddCSSClass("sc-nav-btn")
	prevBtn.ConnectClicked(func() {
		app.currentDate = app.currentDate.AddDate(0, -1, 0)
		app.refreshMonthView()
	})

	nextBtn := gtk.NewButtonFromIconName("go-next-symbolic")
	nextBtn.AddCSSClass("sc-nav-btn")
	nextBtn.ConnectClicked(func() {
		app.currentDate = app.currentDate.AddDate(0, 1, 0)
		app.refreshMonthView()
	})

	todayBtn := gtk.NewButtonWithLabel("Today")
	todayBtn.AddCSSClass("sc-today-btn")
	todayBtn.ConnectClicked(func() {
		app.currentDate = time.Now()
		app.selectedDate = time.Now()
		app.refreshMonthView()
		app.refreshDayDetail()
	})

	app.monthLabel = gtk.NewLabel("")
	app.monthLabel.AddCSSClass("sc-month-label")
	app.monthLabel.SetHExpand(true)
	app.monthLabel.SetXAlign(0.5)

	headerBox.Append(prevBtn)
	headerBox.Append(nextBtn)
	headerBox.Append(todayBtn)
	headerBox.Append(app.monthLabel)

	// Add event button
	addEventBtn := gtk.NewButtonWithLabel("+ Event")
	addEventBtn.AddCSSClass("sc-add-event-btn")
	addEventBtn.ConnectClicked(func() {
		app.showEventDialog(nil)
	})
	headerBox.Append(addEventBtn)

	// Day detail toggle button
	app.detailBtn = gtk.NewToggleButton()
	app.detailBtn.SetIconName("view-reveal-symbolic")
	app.detailBtn.SetActive(app.dayDetailVisible)
	app.detailBtn.SetTooltipText("Toggle day details")
	app.detailBtn.AddCSSClass("sc-toggle-btn")
	app.detailBtn.ConnectToggled(func() {
		app.setDayDetailVisible(app.detailBtn.Active())
	})
	headerBox.Append(app.detailBtn)

	view.Append(headerBox)

	// Calendar grid
	app.monthView = gtk.NewGrid()
	app.monthView.AddCSSClass("sc-grid")
	app.monthView.SetRowHomogeneous(true)
	app.monthView.SetColumnHomogeneous(true)
	app.monthView.SetVExpand(true)
	app.monthView.SetHExpand(true)
	app.monthView.SetRowSpacing(4)
	app.monthView.SetColumnSpacing(4)

	view.Append(app.monthView)

	return view
}

func (app *App) buildDayDetail() *gtk.Box {
	detail := gtk.NewBox(gtk.OrientationVertical, 4)
	detail.AddCSSClass("sc-detail")
	detail.SetHExpand(true)
	detail.SetVExpand(true)

	// Will be populated by refreshDayDetail
	placeholder := gtk.NewLabel("Select a day to view events")
	placeholder.AddCSSClass("sc-no-events")
	detail.Append(placeholder)

	return detail
}

func (app *App) refreshMonthView() {
	// Run database query in background to avoid blocking UI
	go func() {
		// Get first day of month
		firstOfMonth := time.Date(app.currentDate.Year(), app.currentDate.Month(), 1, 0, 0, 0, 0, app.currentDate.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

		// Get events for this month (background thread)
		monthStart := firstOfMonth
		monthEnd := lastOfMonth.Add(24 * time.Hour)
		events, _ := app.store.GetEventsInRange(monthStart, monthEnd)

		// Create event map by date
		eventsByDate := make(map[string][]*calendar.Event)
		for _, e := range events {
			dateKey := e.Start.Format("2006-01-02")
			eventsByDate[dateKey] = append(eventsByDate[dateKey], e)
		}

		// Update UI on main thread
		glib.IdleAdd(func() {
			app.updateMonthViewWithEvents(eventsByDate)
		})
	}()
}

// updateMonthViewWithEvents updates the month view with pre-fetched events (must run on main thread)
func (app *App) updateMonthViewWithEvents(eventsByDate map[string][]*calendar.Event) {
	// Clear all cells
	for row := 0; row < 7; row++ {
		for col := 0; col < 7; col++ {
			if child := app.monthView.ChildAt(col, row); child != nil {
				app.monthView.Remove(child)
			}
		}
	}

	// Clear day widget tracking
	app.dayWidgets = make(map[int]*gtk.Button)
	app.selectedDayWidget = nil

	// Update month label
	app.monthLabel.SetText(app.currentDate.Format("January 2006"))

	// Day headers
	days := []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
	for i, day := range days {
		label := gtk.NewLabel(day)
		label.AddCSSClass("sc-day-header")
		app.monthView.Attach(label, i, 0, 1, 1)
	}

	// Get first day of month
	firstOfMonth := time.Date(app.currentDate.Year(), app.currentDate.Month(), 1, 0, 0, 0, 0, app.currentDate.Location())

	// Get weekday (0=Sunday in Go, we want 0=Monday)
	weekday := int(firstOfMonth.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekday-- // Now 0=Monday

	// Get days in month
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	daysInMonth := lastOfMonth.Day()

	// Fill in days
	today := time.Now()
	row := 1
	col := weekday

	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(app.currentDate.Year(), app.currentDate.Month(), day, 0, 0, 0, 0, app.currentDate.Location())
		dateKey := date.Format("2006-01-02")
		dayEvents := eventsByDate[dateKey]

		isToday := date.Year() == today.Year() && date.Month() == today.Month() && date.Day() == today.Day()
		isSelected := date.Year() == app.selectedDate.Year() && date.Month() == app.selectedDate.Month() && date.Day() == app.selectedDate.Day()

		cell := app.createDayCell(day, date, dayEvents, isToday, isSelected)

		// Track the widget for efficient selection updates
		app.dayWidgets[day] = cell
		if isSelected {
			app.selectedDayWidget = cell
		}

		app.monthView.Attach(cell, col, row, 1, 1)

		col++
		if col > 6 {
			col = 0
			row++
		}
	}
}

func (app *App) createDayCell(day int, date time.Time, events []*calendar.Event, isToday, isSelected bool) *gtk.Button {
	btn := gtk.NewButton()
	btn.SetHExpand(true)
	btn.SetVExpand(true)
	btn.AddCSSClass("sc-day-cell")

	if isToday {
		btn.AddCSSClass("sc-today")
	}
	if isSelected {
		btn.AddCSSClass("sc-selected")
	}

	box := gtk.NewBox(gtk.OrientationVertical, 1)
	box.SetMarginTop(2)
	box.SetMarginBottom(2)
	box.SetMarginStart(4)
	box.SetMarginEnd(4)

	// Day number
	dayLabel := gtk.NewLabel(strconv.Itoa(day))
	dayLabel.AddCSSClass("sc-day-number")
	dayLabel.SetXAlign(0)
	if isToday {
		dayLabel.AddCSSClass("sc-today")
	}
	box.Append(dayLabel)

	// Event pills (show up to 3)
	for i, event := range events {
		if i >= 3 {
			moreLabel := gtk.NewLabel(fmt.Sprintf("+%d", len(events)-3))
			moreLabel.AddCSSClass("sc-more-events")
			moreLabel.SetXAlign(0)
			box.Append(moreLabel)
			break
		}
		pill := gtk.NewLabel(truncate(event.Title, 12))
		pill.SetXAlign(0)
		pill.SetEllipsize(3) // PANGO_ELLIPSIZE_END
		pill.AddCSSClass("sc-event-pill")

		// Apply calendar color as inline CSS
		color := app.calendarColors[event.CalendarID]
		if color == "" {
			color = "#7c6ef6"
		}
		pillCSS := gtk.NewCSSProvider()
		pillCSS.LoadFromString(fmt.Sprintf(
			".sc-event-pill { background: alpha(%s, 0.2); color: %s; }", color, color))
		pill.StyleContext().AddProvider(pillCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+1)
		box.Append(pill)
	}

	btn.SetChild(box)

	// Click handler
	btn.ConnectClicked(func() {
		app.selectDay(day, date, btn)
	})

	return btn
}

// scheduleRefresh debounces rapid UI refresh requests (e.g., multiple calendar toggles)
func (app *App) scheduleRefresh() {
	app.refreshMu.Lock()
	defer app.refreshMu.Unlock()

	// Cancel any pending refresh
	if app.refreshTimer != nil {
		app.refreshTimer.Stop()
	}

	// Schedule a new refresh after a short delay
	app.refreshTimer = time.AfterFunc(100*time.Millisecond, func() {
		glib.IdleAdd(func() {
			app.refreshMonthView()
			app.refreshDayDetail()
		})
	})
}

// selectDay efficiently switches the selected day without rebuilding the month view
func (app *App) selectDay(day int, date time.Time, btn *gtk.Button) {
	// Only process if it's a different day
	if app.selectedDate.Year() == date.Year() && app.selectedDate.Month() == date.Month() && app.selectedDate.Day() == date.Day() {
		return
	}

	// Remove selection from old day
	if app.selectedDayWidget != nil {
		app.selectedDayWidget.RemoveCSSClass("sc-selected")
	}

	// Update state and add selection to new day
	app.selectedDate = date
	app.selectedDayWidget = btn
	btn.AddCSSClass("sc-selected")

	// Only refresh the day detail panel (async)
	app.refreshDayDetail()
}

func (app *App) refreshDayDetail() {
	// Capture selected date to avoid race conditions
	selectedDate := app.selectedDate

	// Fetch events in background
	go func() {
		events, err := app.store.GetEventsForDate(selectedDate)
		if err != nil {
			log.Printf("Error loading events: %v", err)
			events = []*calendar.Event{}
		}

		// Update UI on main thread
		glib.IdleAdd(func() {
			app.updateDayDetailWithEvents(selectedDate, events)
		})
	}()
}

// updateDayDetailWithEvents updates the day detail panel with pre-fetched events (must run on main thread)
func (app *App) updateDayDetailWithEvents(date time.Time, events []*calendar.Event) {
	// Only update if the selected date hasn't changed
	if app.selectedDate != date {
		return
	}

	// Clear existing children
	for {
		child := app.dayDetail.FirstChild()
		if child == nil {
			break
		}
		app.dayDetail.Remove(child)
	}

	// Header
	dateLabel := gtk.NewLabel(date.Format("Monday, 2 January 2006"))
	dateLabel.AddCSSClass("sc-detail-date")
	dateLabel.SetXAlign(0)
	app.dayDetail.Append(dateLabel)

	if len(events) == 0 {
		noEvents := gtk.NewLabel("No events scheduled")
		noEvents.AddCSSClass("sc-no-events")
		app.dayDetail.Append(noEvents)
	} else {
		for _, event := range events {
			eventBox := app.createEventCard(event)
			app.dayDetail.Append(eventBox)
		}
	}

	// Add event button
	addBtn := gtk.NewButtonWithLabel("+ Add Event")
	addBtn.AddCSSClass("sc-add-event-day")
	addBtn.ConnectClicked(func() {
		app.showEventDialog(nil)
	})
	app.dayDetail.Append(addBtn)
}

func (app *App) createEventCard(event *calendar.Event) *gtk.Box {
	// Outer horizontal box: color stripe + content
	card := gtk.NewBox(gtk.OrientationHorizontal, 0)
	card.AddCSSClass("sc-event-card")

	// Color stripe
	color := app.calendarColors[event.CalendarID]
	if color == "" {
		color = "#7c6ef6"
	}
	stripe := gtk.NewBox(gtk.OrientationVertical, 0)
	stripe.AddCSSClass("sc-color-stripe")
	stripeCSS := gtk.NewCSSProvider()
	stripeCSS.LoadFromString(fmt.Sprintf(".sc-color-stripe { background: %s; }", color))
	stripe.StyleContext().AddProvider(stripeCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+1)
	card.Append(stripe)

	// Content
	content := gtk.NewBox(gtk.OrientationVertical, 2)

	title := gtk.NewLabel(event.Title)
	title.AddCSSClass("sc-event-title")
	title.SetXAlign(0)
	content.Append(title)

	var timeStr string
	if event.AllDay {
		timeStr = "All day"
	} else {
		timeStr = event.Start.Format(app.config.TimeFormat()) + " — " + event.End.Format(app.config.TimeFormat())
	}
	timeLabel := gtk.NewLabel(timeStr)
	timeLabel.AddCSSClass("sc-event-time")
	timeLabel.SetXAlign(0)
	content.Append(timeLabel)

	if event.Location != "" {
		locLabel := gtk.NewLabel(event.Location)
		locLabel.AddCSSClass("sc-event-location")
		locLabel.SetXAlign(0)
		content.Append(locLabel)
	}

	card.Append(content)

	return card
}

func (app *App) loadCalendars() {
	// Clear existing
	for {
		child := app.calendarList.FirstChild()
		if child == nil {
			break
		}
		app.calendarList.Remove(child)
	}

	accounts, err := app.store.GetAllAccounts()
	if err != nil {
		log.Printf("Error loading accounts: %v", err)
		return
	}

	calendars, err := app.store.GetAllCalendars()
	if err != nil {
		log.Printf("Error loading calendars: %v", err)
		return
	}

	// Build calendar color cache
	app.calendarColors = make(map[string]string)
	for _, cal := range calendars {
		if cal.Color != "" {
			app.calendarColors[cal.ID] = cal.Color
		}
	}

	if len(calendars) == 0 {
		label := gtk.NewLabel("No calendars yet")
		label.AddCSSClass("sc-no-events")
		label.SetMarginTop(20)
		app.calendarList.Append(label)
		return
	}

	// Group calendars by account
	calsByAccount := make(map[string][]*calendar.Calendar)
	for _, cal := range calendars {
		calsByAccount[cal.AccountID] = append(calsByAccount[cal.AccountID], cal)
	}

	// Create collapsible section for each account
	for _, account := range accounts {
		cals := calsByAccount[account.ID]
		if len(cals) == 0 {
			continue
		}

		// Create expander for this account
		expander := gtk.NewExpander(account.Name)
		expander.SetExpanded(true)

		// Box to hold calendars
		calBox := gtk.NewBox(gtk.OrientationVertical, 2)
		calBox.SetMarginStart(12)

		for _, cal := range cals {
			row := app.createCalendarRow(cal)
			calBox.Append(row)
		}

		expander.SetChild(calBox)
		app.calendarList.Append(expander)
	}
}

func (app *App) createCalendarRow(cal *calendar.Calendar) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.SetMarginTop(4)
	row.SetMarginBottom(4)

	// Checkbox
	check := gtk.NewCheckButton()
	check.SetActive(cal.Visible)
	check.ConnectToggled(func() {
		cal.Visible = check.Active()
		go app.store.SaveCalendar(cal)
		app.scheduleRefresh()
	})
	row.Append(check)

	// Color dot
	colorDot := gtk.NewBox(gtk.OrientationVertical, 0)
	colorDot.AddCSSClass("sc-cal-color")
	colorDot.SetSizeRequest(10, 10)
	colorDot.SetVAlign(gtk.AlignCenter)
	color := cal.Color
	if color == "" {
		color = "#7c6ef6"
	}
	dotCSS := gtk.NewCSSProvider()
	dotCSS.LoadFromString(fmt.Sprintf(".sc-cal-color { background: %s; }", color))
	colorDot.StyleContext().AddProvider(dotCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+1)
	row.Append(colorDot)

	// Name
	name := gtk.NewLabel(cal.Name)
	name.AddCSSClass("sc-cal-name")
	name.SetXAlign(0)
	name.SetHExpand(true)
	name.SetEllipsize(3) // PANGO_ELLIPSIZE_END
	row.Append(name)

	return row
}

func (app *App) showAddAccountDialog() {
	dialog := gtk.NewDialog()
	dialog.SetTitle("Add Calendar Account")
	dialog.SetTransientFor(&app.window.Window)
	dialog.SetModal(true)
	dialog.SetDefaultSize(450, 350)

	content := dialog.ContentArea()
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)
	content.SetSpacing(12)

	// Account type selection
	typeLabel := gtk.NewLabel("Account Type:")
	typeLabel.SetXAlign(0)
	content.Append(typeLabel)

	typeCombo := gtk.NewComboBoxText()
	typeCombo.AppendText("Local Calendar")
	typeCombo.AppendText("Google Calendar (CalDAV)")
	typeCombo.AppendText("Apple iCloud (CalDAV)")
	typeCombo.AppendText("CalDAV Server")
	typeCombo.SetActive(0)
	content.Append(typeCombo)

	// Info label
	infoLabel := gtk.NewLabel("Create a local calendar stored on this device.")
	infoLabel.SetXAlign(0)
	infoLabel.SetWrap(true)
	infoLabel.AddCSSClass("dim-label")
	content.Append(infoLabel)

	// CalDAV fields container
	caldavBox := gtk.NewBox(gtk.OrientationVertical, 8)
	caldavBox.SetVisible(false)

	// Server URL (for CalDAV)
	serverLabel := gtk.NewLabel("Server URL:")
	serverLabel.SetXAlign(0)
	caldavBox.Append(serverLabel)
	serverEntry := gtk.NewEntry()
	serverEntry.SetPlaceholderText("https://caldav.example.com/")
	caldavBox.Append(serverEntry)

	// Username
	userLabel := gtk.NewLabel("Username/Email:")
	userLabel.SetXAlign(0)
	caldavBox.Append(userLabel)
	userEntry := gtk.NewEntry()
	userEntry.SetPlaceholderText("user@example.com")
	caldavBox.Append(userEntry)

	// Password/App Password
	passLabel := gtk.NewLabel("App Password:")
	passLabel.SetXAlign(0)
	caldavBox.Append(passLabel)
	passEntry := gtk.NewEntry()
	passEntry.SetVisibility(false)
	passEntry.SetPlaceholderText("App-specific password")
	caldavBox.Append(passEntry)

	content.Append(caldavBox)

	// Action button
	actionBtn := gtk.NewButtonWithLabel("Create Local Calendar")
	actionBtn.AddCSSClass("suggested-action")
	content.Append(actionBtn)

	// Update UI based on type selection
	updateUI := func() {
		active := typeCombo.Active()
		switch active {
		case 0: // Local
			infoLabel.SetText("Create a local calendar stored on this device.")
			infoLabel.SetVisible(true)
			caldavBox.SetVisible(false)
			actionBtn.SetLabel("Create Local Calendar")
		case 1: // Google CalDAV
			infoLabel.SetText("Sign in with your Google account to sync calendars.")
			infoLabel.SetVisible(true)
			caldavBox.SetVisible(false)
			actionBtn.SetLabel("Sign in with Google")
		case 2: // Apple iCloud
			infoLabel.SetText("For Apple iCloud, you need an app-specific password.\nGo to appleid.apple.com → Security → App-Specific Passwords")
			infoLabel.SetVisible(true)
			caldavBox.SetVisible(true)
			serverEntry.SetText("https://caldav.icloud.com/")
			serverEntry.SetSensitive(false)
			actionBtn.SetLabel("Connect Apple iCloud")
		case 3: // CalDAV
			infoLabel.SetText("Enter your CalDAV server details.")
			infoLabel.SetVisible(true)
			caldavBox.SetVisible(true)
			serverEntry.SetText("")
			serverEntry.SetSensitive(true)
			actionBtn.SetLabel("Connect CalDAV")
		}
	}

	typeCombo.Connect("changed", func() {
		updateUI()
	})

	// Initial update
	updateUI()

	// Handle button click
	actionBtn.ConnectClicked(func() {
		activeType := typeCombo.Active()

		switch activeType {
		case 0: // Local Calendar
			account := &calendar.Account{
				ID:      fmt.Sprintf("acc-%d", time.Now().UnixNano()),
				Name:    "Local",
				Type:    calendar.AccountTypeLocal,
				Enabled: true,
			}

			if err := app.store.SaveAccount(account); err != nil {
				log.Printf("Error saving account: %v", err)
				return
			}

			// Create default calendar
			localCal := &calendar.Calendar{
				ID:        fmt.Sprintf("cal-%d", time.Now().UnixNano()),
				AccountID: account.ID,
				Name:      "My Calendar",
				Color:     "#4285f4",
				Visible:   true,
			}
			app.store.SaveCalendar(localCal)

			app.loadCalendars()
			dialog.Close()

		case 1: // Google OAuth
			app.startGoogleOAuth(dialog, infoLabel)

		case 2, 3: // Apple or CalDAV
			username := userEntry.Text()
			password := passEntry.Text()
			serverURL := serverEntry.Text()

			if username == "" || password == "" || serverURL == "" {
				infoLabel.SetText("Please fill in all fields.")
				return
			}

			var accountType calendar.AccountType
			var accountName string
			switch activeType {
			case 2:
				accountType = calendar.AccountTypeApple
				accountName = "Apple - " + username
			case 3:
				accountType = calendar.AccountTypeCalDAV
				accountName = "CalDAV - " + username
			}

			account := &calendar.Account{
				ID:          fmt.Sprintf("acc-%d", time.Now().UnixNano()),
				Name:        accountName,
				Email:       username,
				Username:    username,
				AppPassword: password,
				ServerURL:   serverURL,
				Type:        accountType,
				Enabled:     true,
			}

			if err := app.store.SaveAccount(account); err != nil {
				log.Printf("Error saving account: %v", err)
				infoLabel.SetText("Error saving account: " + err.Error())
				return
			}

			// Try to sync calendars
			go app.syncCalDAVAccount(account)

			app.loadCalendars()
			dialog.Close()
		}
	})

	// Cancel button
	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.SetHAlign(gtk.AlignEnd)
	btnBox.SetMarginTop(12)

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	btnBox.Append(cancelBtn)

	content.Append(btnBox)
	dialog.Show()
}

// startGoogleOAuth initiates Google OAuth flow
func (app *App) startGoogleOAuth(parentDialog *gtk.Dialog, statusLabel *gtk.Label) {
	statusLabel.SetText("Opening browser for Google sign-in...")

	// Create callback channel
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Create a new mux to avoid handler conflicts
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code received")
			fmt.Fprintf(w, "<html><body><h1>Error</h1><p>No authorization code received.</p></body></html>")
			return
		}
		codeChan <- code
		fmt.Fprintf(w, "<html><body><h1>Success!</h1><p>You can close this window and return to SwitchCal.</p><script>window.close();</script></body></html>")
	})

	// Start local callback server with custom mux
	server := &http.Server{Addr: fmt.Sprintf(":%d", redirectPort), Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Build OAuth URL
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", redirectPort)
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent",
		googleAuthURL,
		url.QueryEscape(googleClientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(googleScope))

	// Open browser
	if err := openBrowser(authURL); err != nil {
		statusLabel.SetText("Failed to open browser: " + err.Error())
		return
	}

	// Wait for callback in background
	go func() {
		defer server.Shutdown(context.Background())

		select {
		case code := <-codeChan:
			// Exchange code for token
			accessToken, refreshToken, email, err := exchangeGoogleCode(code, redirectURI)
			if err != nil {
				glib.IdleAdd(func() {
					statusLabel.SetText("Auth failed: " + err.Error())
				})
				return
			}

			// Create and save account
			glib.IdleAdd(func() {
				// Google CalDAV URL includes user email
				serverURL := fmt.Sprintf("%s%s/", googleCalDAVURL, url.PathEscape(email))

				account := &calendar.Account{
					ID:           fmt.Sprintf("acc-%d", time.Now().UnixNano()),
					Name:         "Google - " + email,
					Email:        email,
					Type:         calendar.AccountTypeGoogle,
					ServerURL:    serverURL,
					AccessToken:  accessToken,
					RefreshToken: refreshToken,
					TokenExpiry:  time.Now().Add(time.Hour),
					Enabled:      true,
				}

				if err := app.store.SaveAccount(account); err != nil {
					statusLabel.SetText("Error saving: " + err.Error())
					return
				}

				// Sync calendars
				go app.syncCalDAVAccount(account)

				app.loadCalendars()
				parentDialog.Close()
			})

		case err := <-errChan:
			glib.IdleAdd(func() {
				statusLabel.SetText("Error: " + err.Error())
			})

		case <-time.After(5 * time.Minute):
			glib.IdleAdd(func() {
				statusLabel.SetText("Timeout - please try again")
			})
		}
	}()
}

// exchangeGoogleCode exchanges authorization code for tokens
func exchangeGoogleCode(code, redirectURI string) (accessToken, refreshToken, email string, err error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", googleClientID)
	data.Set("client_secret", googleClientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	resp, err := http.Post(googleTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	json.Unmarshal(body, &result)

	log.Printf("Token exchange - access token received: %v, refresh token received: %v",
		result.AccessToken != "", result.RefreshToken != "")

	if result.Error != "" {
		return "", "", "", fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}

	if result.RefreshToken == "" {
		log.Printf("WARNING: No refresh token received. Token refresh will fail after 1 hour.")
	}

	// Get user email
	email = getUserEmail(result.AccessToken)

	return result.AccessToken, result.RefreshToken, email, nil
}

// getUserEmail fetches the user's email from Google
func getUserEmail(accessToken string) string {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "Google Account"
	}
	defer resp.Body.Close()

	var result struct {
		Email string `json:"email"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Email != "" {
		return result.Email
	}
	return "Google Account"
}

// refreshGoogleToken refreshes an expired Google access token
func (app *App) refreshGoogleToken(account *calendar.Account) error {
	log.Printf("Attempting token refresh for account: %s", account.Email)
	log.Printf("Refresh token present: %v", account.RefreshToken != "")

	if account.RefreshToken == "" {
		return fmt.Errorf("no refresh token available - please re-authenticate")
	}

	data := url.Values{}
	data.Set("client_id", googleClientID)
	data.Set("client_secret", googleClientSecret)
	data.Set("refresh_token", account.RefreshToken)
	data.Set("grant_type", "refresh_token")

	resp, err := http.Post(googleTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refresh response: %w", err)
	}

	log.Printf("Token refresh response status: %d", resp.StatusCode)
	if resp.StatusCode != 200 {
		log.Printf("Token refresh response body: %s", string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("refresh failed: %s - %s", result.Error, result.ErrorDesc)
	}

	account.AccessToken = result.AccessToken
	account.TokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	log.Printf("Token refreshed successfully, expires in %d seconds", result.ExpiresIn)

	return app.store.SaveAccount(account)
}

// syncCalDAVAccount syncs calendars from a CalDAV account
func (app *App) syncCalDAVAccount(account *calendar.Account) {
	log.Printf("Syncing CalDAV account: %s", account.Name)

	// For Google, use the Calendar API directly (more reliable)
	if account.Type == calendar.AccountTypeGoogle {
		app.syncGoogleCalendarAPI(account)
		return
	}

	ctx := context.Background()

	// Create CalDAV client
	client := caldav.NewClient(account)

	// Authenticate
	if err := client.Authenticate(ctx); err != nil {
		log.Printf("CalDAV auth failed: %v", err)
		return
	}

	// List calendars
	calendars, err := client.ListCalendars(ctx)
	if err != nil {
		log.Printf("Failed to list calendars: %v", err)
		return
	}

	log.Printf("Found %d calendars", len(calendars))

	// Save calendars to store
	for _, cal := range calendars {
		cal.AccountID = account.ID
		if err := app.store.SaveCalendar(cal); err != nil {
			log.Printf("Failed to save calendar %s: %v", cal.Name, err)
			continue
		}

		// Fetch events for this calendar (last 30 days to next 90 days)
		start := time.Now().AddDate(0, -1, 0)
		end := time.Now().AddDate(0, 3, 0)

		events, err := client.GetEvents(ctx, cal.ID, start, end)
		if err != nil {
			log.Printf("Failed to get events for %s: %v", cal.Name, err)
			continue
		}

		log.Printf("Found %d events in %s", len(events), cal.Name)

		// Save events and track IDs
		var activeIDs []string
		for _, event := range events {
			event.CalendarID = cal.ID
			activeIDs = append(activeIDs, event.ID)
			if err := app.store.SaveEvent(event); err != nil {
				log.Printf("Failed to save event %s: %v", event.Title, err)
			}
		}

		// Remove local events that no longer exist on remote
		if deleted, err := app.store.DeleteEventsNotIn(cal.ID, activeIDs); err != nil {
			log.Printf("Failed to clean up deleted events for %s: %v", cal.Name, err)
		} else if deleted > 0 {
			log.Printf("Removed %d deleted events from %s", deleted, cal.Name)
		}
	}

	// Update last sync time
	account.LastSync = time.Now()
	app.store.SaveAccount(account)

	// Refresh UI on main thread
	glib.IdleAdd(func() {
		app.loadCalendars()
		app.refreshMonthView()
	})
}

// syncGoogleCalendarAPI syncs using Google Calendar API directly
func (app *App) syncGoogleCalendarAPI(account *calendar.Account) {
	// Refresh token if expired
	if time.Now().After(account.TokenExpiry) {
		if err := app.refreshGoogleToken(account); err != nil {
			log.Printf("Failed to refresh token: %v", err)
			return
		}
	}

	// Fetch calendar list
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/calendar/v3/users/me/calendarList", nil)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch calendars: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Calendar list error %d: %s", resp.StatusCode, string(body))
		return
	}

	var calList struct {
		Items []struct {
			ID              string `json:"id"`
			Summary         string `json:"summary"`
			Description     string `json:"description"`
			BackgroundColor string `json:"backgroundColor"`
			Primary         bool   `json:"primary"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&calList); err != nil {
		log.Printf("Failed to decode calendar list: %v", err)
		return
	}

	log.Printf("Found %d Google calendars", len(calList.Items))

	for _, item := range calList.Items {
		// Load existing calendar to preserve sync token and visibility
		existing, _ := app.store.GetCalendar(item.ID)

		cal := &calendar.Calendar{
			ID:          item.ID,
			AccountID:   account.ID,
			Name:        item.Summary,
			Description: item.Description,
			Color:       item.BackgroundColor,
			Visible:     true,
		}
		if existing != nil {
			cal.SyncToken = existing.SyncToken
			cal.Visible = existing.Visible
		}
		if cal.Color == "" {
			cal.Color = "#4285f4"
		}

		if err := app.store.SaveCalendar(cal); err != nil {
			log.Printf("Failed to save calendar %s: %v", cal.Name, err)
			continue
		}

		// Fetch events for this calendar
		app.fetchGoogleEvents(account, cal)
	}

	// Update last sync time
	account.LastSync = time.Now()
	app.store.SaveAccount(account)

	// Refresh UI on main thread
	glib.IdleAdd(func() {
		app.loadCalendars()
		app.refreshMonthView()
	})
}

// fetchGoogleEvents fetches events from a Google calendar using incremental sync when possible
func (app *App) fetchGoogleEvents(account *calendar.Account, cal *calendar.Calendar) {
	var eventsURL string
	incremental := cal.SyncToken != "" && cal.SyncToken != "disabled"

	if cal.SyncToken == "disabled" {
		// This calendar doesn't support incremental sync
		app.fetchGoogleEventsFull(account, cal)
		return
	}

	if incremental {
		// Incremental sync: only get changes since last sync
		eventsURL = fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?syncToken=%s",
			url.PathEscape(cal.ID), url.QueryEscape(cal.SyncToken))
	} else {
		// Full sync: fetch everything in time window (no singleEvents so we get a reusable sync token)
		timeMin := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
		timeMax := time.Now().AddDate(0, 3, 0).Format(time.RFC3339)
		eventsURL = fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?timeMin=%s&timeMax=%s&maxResults=2500",
			url.PathEscape(cal.ID), url.QueryEscape(timeMin), url.QueryEscape(timeMax))
	}

	req, _ := http.NewRequest("GET", eventsURL, nil)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch events for %s: %v", cal.Name, err)
		return
	}
	defer resp.Body.Close()

	// 410 Gone means sync token is invalid — permanently disable incremental sync for this calendar
	if resp.StatusCode == 410 && incremental {
		log.Printf("Sync token invalid for %s, disabling incremental sync", cal.Name)
		cal.SyncToken = "disabled"
		app.store.SaveCalendar(cal)
		app.fetchGoogleEventsFull(account, cal)
		return
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Events error %d: %s", resp.StatusCode, string(body))
		return
	}

	var eventList struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
			Status string `json:"status"`
		} `json:"items"`
		NextSyncToken string `json:"nextSyncToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventList); err != nil {
		log.Printf("Failed to decode event list for %s: %v", cal.Name, err)
		return
	}

	if incremental {
		if len(eventList.Items) > 0 {
			log.Printf("Incremental sync: %d changes in %s", len(eventList.Items), cal.Name)
		}
	} else {
		log.Printf("Full sync: %d events in %s", len(eventList.Items), cal.Name)
	}

	var activeIDs []string
	for _, item := range eventList.Items {
		// In incremental sync, cancelled means deleted — remove locally
		if item.Status == "cancelled" {
			if incremental {
				if err := app.store.DeleteEvent(item.ID); err != nil {
					log.Printf("Failed to delete cancelled event %s: %v", item.ID, err)
				}
			}
			continue
		}

		activeIDs = append(activeIDs, item.ID)

		event := &calendar.Event{
			ID:          item.ID,
			CalendarID:  cal.ID,
			UID:         item.ID,
			Title:       item.Summary,
			Description: item.Description,
			Location:    item.Location,
			Status:      calendar.StatusConfirmed,
		}

		// Parse start time
		if item.Start.DateTime != "" {
			event.Start, _ = time.Parse(time.RFC3339, item.Start.DateTime)
		} else if item.Start.Date != "" {
			event.Start, _ = time.Parse("2006-01-02", item.Start.Date)
			event.AllDay = true
		}

		// Parse end time
		if item.End.DateTime != "" {
			event.End, _ = time.Parse(time.RFC3339, item.End.DateTime)
		} else if item.End.Date != "" {
			event.End, _ = time.Parse("2006-01-02", item.End.Date)
		}

		if err := app.store.SaveEvent(event); err != nil {
			log.Printf("Failed to save event %s: %v", event.Title, err)
		}
	}

	// On full sync, remove events that no longer exist remotely
	if !incremental {
		if deleted, err := app.store.DeleteEventsNotIn(cal.ID, activeIDs); err != nil {
			log.Printf("Failed to clean up deleted events for %s: %v", cal.Name, err)
		} else if deleted > 0 {
			log.Printf("Removed %d stale events from %s", deleted, cal.Name)
		}
	}

	// Save sync token for next incremental sync
	if eventList.NextSyncToken != "" {
		cal.SyncToken = eventList.NextSyncToken
		app.store.SaveCalendar(cal)
	}
}

// fetchGoogleEventsFull does a full sync without saving a sync token (for calendars that don't support incremental sync)
func (app *App) fetchGoogleEventsFull(account *calendar.Account, cal *calendar.Calendar) {
	timeMin := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
	timeMax := time.Now().AddDate(0, 3, 0).Format(time.RFC3339)
	eventsURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?timeMin=%s&timeMax=%s&singleEvents=true&maxResults=2500",
		url.PathEscape(cal.ID), url.QueryEscape(timeMin), url.QueryEscape(timeMax))

	req, _ := http.NewRequest("GET", eventsURL, nil)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch events for %s: %v", cal.Name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Events error %d: %s", resp.StatusCode, string(body))
		return
	}

	var eventList struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventList); err != nil {
		log.Printf("Failed to decode event list for %s: %v", cal.Name, err)
		return
	}

	var activeIDs []string
	for _, item := range eventList.Items {
		if item.Status == "cancelled" {
			continue
		}

		activeIDs = append(activeIDs, item.ID)

		event := &calendar.Event{
			ID:          item.ID,
			CalendarID:  cal.ID,
			UID:         item.ID,
			Title:       item.Summary,
			Description: item.Description,
			Location:    item.Location,
			Status:      calendar.StatusConfirmed,
		}

		if item.Start.DateTime != "" {
			event.Start, _ = time.Parse(time.RFC3339, item.Start.DateTime)
		} else if item.Start.Date != "" {
			event.Start, _ = time.Parse("2006-01-02", item.Start.Date)
			event.AllDay = true
		}

		if item.End.DateTime != "" {
			event.End, _ = time.Parse(time.RFC3339, item.End.DateTime)
		} else if item.End.Date != "" {
			event.End, _ = time.Parse("2006-01-02", item.End.Date)
		}

		if err := app.store.SaveEvent(event); err != nil {
			log.Printf("Failed to save event %s: %v", event.Title, err)
		}
	}

	if deleted, err := app.store.DeleteEventsNotIn(cal.ID, activeIDs); err != nil {
		log.Printf("Failed to clean up deleted events for %s: %v", cal.Name, err)
	} else if deleted > 0 {
		log.Printf("Removed %d stale events from %s", deleted, cal.Name)
	}
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

func (app *App) showEventDialog(event *calendar.Event) {
	isNew := event == nil
	if isNew {
		event = &calendar.Event{
			ID:         fmt.Sprintf("evt-%d", time.Now().UnixNano()),
			Start:      time.Date(app.selectedDate.Year(), app.selectedDate.Month(), app.selectedDate.Day(), 9, 0, 0, 0, app.selectedDate.Location()),
			End:        time.Date(app.selectedDate.Year(), app.selectedDate.Month(), app.selectedDate.Day(), 10, 0, 0, 0, app.selectedDate.Location()),
			Status:     calendar.StatusConfirmed,
		}
	}

	dialog := gtk.NewDialog()
	if isNew {
		dialog.SetTitle("New Event")
	} else {
		dialog.SetTitle("Edit Event")
	}
	dialog.SetTransientFor(&app.window.Window)
	dialog.SetModal(true)
	dialog.SetDefaultSize(400, 350)

	content := dialog.ContentArea()
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)
	content.SetSpacing(8)

	// Title
	titleLabel := gtk.NewLabel("Title:")
	titleLabel.SetXAlign(0)
	content.Append(titleLabel)

	titleEntry := gtk.NewEntry()
	titleEntry.SetText(event.Title)
	titleEntry.SetPlaceholderText("Event title")
	content.Append(titleEntry)

	// Calendar selection
	calLabel := gtk.NewLabel("Calendar:")
	calLabel.SetXAlign(0)
	content.Append(calLabel)

	calCombo := gtk.NewComboBoxText()
	calendars, _ := app.store.GetAllCalendars()
	selectedIdx := 0
	for i, cal := range calendars {
		calCombo.AppendText(cal.Name)
		if cal.ID == event.CalendarID {
			selectedIdx = i
		}
	}
	if len(calendars) > 0 {
		calCombo.SetActive(selectedIdx)
	}
	content.Append(calCombo)

	// All day checkbox
	allDayCheck := gtk.NewCheckButtonWithLabel("All day")
	allDayCheck.SetActive(event.AllDay)
	content.Append(allDayCheck)

	// Date - UK format DD/MM/YYYY
	dateLabel := gtk.NewLabel("Date (DD/MM/YYYY):")
	dateLabel.SetXAlign(0)
	content.Append(dateLabel)

	dateEntry := gtk.NewEntry()
	dateEntry.SetText(event.Start.Format("02/01/2006"))
	dateEntry.SetPlaceholderText("DD/MM/YYYY")
	content.Append(dateEntry)

	// Time (start - end)
	timeBox := gtk.NewBox(gtk.OrientationHorizontal, 8)

	startEntry := gtk.NewEntry()
	startEntry.SetText(event.Start.Format(app.config.TimeFormat()))
	startEntry.SetWidthChars(8)
	timeBox.Append(startEntry)

	timeBox.Append(gtk.NewLabel("to"))

	endEntry := gtk.NewEntry()
	endEntry.SetText(event.End.Format(app.config.TimeFormat()))
	endEntry.SetWidthChars(8)
	timeBox.Append(endEntry)

	content.Append(timeBox)

	// Location
	locLabel := gtk.NewLabel("Location:")
	locLabel.SetXAlign(0)
	content.Append(locLabel)

	locEntry := gtk.NewEntry()
	locEntry.SetText(event.Location)
	locEntry.SetPlaceholderText("Add location")
	content.Append(locEntry)

	// Description
	descLabel := gtk.NewLabel("Description:")
	descLabel.SetXAlign(0)
	content.Append(descLabel)

	descEntry := gtk.NewEntry()
	descEntry.SetText(event.Description)
	descEntry.SetPlaceholderText("Add description")
	content.Append(descEntry)

	// Buttons
	btnBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
	btnBox.SetHAlign(gtk.AlignEnd)
	btnBox.SetMarginTop(12)

	if !isNew {
		deleteBtn := gtk.NewButtonWithLabel("Delete")
		deleteBtn.AddCSSClass("destructive-action")
		deleteBtn.ConnectClicked(func() {
			go app.store.DeleteEvent(event.ID)
			app.refreshMonthView()
			app.refreshDayDetail()
			dialog.Close()
		})
		btnBox.Append(deleteBtn)
	}

	cancelBtn := gtk.NewButtonWithLabel("Cancel")
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	btnBox.Append(cancelBtn)

	saveBtn := gtk.NewButtonWithLabel("Save")
	saveBtn.AddCSSClass("suggested-action")
	saveBtn.ConnectClicked(func() {
		// Get values
		event.Title = titleEntry.Text()
		event.Location = locEntry.Text()
		event.Description = descEntry.Text()
		event.AllDay = allDayCheck.Active()

		// Parse date - UK format DD/MM/YYYY
		dateStr := dateEntry.Text()
		if date, err := time.Parse("02/01/2006", dateStr); err == nil {
			// Parse times
			startStr := startEntry.Text()
			endStr := endEntry.Text()

			if startTime, err := time.Parse(app.config.TimeFormat(), startStr); err == nil {
				event.Start = time.Date(date.Year(), date.Month(), date.Day(),
					startTime.Hour(), startTime.Minute(), 0, 0, date.Location())
			}
			if endTime, err := time.Parse(app.config.TimeFormat(), endStr); err == nil {
				event.End = time.Date(date.Year(), date.Month(), date.Day(),
					endTime.Hour(), endTime.Minute(), 0, 0, date.Location())
			}
		}

		// Set calendar
		if len(calendars) > 0 && calCombo.Active() >= 0 {
			event.CalendarID = calendars[calCombo.Active()].ID
		}

		event.Modified = time.Now()
		if isNew {
			event.Created = time.Now()
			event.UID = event.ID
		}

		// Save
		if err := app.store.SaveEvent(event); err != nil {
			log.Printf("Error saving event: %v", err)
			return
		}

		app.refreshMonthView()
		app.refreshDayDetail()
		dialog.Close()
	})
	btnBox.Append(saveBtn)

	content.Append(btnBox)
	dialog.Show()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// updateLayoutForWidth adjusts panel visibility and sizing based on window width
func (app *App) updateLayoutForWidth() {
	width := app.window.Width()

	// Skip if window not yet realized
	if width <= 0 {
		return
	}

	if width < 600 {
		// Compact: calendar only, no panels
		app.setSidebarVisible(false)
		app.setCalendarVisible(true)
		app.setDayDetailVisible(false)
	} else if width < 1000 {
		// Medium: calendar + day detail, no sidebar
		app.setSidebarVisible(false)
		app.setCalendarVisible(true)
		app.setDayDetailVisible(true)
		app.contentPaned.SetPosition(width * 6 / 10)
	} else {
		// Large: all three panels
		app.setSidebarVisible(true)
		app.setCalendarVisible(true)
		app.setDayDetailVisible(true)
		app.contentPaned.SetPosition((width - 200) * 6 / 10)
	}
}

// setSidebarVisible shows or hides the sidebar
func (app *App) setSidebarVisible(visible bool) {
	app.sidebarVisible = visible

	if app.sidebar != nil {
		app.sidebar.SetVisible(visible)
	}
	if app.sidebarSep != nil {
		app.sidebarSep.SetVisible(visible)
	}

	// Sync toggle button state without triggering callback
	if app.sidebarBtn != nil && app.sidebarBtn.Active() != visible {
		app.sidebarBtn.SetActive(visible)
	}
}

// setDayDetailVisible shows or hides the day detail panel
func (app *App) setDayDetailVisible(visible bool) {
	app.dayDetailVisible = visible

	if app.dayDetail != nil {
		app.dayDetail.SetVisible(visible)
	}

	// Sync toggle button state without triggering callback
	if app.detailBtn != nil && app.detailBtn.Active() != visible {
		app.detailBtn.SetActive(visible)
	}
}

// setCalendarVisible shows or hides the calendar (month view)
func (app *App) setCalendarVisible(visible bool) {
	if app.monthBox != nil {
		app.monthBox.SetVisible(visible)
	}
}
