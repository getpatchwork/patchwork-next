# Patchwork - automated patch tracking system
# Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
#
# SPDX-License-Identifier: GPL-2.0-or-later

PATCHWORK_VERSION ?= $(shell git describe --long --abbrev=8 --dirty 2>/dev/null || echo v4.0.0-rc1)

GO ?= go
V ?= 0
ifeq ($V,1)
Q =
else
Q = @
endif

.PHONY: all
all: pw

src = $(shell git ls-files ':!:*_templ.go' '*.go' '*.css' '*.templ')

pw: $(src)
	$(GO) generate ./...
	$(GO) build -trimpath -o pw ./cmd/pw

prefix ?= /usr
sysconfdir ?= /etc
unitdir ?= /usr/lib/systemd/system

.PHONY: install
install: pw
	install -Dm755 pw $(DESTDIR)$(prefix)/bin/pw
	install -Dm644 etc/pw-http.service $(DESTDIR)$(unitdir)/pw-http.service
	install -Dm644 etc/pw-ingress.service $(DESTDIR)$(unitdir)/pw-ingress.service
	install -Dm644 etc/nginx.conf $(DESTDIR)$(sysconfdir)/nginx/conf.d/patchwork.conf
	$(DESTDIR)$(prefix)/bin/pw config > $(DESTDIR)$(sysconfdir)/patchwork.toml

PYTHON ?= python3

.PHONY: docs
docs:
	$Q if ! [ -x docs/.venv/bin/sphinx-build ]; then \
		set -xe && \
		$(PYTHON) -m venv docs/.venv && \
		docs/.venv/bin/pip install -q -r docs/requirements.txt; \
	fi
	docs/.venv/bin/sphinx-build -b html docs docs/_build

import_reviser ?= github.com/incu6us/goimports-reviser/v3@v3.12.6
import_reviser_flags ?= -rm-unused -project-name github.com/getpatchwork/patchwork
gofumpt ?= mvdan.cc/gofumpt@v0.9.2
golangci_lint ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
license_exclude = ':!:*.md' ':!:*.asc' ':!:*.yaml' ':!:docs/requirements.txt' ':!:*.service' ':!:CONTRIBUTORS' ':!:LICENSE' ':!:.*' ':!:go.mod' ':!:go.sum' ':!:pkg/mail/testdata'

.PHONY: test
test:
	$(GO) test ./...

.PHONY: lint
lint:
	$(GO) generate ./...
	@echo '[goimports-reviser]'
	$Q ! $(GO) run $(import_reviser) $(import_reviser_flags) -list-diff -output stdout ./... | grep . || { \
		echo 'error: above files need import sorting'; \
		exit 1; \
	}
	@echo '[gofumpt]'
	$Q ! $(GO) run $(gofumpt) -d . | grep ^diff || { \
		echo 'error: above files need reformatting'; \
		exit 1; \
	}
	@echo '[golangci-lint]'
	@$(GO) run $(golangci_lint) run
	@echo '[license-check]'
	$Q ! git --no-pager grep -LF 'SPDX-License-Identifier: GPL-2.0-or-later' -- $(license_exclude) || { \
		echo 'error: above files are missing license'; \
		exit 1; \
	}
	$Q ! git --no-pager grep -LF 'Copyright (C) The Patchwork Contributors' -- $(license_exclude) || { \
		echo 'error: above files are missing copyright notice'; \
		exit 1; \
	}
	@echo '[white-space]'
	$Q git ls-files ':!:pkg/mail/testdata' | xargs devtools/check-whitespace
	@echo '[codespell]'
	$Q codespell *

.PHONY: format
format:
	$(GO) run $(import_reviser) $(import_reviser_flags) ./...
	$(GO) run $(gofumpt) -w .

REVISION_RANGE ?= @{u}..

.PHONY: check-patches
check-patches:
	$Q devtools/check-patches $(REVISION_RANGE)

.PHONY: git-config
git-config:
	git config format.subjectPrefix "PATCH patchwork"
	git config sendemail.to "patchwork@lists.ozlabs.org"
	@mkdir -p .git/hooks
	@rm -f .git/hooks/commit-msg*
	ln -s ../../devtools/commit-msg .git/hooks/commit-msg

.PHONY: tag-release
tag-release:
	@cur_version=`sed -En 's/PATCHWORK_VERSION .* \|\| echo v([0-9].*)\>\)$$/\1/p' Makefile` && \
	next_version=`echo $$cur_version | awk -F. -v OFS=. '{$$(NF) += 1; print}'` && \
	read -rp "next version ($$next_version)? " n && \
	if [ -n "$$n" ]; then next_version="$$n"; fi && \
	set -xe && \
	sed -i "s/\<v$$cur_version\>/v$$next_version/" Makefile && \
	git commit -sm "patchwork: release v$$next_version" -m "`devtools/git-stats v$$cur_version..`" Makefile && \
	git tag -sm "`devtools/git-stats v$$cur_version..HEAD^`" "v$$next_version"
