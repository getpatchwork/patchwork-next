// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"os"

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

	GenCfg  struct{}    `cmd:"" name:"config" help:"Print default configuration to stdout."`
	Admin   admin.CLI   `cmd:"" help:"Administration CLI."`
	DB      pwdb.CLI    `cmd:"" name:"db" help:"Database management."`
	Ingress ingress.CLI `cmd:"" help:"Ingress SMTP/LMTP daemon."`
	Http    http.CLI    `cmd:"" help:"HTTP server daemon."`
}

func main() {
	var cli CLI

	k := config.Parse(&cli, "Patchwork runtime commands.")

	if k.Command() == "config" {
		k.FatalIfErrorf(config.Generate(&cli.Config, os.Stdout))
		return
	}

	if cli.Syslog {
		log.InitSyslog()
		k.Stderr = log.ErrLogger().Writer()
	}

	database, err := db.Open(&cli.Config)
	k.FatalIfErrorf(err, "database")
	defer database.Close()

	k.FatalIfErrorf(k.Run(&pw.Context{
		Context: context.Background(),
		Config:  &cli.Config,
		DB:      database,
	}))
}
