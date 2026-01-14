package gnome

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// OnlineAccount represents a GNOME Online Account
type OnlineAccount struct {
	ID           string
	ProviderType string
	ProviderName string
	Identity     string // email
	CalendarURL  string
}

// GetGoogleAccounts returns Google accounts from GNOME Online Accounts
func GetGoogleAccounts() ([]*OnlineAccount, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	// Get account manager
	obj := conn.Object("org.gnome.OnlineAccounts", "/org/gnome/OnlineAccounts")

	// List all accounts
	var accountPaths []dbus.ObjectPath
	err = obj.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&accountPaths)
	if err != nil {
		// Try alternative method - enumerate child objects
		node, err := introspectNode(conn, "/org/gnome/OnlineAccounts")
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}
		accountPaths = node
	}

	var accounts []*OnlineAccount
	for _, path := range accountPaths {
		account, err := getAccountInfo(conn, path)
		if err != nil {
			continue
		}
		if account.ProviderType == "google" {
			accounts = append(accounts, account)
		}
	}

	return accounts, nil
}

func introspectNode(conn *dbus.Conn, path string) ([]dbus.ObjectPath, error) {
	obj := conn.Object("org.gnome.OnlineAccounts", dbus.ObjectPath(path))

	var xmlData string
	err := obj.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xmlData)
	if err != nil {
		return nil, err
	}

	// Parse XML to find child nodes (simplified - just look for account_*)
	var paths []dbus.ObjectPath
	// For GNOME Online Accounts, accounts are at /org/gnome/OnlineAccounts/Accounts/account_*

	accountsObj := conn.Object("org.gnome.OnlineAccounts", "/org/gnome/OnlineAccounts/Accounts")
	err = accountsObj.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xmlData)
	if err != nil {
		return nil, err
	}

	// Simple parsing - find account paths
	for i := 0; i < 100; i++ {
		testPath := dbus.ObjectPath(fmt.Sprintf("/org/gnome/OnlineAccounts/Accounts/account_%d", i))
		testObj := conn.Object("org.gnome.OnlineAccounts", testPath)

		var providerType string
		err := testObj.Call("org.freedesktop.DBus.Properties.Get",
			0, "org.gnome.OnlineAccounts.Account", "ProviderType").Store(&providerType)
		if err == nil {
			paths = append(paths, testPath)
		}
	}

	return paths, nil
}

func getAccountInfo(conn *dbus.Conn, path dbus.ObjectPath) (*OnlineAccount, error) {
	obj := conn.Object("org.gnome.OnlineAccounts", path)

	account := &OnlineAccount{
		ID: string(path),
	}

	// Get ProviderType
	var providerType dbus.Variant
	err := obj.Call("org.freedesktop.DBus.Properties.Get",
		0, "org.gnome.OnlineAccounts.Account", "ProviderType").Store(&providerType)
	if err != nil {
		return nil, err
	}
	account.ProviderType = providerType.Value().(string)

	// Get ProviderName
	var providerName dbus.Variant
	err = obj.Call("org.freedesktop.DBus.Properties.Get",
		0, "org.gnome.OnlineAccounts.Account", "ProviderName").Store(&providerName)
	if err == nil {
		account.ProviderName = providerName.Value().(string)
	}

	// Get Identity (email)
	var identity dbus.Variant
	err = obj.Call("org.freedesktop.DBus.Properties.Get",
		0, "org.gnome.OnlineAccounts.Account", "Identity").Store(&identity)
	if err == nil {
		account.Identity = identity.Value().(string)
	}

	// Get Calendar URL if available
	var calendarURL dbus.Variant
	err = obj.Call("org.freedesktop.DBus.Properties.Get",
		0, "org.gnome.OnlineAccounts.Calendar", "Uri").Store(&calendarURL)
	if err == nil {
		account.CalendarURL = calendarURL.Value().(string)
	}

	return account, nil
}

// GetOAuthToken gets the OAuth access token for an account
func GetOAuthToken(accountPath string) (string, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return "", fmt.Errorf("failed to connect to session bus: %w", err)
	}

	obj := conn.Object("org.gnome.OnlineAccounts", dbus.ObjectPath(accountPath))

	var token string
	err = obj.Call("org.gnome.OnlineAccounts.OAuthBased.GetAccessToken", 0).Store(&token)
	if err != nil {
		// Try OAuth2Based interface
		var accessToken string
		var expiresIn int32
		err = obj.Call("org.gnome.OnlineAccounts.OAuth2Based.GetAccessToken", 0).Store(&accessToken, &expiresIn)
		if err != nil {
			return "", fmt.Errorf("failed to get access token: %w", err)
		}
		return accessToken, nil
	}

	return token, nil
}
