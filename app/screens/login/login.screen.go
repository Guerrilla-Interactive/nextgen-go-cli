package login

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	appUtils "github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils"
	config "github.com/Guerrilla-Interactive/nextgen-go-cli/internal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoginCompletedMsg is emitted when the browser flow returns to the local callback.
type LoginCompletedMsg struct {
	token string
	err   error
}

// FetchUserCompletedMsg indicates the async /api/me call finished.
type FetchUserCompletedMsg struct {
	user appUtils.MeUser
	err  error
}

// UpdateScreenLogin handles key events for the login screen.
func UpdateScreenLogin(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter":
		return m, startBrowserLoginFlow()
	}
	return m, nil
}

// ViewScreenLogin renders the login screen content.
func ViewScreenLogin(m app.Model) string {
	// Simple instructions with styling consistent with other screens
	title := app.TitleStyle.Render("Login")
	body := "\nOpening your browser to sign in with Clerk...\n" +
		"After signing in, you'll be redirected back to complete login.\n\n" +
		"Instance: " + app.LinkStyle.Render("https://www.nextgen-cli.com/sign-in") + "\n" +
		"Callback: " + app.PathStyle.Render("http://localhost:4455/callback") + "\n\n" +
		sharedScreens.Footer("Enter start login", "ctrl+c quit")
	// Append status if available
	if m.HistorySaveStatus != "" {
		body += "\n" + app.HelpStyle.Render(m.HistorySaveStatus)
	}
	panel := lipgloss.NewStyle().Padding(1, 2).Margin(1).Render(title + "\n" + body)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, panel)
	}
	return panel
}

// startBrowserLoginFlow opens the hosted Clerk sign-in page and starts a local callback server.
func startBrowserLoginFlow() tea.Cmd {
	return func() tea.Msg {
		// Start a local HTTP server to capture the callback
		tokenCh := make(chan string, 1)
		errCh := make(chan error, 1)

		mux := http.NewServeMux()
		server := &http.Server{Addr: ":4455", Handler: mux}

		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			// Try a few common param names
			token := q.Get("token")
			if token == "" {
				token = q.Get("session")
			}
			if token == "" {
				token = q.Get("session_id")
			}
			// Clerk Desktop Browser / hosted flows may return different param names
			if token == "" {
				token = q.Get("__clerk_db_jwt")
			}
			if token == "" {
				token = q.Get("__session")
			}
			if token == "" {
				token = q.Get("clerk_token")
			}
			if token == "" {
				token = q.Get("jwt")
			}
			fmt.Fprintln(w, "Login complete. You may close this window and return to the CLI.")
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = server.Shutdown(ctx)
			}()
			tokenCh <- token
		})

		// Run server in background
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}()

		// Open your app's cli-bridge which will fetch a Clerk token and redirect to our callback
		// Normalize to scheme+host only in case NEXTGEN_BASE_URL includes a path like /sign-in
		base := appUtils.GetBaseURL()
		hostBase := base
		if u, err := url.Parse(base); err == nil && u.Scheme != "" && u.Host != "" {
			hostBase = u.Scheme + "://" + u.Host
		}
		loginURL := hostBase + "/cli-bridge?redirect=" + url.QueryEscape("http://localhost:4455/callback")

		// Open the browser
		if err := openBrowser(loginURL); err != nil {
			return LoginCompletedMsg{"", fmt.Errorf("failed to open browser: %w", err)}
		}

		// Wait for token or error/timeout
		select {
		case err := <-errCh:
			return LoginCompletedMsg{"", err}
		case token := <-tokenCh:
			if token == "" {
				return LoginCompletedMsg{"", fmt.Errorf("no token received in callback")}
			}
			// Persist to config; user details will be fetched asynchronously in Update
			cfg, _ := config.LoadConfig()
			cfg.IsLoggedIn = true
			cfg.Token = token
			_ = config.SaveConfig(cfg)
			return LoginCompletedMsg{token, nil}
		case <-time.After(2 * time.Minute):
			return LoginCompletedMsg{"", fmt.Errorf("login timed out waiting for callback")}
		}
	}
}

// On receiving loginCompletedMsg, toggle model state; exported helper to be used by Update.
func HandleLoginMsg(m app.Model, msg tea.Msg) (app.Model, tea.Cmd) {
	if lm, ok := msg.(LoginCompletedMsg); ok {
		if lm.err != nil {
			// Keep user on screen; show a minimal error line next render by setting status in HistorySaveStatus
			m.HistorySaveStatus = fmt.Sprintf("Login error: %v", lm.err)
			return m, nil
		}
		// Set flag and show retrieving status, then fetch user asynchronously
		m.IsLoggedIn = true
		m.HistorySaveStatus = "Retrieving account details..."
		token := lm.token
		return m, func() tea.Msg {
			me, err := appUtils.FetchMe(token)
			if err != nil {
				return FetchUserCompletedMsg{err: err}
			}
			return FetchUserCompletedMsg{user: me.User}
		}
	}

	if fm, ok := msg.(FetchUserCompletedMsg); ok {
		if fm.err != nil {
			m.HistorySaveStatus = fmt.Sprintf("Login ok, but failed to fetch account: %v", fm.err)
			m.CurrentScreen = app.ScreenMain
			return m, nil
		}
		if fm.user.ID != "" {
			cfg, _ := config.LoadConfig()
			cfg.UserID = fm.user.ID
			_ = config.SaveConfig(cfg)
		}
		displayName := ""
		if fm.user.FirstName != nil && *fm.user.FirstName != "" {
			displayName = *fm.user.FirstName
		} else if fm.user.Email != "" {
			displayName = fm.user.Email
		} else if fm.user.ID != "" {
			displayName = fm.user.ID
		}
		if displayName != "" {
			m.HistorySaveStatus = fmt.Sprintf("Welcome, %s!", displayName)
		} else {
			m.HistorySaveStatus = "Welcome!"
		}
		m.CurrentScreen = app.ScreenMain
		return m, nil
	}
	return m, nil
}

// openBrowser tries to open the URL in the default browser across platforms.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// StartLoginFlowCmd is an exported wrapper to initiate the login flow immediately.
func StartLoginFlowCmd() tea.Cmd {
	return startBrowserLoginFlow()
}
