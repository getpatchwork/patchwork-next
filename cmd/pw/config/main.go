// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/config"
)

type PrintCmd struct{}

type URLCmd struct{}

type CLI struct {
	Print PrintCmd `cmd:"" default:"withargs" help:"Print default configuration to stdout."`
	URL   URLCmd   `cmd:"" help:"Generate a database connection URL."`
}

func (c *PrintCmd) Run(ctx *pw.Context) error {
	return config.Generate(ctx.Config, os.Stdout)
}

func (c *URLCmd) Run(*pw.Context) error {
	reader := bufio.NewReader(os.Stdin)

	scheme, err := prompt(reader, "Database type (postgres, mysql, sqlite)", "postgres")
	if err != nil {
		return err
	}

	switch scheme {
	case "sqlite":
		return c.sqliteURL(reader)
	case "postgres", "mysql":
		return c.networkURL(reader, scheme)
	default:
		return fmt.Errorf("unsupported database type: %s", scheme)
	}
}

func (c *URLCmd) sqliteURL(reader *bufio.Reader) error {
	path, err := prompt(reader, "Database file path", "patchwork.db")
	if err != nil {
		return err
	}
	fmt.Printf("\nurl = %q\n", "sqlite://"+path)
	return nil
}

func (c *URLCmd) networkURL(reader *bufio.Reader, scheme string) error {
	defaultPort := "5432"
	if scheme == "mysql" {
		defaultPort = "3306"
	}

	host, err := prompt(reader, "Host", "localhost")
	if err != nil {
		return err
	}
	port, err := prompt(reader, "Port", defaultPort)
	if err != nil {
		return err
	}
	dbname, err := prompt(reader, "Database name", "patchwork")
	if err != nil {
		return err
	}
	user, err := prompt(reader, "Username", "patchwork")
	if err != nil {
		return err
	}
	password, err := promptPassword("Password: ")
	if err != nil {
		return err
	}

	u := &url.URL{
		Scheme: scheme,
		User:   url.UserPassword(user, password),
		Host:   host,
		Path:   dbname,
	}
	if port != defaultPort {
		u.Host += ":" + port
	}

	fmt.Printf("\nurl = %q\n", u.String())
	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

func promptPassword(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(password), nil
}
