package google

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
)

// OAuthCallbackServer handles OAuth redirects
type OAuthCallbackServer struct {
	server   *http.Server
	listener net.Listener
	port     int
	codeChan chan string
	errChan  chan error
	mu       sync.Mutex
}

// NewOAuthCallbackServer creates a new callback server
func NewOAuthCallbackServer() (*OAuthCallbackServer, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	s := &OAuthCallbackServer{
		listener: listener,
		port:     port,
		codeChan: make(chan string, 1),
		errChan:  make(chan error, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)

	s.server = &http.Server{
		Handler: mux,
	}

	return s, nil
}

// Start starts the callback server
func (s *OAuthCallbackServer) Start() {
	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.Printf("OAuth callback server error: %v", err)
		}
	}()
}

// Stop stops the callback server
func (s *OAuthCallbackServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		s.server.Shutdown(context.Background())
	}
}

// GetRedirectURL returns the callback URL
func (s *OAuthCallbackServer) GetRedirectURL() string {
	return fmt.Sprintf("http://localhost:%d/callback", s.port)
}

// WaitForCode waits for the authorization code
func (s *OAuthCallbackServer) WaitForCode(ctx context.Context) (string, error) {
	select {
	case code := <-s.codeChan:
		return code, nil
	case err := <-s.errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *OAuthCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "no authorization code received"
		}
		s.errChan <- fmt.Errorf("OAuth error: %s", errMsg)
		http.Error(w, "Authorization failed", http.StatusBadRequest)
		return
	}

	s.codeChan <- code

	// Show success page
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head>
	<title>UniCal - Authorization Successful</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
			color: white;
		}
		.container {
			text-align: center;
			padding: 40px;
			background: rgba(255,255,255,0.1);
			border-radius: 16px;
			backdrop-filter: blur(10px);
		}
		h1 { font-size: 2em; margin-bottom: 16px; }
		p { font-size: 1.2em; opacity: 0.9; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Authorization Successful!</h1>
		<p>You can close this window and return to UniCal.</p>
	</div>
</body>
</html>
`)
}
