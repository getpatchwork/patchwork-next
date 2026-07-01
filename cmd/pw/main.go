// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/alecthomas/kong"

	"github.com/getpatchwork/patchwork/cmd/pw/admin"
	pwdb "github.com/getpatchwork/patchwork/cmd/pw/db"
	"github.com/getpatchwork/patchwork/cmd/pw/http"
	"github.com/getpatchwork/patchwork/cmd/pw/ingress"
	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type CLI struct {
	config.Config

	ShowVersion VersionFlag `name:"version" help:"Print patchwork version and exit."`

	GenCfg  struct{}    `cmd:"" name:"config" help:"Print default configuration to stdout."`
	Admin   admin.CLI   `cmd:"" help:"Administration CLI."`
	DB      pwdb.CLI    `cmd:"" name:"db" help:"Database management."`
	Ingress ingress.CLI `cmd:"" help:"Ingress SMTP/LMTP daemon."`
	Http    http.CLI    `cmd:"" help:"HTTP server daemon."`
}

// set at build time
var (
	Version string
	Date    string
)

type VersionFlag bool

func (v VersionFlag) BeforeReset(app *kong.Kong, vars kong.Vars) error {
	fmt.Printf("patchwork %s (%s %s %s %s)\n",
		Version, runtime.Version(), runtime.GOARCH, runtime.GOOS, Date)
	app.Exit(0)
	return nil
}

func main() {
	var cli CLI

	config.RegisterHints("events", admin.EventCategories())

	k := config.Parse(&cli, "Patchwork runtime commands.")

	if k.Command() == "config" {
		k.FatalIfErrorf(config.Generate(&cli.Config, os.Stdout))
		return
	}

	if cli.Syslog {
		log.InitSyslog("pw-" + k.Command())
		k.Stderr = log.ErrLogger().Writer()
	}

	database, err := db.Open(&cli.Config)
	k.FatalIfErrorf(err, "database")
	defer database.Close()

	k.FatalIfErrorf(k.Run(&pw.Context{
		Context: context.Background(),
		Config:  &cli.Config,
		DB:      database,
		Version: Version,
	}))
}
