package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/MrPuls/local-ci/internal/runmanager"
	"github.com/MrPuls/local-ci/internal/server"
	"github.com/spf13/cobra"
)

var (
	serveHost   string
	servePort   int
	serveToken  string
	serveConfig string
)

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the local-ci backend server (HTTP API + live event stream)",
		Long:  "Starts a loopback HTTP server exposing run history, the config graph, and live run control (trigger/cancel/observe) for the desktop UI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			token := serveToken
			if token == "" {
				token = randomToken()
			}
			// The server operates on a single project config, fixed here from a
			// trusted flag (relative to the working directory) — never from a
			// request. Runs also execute in this working directory.
			configPath, err := filepath.Abs(serveConfig)
			if err != nil {
				return err
			}
			mgr := runmanager.New(st)
			srv := server.New(st, mgr, token, version, configPath)

			addr := net.JoinHostPort(serveHost, strconv.Itoa(servePort))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", addr, err)
			}

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

			fmt.Printf("local-ci serve listening on http://%s (token: %s)\n", ln.Addr().String(), token)
			if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Host/interface to bind (loopback by default)")
	cmd.Flags().IntVar(&servePort, "port", 0, "Port to bind (0 = pick an ephemeral port)")
	cmd.Flags().StringVar(&serveToken, "token", "", "Bearer token clients must present (default: a random per-launch token)")
	cmd.Flags().StringVarP(&serveConfig, "config", "c", ".local-ci.yaml", "Project config file the server operates on (relative to the working directory)")
	return cmd
}

func randomToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
