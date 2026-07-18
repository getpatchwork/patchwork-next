// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"os"

	"github.com/alecthomas/kong"
	kongtoml "github.com/alecthomas/kong-toml"
)

type Config struct {
	Syslog   bool           `short:"S" help:"Redirect logging to syslog."`
	Database DatabaseConfig `embed:"" prefix:"database-"`
	Ingress  IngressConfig  `embed:"" prefix:"ingress-"`
	Http     HttpConfig     `embed:"" prefix:"http-"`
	SMTP     SMTPConfig     `embed:"" prefix:"smtp-"`
}

type DatabaseConfig struct {
	URL         string   `help:"Database connection URL." completion:"sqlite://,postgres://,mysql://"`
	AutoSync    bool     `help:"Automatically run pending migrations at startup."`
	EventMaxAge Duration `help:"Maximum age of events. Used by pw admin gc." default:"30d"`
}

type IngressConfig struct {
	Listen string `help:"SMTP listen address." default:"127.0.0.1:2525"`
}

type HttpConfig struct {
	BaseURL     string   `help:"Base URL of the patchwork instance."`
	Listen      string   `help:"HTTP listen address." default:"127.0.0.1:8080"`
	SlowRequest Duration `help:"Log HTTP requests slower than this threshold."`
	SlowQuery   Duration `help:"Log SQL queries slower than this threshold."`
	CustomCSS   string   `help:"Path to a custom CSS file served after the built-in stylesheet." type:"existingfile"`
	NavHTML     string   `help:"Path to an HTML file whose content is inserted in the navigation bar." type:"existingfile"`
	FooterHTML  string   `help:"Path to an HTML file whose content is inserted in the footer." type:"existingfile"`
	WebPageSize int      `help:"Default number of items per page in the web interface." default:"200"`
	WebPageMax  int      `help:"Maximum number of items per page in the web interface." default:"500"`
	ApiPageSize int      `help:"Default number of items per page in the API." default:"30"`
	ApiPageMax  int      `help:"Maximum number of items per page in the API." default:"250"`
}

type SMTPConfig struct {
	Encryption string `help:"SMTP encryption" default:"none" enum:"none,starttls,tls"`
	Host       string `help:"SMTP server hostname." default:"localhost"`
	Port       int    `help:"SMTP server port." default:"25"`
	User       string `help:"SMTP authentication username."`
	Password   string `help:"SMTP authentication password."`
	From       string `help:"Sender email address for outgoing mail." default:"patchwork@localhost"`
}

const commonDescription = `

Configuration is loaded from:

	/etc/patchwork.toml
	patchwork.toml (current directory)
	$PATCHWORK_TOML

In that order. Settings from later files override earlier ones. CLI
flags take precedence over any configuration file settings.
`

func Parse(cfg any, description string) *kong.Context {
	paths := []string{
		"/etc/patchwork.toml",
		"patchwork.toml",
	}
	p := os.Getenv("PATCHWORK_TOML")
	if p != "" {
		paths = append(paths, p)
	}

	app, err := kong.New(
		cfg,
		kong.Description(description+commonDescription),
		kong.Configuration(kongtoml.Loader, paths...),
	)
	if err != nil {
		panic(err)
	}

	if bashComplete(app) {
		os.Exit(0)
	}

	ctx, err := app.Parse(os.Args[1:])
	app.FatalIfErrorf(err)
	return ctx
}
