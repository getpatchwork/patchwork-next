.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Migrating from Django to Go
===========================

This guide walks through migrating an existing Django-based Patchwork
installation to Patchwork 4.0.

.. important::

   The export tool requires a **patchwork 3.x** database schema or later (Django
   migration 0042+). If you are running an older version, upgrade your Django
   installation to the latest 3.x first, then proceed with the migration.


Overview
--------

Patchwork 4.0 is a complete rewrite in Go. The deployment model is fundamentally
different: a single `pw` binary replaces Django, gunicorn/uwsgi, virtualenvs,
and management commands.

The database schema is similar but not identical, which is why data must be
exported and re-imported rather than used in place.

All 3.x schema variations are handled automatically (missing columns in older
releases get sensible defaults, and tables added after 3.2 are skipped if
absent). Databases with non-standard customizations (extra columns or additional
tables) should also work (to some extent): the export procedure only reads known
columns and silently ignores everything else.


Command Mapping
---------------

.. list-table::
   :header-rows: 1
   :widths: 40 40

   * - Django (old)
     - Go (new)
   * - `manage.py createsuperuser`
     - `pw admin user create --admin`
   * - `manage.py migrate`
     - `pw db sync`
   * - `manage.py parsemail`
     - `pw ingress --stdin`
   * - `manage.py parsearchive`
     - `pw ingress --mbox`
   * - `manage.py cron`
     - `pw admin gc`
   * - `manage.py dumparchive`
     - no equivalent
   * - Django admin panel (`/admin`)
     - `pw admin` subcommands
   * - `production.py` settings
     - `patchwork.toml`
   * - gunicorn/uwsgi + nginx
     - `pw http` + nginx
   * - XML-RPC API
     - removed, use REST API
   * - `/etc/aliases` + `parsemail.sh`
     - postfix transport maps to `pw ingress`


Configuration Mapping
---------------------

.. list-table::
   :header-rows: 1
   :widths: 40 40

   * - Django setting
     - TOML equivalent
   * - `DATABASES['default']`
     - `[database] url`
   * - `NOTIFICATION_FROM_EMAIL`
     - `[smtp] from`
   * - `DEFAULT_ITEMS_PER_PAGE`
     - built-in default (30)
   * - `ENABLE_REST_API`
     - always enabled
   * - `ENABLE_XMLRPC`
     - removed
   * - `FORCE_HTTPS_LINKS`
     - set `[http] base-url` with `https://`


Migration Procedure
-------------------

1. Back up your existing Django database
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: console

   $ pg_dump patchwork > patchwork-django-backup.sql

2. Install the `pw` binary
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Build from source or download a release:

.. code-block:: console

   $ make
   $ sudo make install

3. Create the configuration file
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Generate a template and edit it:

.. code-block:: console

   $ pw config > /etc/patchwork.toml

Point `[database] url` at a **new, empty** database:

.. code-block:: toml

   [database]
   url = "postgres://patchwork:secret@localhost/patchwork_v4"

4. Initialize the new database
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: console

   $ pw db sync

This creates the 4.0 schema and seeds default states and tags.

5. Export data from the old database
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Run `pw db export` against the **old** Django database. This reads the Django
3.2 schema and produces SQL that is compatible with the 4.0 schema. The output
format and identifier quoting are automatically matched to the source database
dialect. PostgreSQL exports use the COPY protocol for faster imports.

.. code-block:: console

   $ pw --database-url "postgres://patchwork:secret@localhost/patchwork" \
        db export > patchwork-data.sql

To migrate to a different database engine, use `--dialect` to override the
output format. For example, to export from MySQL for import into PostgreSQL:

.. code-block:: console

   $ pw --database-url "mysql://patchwork:secret@localhost/patchwork" \
        db export --dialect=postgres > patchwork-data.sql

6. Import data into the new database
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Import the exported data using the native client for your database:

.. code-block:: console

   $ psql -U patchwork -d patchwork4 < patchwork-data.sql

.. code-block:: console

   $ mysql -u patchwork -p patchwork4 < patchwork-data.sql

.. code-block:: console

   $ sqlite3 /var/lib/patchwork/patchwork.db < patchwork-data.sql

7. Set up systemd services
~~~~~~~~~~~~~~~~~~~~~~~~~~~

Enable and start the services:

.. code-block:: console

   $ sudo systemctl enable --now pw-http pw-ingress

8. Update Postfix
~~~~~~~~~~~~~~~~~~

Replace the old aliases-based setup with transport maps.

Remove from `/etc/aliases`:

.. code-block:: ini

   patchwork: "|/opt/patchwork/patchwork/bin/parsemail.sh"

Add to `/etc/postfix/main.cf`:

.. code-block:: ini

   transport_maps = lmdb:/etc/postfix/transport

Create `/etc/postfix/transport`:

.. code-block:: bash

   lists.example.com    smtp:127.0.0.1:2525

Rebuild and reload:

.. code-block:: console

   $ sudo postmap /etc/postfix/transport
   $ sudo systemctl reload postfix

9. Update nginx
~~~~~~~~~~~~~~~~

Replace the gunicorn/uwsgi upstream with the `pw http` server endpoint. There is
no need for a `location /static` block anymore:

.. code-block:: nginx

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

.. note:: See :ref:`reverse-proxy` for a more complete nginx example.

10. Verify and switch over
~~~~~~~~~~~~~~~~~~~~~~~~~~~

- Check the web interface is accessible.
- Verify the REST API returns data: `curl https://patchwork.example.com/api/`
- Send a test email to confirm `pw ingress` processes it.
- Decommission the old Django installation.


Existing Credentials
--------------------

User passwords are stored using Django's PBKDF2-SHA256 format. The Go version
reads and verifies passwords in the same format, so all existing user accounts
and API tokens continue to work without any action.
