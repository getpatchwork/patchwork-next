// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/api"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
	"github.com/getpatchwork/patchwork/pkg/events"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/web"
)

type CLI struct{}

func (c *CLI) Run(ctx *pw.Context) error {
	if ctx.Config.Database.AutoSync {
		if err := migrations.RunMigrations(ctx, ctx.DB); err != nil {
			return err
		}
	} else if err := migrations.CheckSchemaVersion(ctx, ctx.DB); err != nil {
		return err
	}

	bus := events.Start(ctx, ctx.DB)
	defer bus.Shutdown()

	router := web.NewRouter(ctx.Config, ctx.DB, bus)
	router.Mount("/", api.NewRouter(ctx.Config, ctx.DB, ctx.Config.Http.BaseURL, bus))

	srv := &http.Server{
		Addr:     ctx.Config.Http.Listen,
		Handler:  router,
		ErrorLog: log.ErrLogger(),
	}
	sock, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Noticef("patchwork %s listening on http://%s", ctx.Version, sock.Addr())
		if e := srv.Serve(sock); e != nil && e != http.ErrServerClosed {
			err = fmt.Errorf("serve: %w", e)
			done <- syscall.SIGCHLD
		}
	}()

	sig := <-done
	if sig != syscall.SIGCHLD {
		log.Noticef("received signal %v, shutting down", sig)
		timeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if e := srv.Shutdown(timeout); e != nil {
			err = fmt.Errorf("shutdown: %w", e)
		}
	}

	return err
}
