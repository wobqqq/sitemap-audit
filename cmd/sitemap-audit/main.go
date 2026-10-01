// Command sitemap-audit audits robots.txt, sitemaps and every URL they list.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/wobqqq/sitemap-audit/internal/cli"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	app := &cli.App{Stdout: os.Stdout, Stderr: os.Stderr, Version: buildVersion()}
	code := app.Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
