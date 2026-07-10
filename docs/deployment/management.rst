.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Management Commands
===================

.. program:: pw

All administrative tasks are performed through the :program:`pw` command.

.. option:: --version

   Print the patchwork version and exit.

.. option:: -S, --syslog

   Redirect logging to syslog.


Database
--------

``pw db sync``
~~~~~~~~~~~~~~

.. program:: pw db sync

Create or update the database schema. On a fresh database, this creates all
tables. On an existing one, it applies pending migrations.

``pw db export``
~~~~~~~~~~~~~~~~

.. program:: pw db export

Export data from a Django 3.2 patchwork database as SQL statements. This is used
during migration from the Python version to the Go version. The output is
written to stdout.

``pw db import``
~~~~~~~~~~~~~~~~

.. program:: pw db import

Import SQL data from stdin into the current database. Used to load data
previously exported with :program:`pw db export` into a fresh 4.0 database.


User Management
---------------

``pw admin user list``
~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin user list

List all users.

``pw admin user create -u <username> -e <email>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin user create

Create a new user. Prompts for a password interactively.

.. option:: -u, --username <username>

   Username.

.. option:: -e, --email <email>

   Email address.

.. option:: --admin

   Grant admin privileges.

``pw admin user delete <username>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin user delete

Delete a user.

.. option:: <username>

   Username to delete.

.. option:: -f, --force

   Skip confirmation.

``pw admin user passwd <username>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin user passwd

Change a user's password interactively.

.. option:: <username>

   Username to change password for.


Project Management
------------------

``pw admin project list``
~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin project list

List all projects.

``pw admin project show <linkname>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin project show

Show full details for a project.

.. option:: <linkname>

   Project linkname.

``pw admin project create -n <name> -l <linkname> -e <email>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin project create

Create a new project.

.. option:: -n, --name <name>

   Display name (required).

.. option:: -l, --linkname <linkname>

   URL-safe identifier (required).

.. option:: -i, --list-id <id>

   Mailing list ID (required).

.. option:: -e, --list-email <email>

   Mailing list email address (required).

.. option:: --web-url <url>

   Project website URL.

.. option:: --scm-url <url>

   Source code management URL.

.. option:: --webscm-url <url>

   Web SCM URL.

.. option:: --list-archive-url <url>

   List archive URL.

.. option:: --subject-match <regex>

   Subject filter regex.

.. option:: --commit-url-format <format>

   Commit URL format string.

``pw admin project update <linkname> ...``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin project update

Update an existing project's fields.

.. option:: <linkname>

   Project linkname.

.. option:: -n, --name <name>

   Display name.

.. option:: --list-id <id>

   Mailing list ID.

.. option:: --list-email <email>

   Mailing list email address.

.. option:: --web-url <url>

   Project website URL.

.. option:: --scm-url <url>

   Source code management URL.

.. option:: --webscm-url <url>

   Web SCM URL.

.. option:: --list-archive-url <url>

   List archive URL.

.. option:: --subject-match <regex>

   Subject filter regex.

.. option:: --commit-url-format <format>

   Commit URL format string.

``pw admin project delete <linkname>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin project delete

Delete a project.

.. option:: <linkname>

   Project linkname.

.. option:: -f, --force

   Skip confirmation.


States
------

``pw admin state list``
~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin state list

List all patch states.

``pw admin state create -n <name> -s <slug> -o <ordering>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin state create

Create a new state.

.. option:: -n, --name <name>

   State name (required).

.. option:: -s, --slug <slug>

   URL-safe slug (required).

.. option:: -o, --ordering <number>

   Sort order (required).

.. option:: --action-required

   Whether this state requires action.

``pw admin state delete <slug>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin state delete

Delete a state.

.. option:: <slug>

   State slug to delete.

.. option:: -f, --force

   Skip confirmation.


Tags
----

``pw admin tag list``
~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin tag list

List all tags.

``pw admin tag create -n <name> -a <abbrev> -p <pattern>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin tag create

Create a new tag.

.. option:: -n, --name <name>

   Tag name (required).

.. option:: -p, --pattern <regex>

   Regex pattern to match (required).

.. option:: -a, --abbrev <abbrev>

   Short abbreviation (required).

.. option:: --show-column

   Show in list columns (default: true).

``pw admin tag delete <name>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin tag delete

Delete a tag.

.. option:: <name>

   Tag name to delete.

.. option:: -f, --force

   Skip confirmation.


Maintainers
-----------

``pw admin maintainer list <project>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin maintainer list

List maintainers for a project.

.. option:: <project>

   Project linkname.

``pw admin maintainer add <project> <username>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin maintainer add

Add a maintainer to a project.

.. option:: <project>

   Project linkname.

.. option:: <username>

   Username to add as maintainer.

``pw admin maintainer remove <project> <username>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin maintainer remove

Remove a maintainer from a project.

.. option:: <project>

   Project linkname.

.. option:: <username>

   Username to remove.


Delegation Rules
----------------

``pw admin delegate-rule list <project>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin delegate-rule list

List autodelegation rules for a project.

.. option:: <project>

   Project linkname.

``pw admin delegate-rule create``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin delegate-rule create

Create a delegation rule.

.. option:: --project <linkname>

   Project linkname (required).

.. option:: --user <username>

   Delegate username (required).

.. option:: --path <pattern>

   File path pattern (required).

.. option:: --priority <number>

   Rule priority; lower values are higher priority (default: 0).

``pw admin delegate-rule delete <id>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin delegate-rule delete

Delete a delegation rule.

.. option:: <id>

   Rule ID to delete.

.. option:: -f, --force

   Skip confirmation.


Webhooks
--------

``pw admin webhook list [<project>]``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin webhook list

List webhooks.

.. option:: <project>

   Project linkname. When omitted, list webhooks for all projects.

``pw admin webhook create --project <linkname> --url <url> --user <username>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin webhook create

Create a webhook.

.. option:: --project <linkname>

   Project linkname (required).

.. option:: --user <username>

   Creator username (required).

.. option:: --url <url>

   Webhook endpoint URL (required).

.. option:: --secret <secret>

   Secret for HMAC signatures.

.. option:: --events <events>

   Comma-separated event categories, or ``*`` for all. Use ``?`` to list all
   supported events (default: ``*``).

.. option:: --active, --no-active

   Whether the webhook is active (default: true).

``pw admin webhook update <id> ...``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin webhook update

Update a webhook.

.. option:: <id>

   Webhook ID.

.. option:: --url <url>

   Webhook endpoint URL.

.. option:: --secret <secret>

   Secret for HMAC signatures.

.. option:: --events <events>

   Comma-separated event categories, or ``*`` for all. Use ``?`` to list all
   supported events.

.. option:: --active, --no-active

   Whether the webhook is active.

``pw admin webhook delete <id>``
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. program:: pw admin webhook delete

Delete a webhook.

.. option:: <id>

   Webhook ID to delete.

.. option:: -f, --force

   Skip confirmation.


Garbage Collection
------------------

``pw admin gc``
~~~~~~~~~~~~~~~

.. program:: pw admin gc

Clean up expired sessions, stale email confirmations, and inactive users with
no pending confirmation. Run this periodically via cron or a systemd timer.


Email Ingress
-------------

``pw ingress``
~~~~~~~~~~~~~~

.. program:: pw ingress

Start the SMTP daemon for receiving emails (long-running service). When neither
:option:`--stdin` nor :option:`--mbox` is given, the daemon listens on the
address configured with :confval:`[ingress].listen`.

.. option:: -i, --stdin

   Read a single email from stdin and exit.

.. option:: -m, --mbox

   Read all emails in mbox format from stdin.

.. option:: -l, --list-id <id>

   Force List-ID value instead of reading it from email headers.


Configuration
-------------

``pw config``
~~~~~~~~~~~~~

.. program:: pw config

Print a default, annotated configuration file to stdout.

``pw config url``
~~~~~~~~~~~~~~~~~

.. program:: pw config url

Generate a database connection URL interactively.


HTTP Server
-----------

``pw http``
~~~~~~~~~~~

.. program:: pw http

Start the HTTP server exposing the web interface and REST API. The listen
address is controlled by :confval:`[http].listen` and other settings such as
:confval:`[http].base-url`, :confval:`[http].custom-css`, :confval:`[http].nav-html`
and :confval:`[http].footer-html`.
