package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/MrPuls/local-ci/internal/runmanager"
	"github.com/MrPuls/local-ci/internal/server"
	"github.com/MrPuls/local-ci/internal/web"
	"github.com/spf13/cobra"
)

var (
	uiHost   string
	uiPort   int
	uiConfig string
	uiNoOpen bool
)

func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Serve the web UI + API from this binary and open it in a browser",
		Long: "Starts a single loopback server that serves the embedded web UI and the " +
			"JSON/SSE API from one process, then opens it in your browser — no separate " +
			"dev server or token to manage.",
		RunE: func(cmd *cobra.Command, args []string) error {
			uiFS, err := web.Dist()
			if err != nil {
				return fmt.Errorf("load embedded UI: %w", err)
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			// The UI operates on a single project config, fixed here from a
			// trusted flag (relative to the working directory) — never from a
			// request. Runs also execute in this working directory.
			configPath, err := filepath.Abs(uiConfig)
			if err != nil {
				return err
			}
			mgr := runmanager.New(st)
			// Loopback + same-origin: no bearer token. The SPA calls /api on the
			// same origin and the server binds 127.0.0.1 only.
			srv := server.New(st, mgr, "", version, configPath)
			srv.SetUI(uiFS)

			addr := net.JoinHostPort(uiHost, strconv.Itoa(uiPort))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", addr, err)
			}
			url := "http://" + ln.Addr().String() + "/"

			httpSrv := &http.Server{Handler: srv.Handler()}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				// Cancel runs first so the engine cleans up containers and SSE
				// streams unblock, then drain the HTTP server.
				mgr.Shutdown(shutdownCtx)
				httpSrv.Shutdown(shutdownCtx)
			}()

			fmt.Printf("local-ci ui listening on %s\n", url)
			if !uiNoOpen {
				openBrowser(url)
			}
			if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&uiHost, "host", "127.0.0.1", "Host/interface to bind (loopback by default)")
	cmd.Flags().IntVar(&uiPort, "port", 4123, "Port to bind")
	cmd.Flags().StringVarP(&uiConfig, "config", "c", ".local-ci.yaml", "Project config file the UI operates on (relative to the working directory)")
	cmd.Flags().BoolVar(&uiNoOpen, "no-open", false, "Do not open a browser automatically")
	return cmd
}

// openBrowser best-effort opens url in the platform default browser; failures
// are ignored (the URL is also printed for the user to open manually).
func openBrowser(url string) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	_ = exec.Command(name, args...).Start()
}
