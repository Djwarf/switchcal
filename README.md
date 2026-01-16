# SwitchCal

A GTK4 calendar application for Linux with Google Calendar sync.

## Features

- Google Calendar sync via OAuth
- Responsive UI with collapsible sidebar and day detail panels
- Collapsible calendar groups by account
- Waybar integration for status bar
- Local calendar support
- CalDAV support (iCloud, generic servers)

## Installation

### Arch Linux (AUR)

```bash
yay -S switchcal
```

### Build from source

Requirements:
- Go 1.21+
- GTK4
- GLib2

```bash
git clone https://github.com/Djwarf/switchcal.git
cd switchcal
go build -o switchcal ./cmd/switchcal
sudo install -Dm755 switchcal /usr/bin/switchcal
```

## Usage

Run the application:

```bash
switchcal
```

### Waybar integration

Add to your waybar config:

```json
"custom/calendar": {
    "exec": "switchcal --waybar",
    "return-type": "json",
    "interval": 60,
    "on-click": "switchcal"
}
```

## Adding Google Calendar

1. Click "+ Add Account" in the sidebar
2. Select "Google Calendar (CalDAV)"
3. Sign in with your Google account in the browser
4. Your calendars will sync automatically

## License

MIT
