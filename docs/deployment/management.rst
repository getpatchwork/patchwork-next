.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Management Commands
===================

All administrative tasks are performed through the `pw` command.


Database
--------

`pw db sync`
  Create the database schema on a fresh database, or apply pending migrations
  on an existing one.

`pw db export`
  Export data from a Django 3.2 patchwork database as SQL statements. This is
  used during migration from the Python version to the Go version. The output
  is written to stdout.

`pw db import`
  Import SQL data from stdin into the current database. Used to load data
  previously exported with `pw db export` into a fresh 4.0 database.


User Management
---------------

`pw admin user list`
  List all users.

`pw admin user create`
  Create a new user. Prompts for a password interactively.

  Options:

  - `-u`, `--username` -- Username
  - `-e`, `--email` -- Email address
  - `--admin` -- Grant admin privileges

`pw admin user delete <username>`
  Delete a user. Use `-f` to skip confirmation.

`pw admin user passwd <username>`
  Change a user's password interactively.


Project Management
------------------

`pw admin project list`
  List all projects.

`pw admin project show <linkname>`
  Show full details for a project.

`pw admin project create`
  Create a new project.

  Required options:

  - `-n`, `--name` -- Display name
  - `-l`, `--linkname` -- URL-safe identifier
  - `-i`, `--list-id` -- Mailing list ID
  - `-e`, `--list-email` -- Mailing list email address

  Optional: `--web-url`, `--scm-url`, `--webscm-url`,
  `--list-archive-url`, `--subject-match`, `--commit-url-format`

`pw admin project update <linkname>`
  Update an existing project's fields.

`pw admin project delete <linkname>`
  Delete a project. Use `-f` to skip confirmation.


States and Tags
---------------

`pw admin state list`
  List all patch states.

`pw admin state create`
  Create a new state with `-n` (name), `-s` (slug), `-o` (ordering),
  and optionally `--action-required`.

`pw admin tag list`
  List all tags.

`pw admin tag create`
  Create a new tag with `-n` (name), `-p` (pattern), `-a` (abbreviation).


Maintainers and Delegation
---------------------------

`pw admin maintainer list <project>`
  List maintainers for a project.

`pw admin maintainer add <project> <username>`
  Add a maintainer to a project.

`pw admin maintainer remove <project> <username>`
  Remove a maintainer from a project.

`pw admin delegate-rule list <project>`
  List autodelegation rules for a project.

`pw admin delegate-rule create`
  Create a delegation rule with `--project`, `--user`, `--path`,
  and `--priority`.

`pw admin delegate-rule delete <id>`
  Delete a delegation rule. Use `-f` to skip confirmation.


Garbage Collection
------------------

`pw admin gc`
  Clean up expired sessions, stale email confirmations, and inactive users
  with no pending confirmation. Run this periodically via cron or a systemd
  timer.


Email Ingress
-------------

`pw ingress`
  Start the SMTP daemon for receiving emails (long-running service).

`pw ingress --stdin`
  Read a single email from stdin and process it.

`pw ingress --mbox`
  Read all emails in mbox format from stdin.

`pw ingress --list-id <id>`
  Override the List-ID header value.


Configuration
-------------

`pw config`
  Print a default, annotated configuration file to stdout.


HTTP Server
-----------

`pw http`
  Start the HTTP server exposing the web interface and REST API.
