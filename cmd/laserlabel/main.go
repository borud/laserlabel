// Package main is the laserlabel command.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/borud/laserlabel/pkg/cli/laserlabel"
)

// opt are the parsed command line options
var opt laserlabel.Options

func main() {
	ktx := kong.Parse(&opt,
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}),
	)

	slog.SetDefault(newLogger(opt.LogLevel, opt.LogFormat))

	err := ktx.Run(&opt)
	if err != nil {
		fmt.Printf("\nError: %v\n\n", err)
		os.Exit(1)
	}
}

func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	err := lvl.UnmarshalText([]byte(level))
	if err != nil {
		lvl = slog.LevelInfo
	}

	hopts := &slog.HandlerOptions{Level: lvl}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, hopts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, hopts))
}
