package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/htmlviewer"
)

// cmdServe implements `docops serve [--port N] [--open] [--json]`.
//
// Serves the embedded SPA plus the current repo's index/state/raw bodies
// over localhost. Index is rebuilt on every `/index.json` request — no
// file watcher, no SSE. A browser reload refreshes the view.
//
// Exit codes:
//
//	0  clean shutdown (SIGINT / SIGTERM)
//	1  runtime error (port in use, no docs.yaml, listen failed)
//	2  usage error
func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", 8484, "port to listen on (0 for auto)")
	fs.IntVar(port, "p", 8484, "port (shorthand)")
	open := fs.Bool("open", false, "open the default browser after startup")
	asJSON := fs.Bool("json", false, "emit JSON summary to stdout")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops serve [--port N] [--open] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops serve: %v\n", err)
		return 1
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops serve: no docops.yaml found — run `docops init` first")
			return 1
		}
		fmt.Fprintf(os.Stderr, "docops serve: %v\n", err)
		return 1
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops serve: listen %s: %v\n", addr, err)
		return 1
	}
	boundAddr := ln.Addr().String()
	url := "http://" + boundAddr + "/"

	srv := &http.Server{
		Handler:           htmlviewer.Handler(root, cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Banner.
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(map[string]any{"url": url, "addr": boundAddr})
	} else {
		fmt.Fprintf(os.Stderr, "docops serve: %s  (ctrl-c to stop)\n", url)
	}

	if *open {
		if err := openInBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "docops serve: could not open browser: %v\n", err)
		}
	}

	// Signal handling → graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		_ = srv.Shutdown(shutdownCtx)
		fmt.Fprintln(os.Stderr, "docops serve: stopped")
		return 0
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "docops serve: %v\n", err)
			return 1
		}
		return 0
	}
}

func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
