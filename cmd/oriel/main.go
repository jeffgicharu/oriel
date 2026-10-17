// Command oriel is the Oriel website and sales desk.
//
//	oriel serve                 run the web server (settings come from the environment)
//	oriel version               print the release's commit id
//	oriel check-release ...     walk a running release before it goes live
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	// Nairobi's time zone travels inside the binary.
	_ "time/tzdata"

	"github.com/jeffgicharu/oriel/internal/app"
	"github.com/jeffgicharu/oriel/internal/assets"
	"github.com/jeffgicharu/oriel/internal/check"
	"github.com/jeffgicharu/oriel/internal/config"
)

// release is the commit id, set when the binary is built.
var release = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(64)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve()
	case "version":
		fmt.Println(release)
	case "check-release":
		err = checkRelease(os.Args[2:])
	default:
		usage()
		os.Exit(64)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "oriel:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: oriel serve | version | check-release [--env-file F] --base URL --release SHA | check-release --print-paths")
}

func serve() error {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return err
	}
	if err := assets.Check("css/site.css", "js/site.js", "css/desk.css", "js/desk.js", "fonts/MonaSans.woff2", "img/og-oriel.png"); err != nil {
		return err
	}
	application, err := app.New(cfg, release, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	srv.RegisterOnShutdown(application.Close)

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	log.Info("serving", "addr", cfg.Addr, "release", release)

	stopped := make(chan error, 1)
	go func() { stopped <- srv.Serve(listener) }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	select {
	case err := <-stopped:
		return err
	case sig := <-signals:
		log.Info("stopping", "signal", sig.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	application.Shutdown()
	return nil
}

func checkRelease(args []string) error {
	fs := flag.NewFlagSet("check-release", flag.ContinueOnError)
	envFile := fs.String("env-file", "", "read settings from this KEY=VALUE file")
	base := fs.String("base", "", "address of the release to check, such as http://127.0.0.1:8401")
	want := fs.String("release", "", "the commit id the release must report")
	printPaths := fs.Bool("print-paths", false, "print the public pages to check through nginx, one per line")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *printPaths {
		fmt.Println(strings.Join(check.PublicPaths(), "\n"))
		return nil
	}
	if *envFile != "" {
		if err := config.LoadEnvFile(*envFile); err != nil {
			return err
		}
	}
	if *base == "" || *want == "" {
		return errors.New("check-release needs --base and --release")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return check.New(*base, *want, os.Stdout).Run(ctx)
}
