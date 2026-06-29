.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Contributing
============

Coding Standards
----------------

All Go code must pass ``make lint`` which runs goimports-reviser, gofumpt and
golangci-lint. Running ``make format`` will auto-fix most formatting issues.

All code must be licensed using `GPL v2.0 or later`_ and must have a `SPDX
License Identifier`_ stating this. A copyright line should be included on new
files:

.. code-block:: go

   // Patchwork - automated patch tracking system
   // Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
   //
   // SPDX-License-Identifier: GPL-2.0-or-later

.. _GPL v2.0 or later: https://spdx.org/licenses/GPL-2.0-or-later.html
.. _SPDX License Identifier: https://spdx.org/using-spdx-license-identifier


Building
--------

Building requires a Go toolchain (see ``go.mod`` for the minimum version):

.. code-block:: console

   $ make

This runs ``go generate`` (for templ templates) then ``go build``. The
resulting binary is ``./pw``.


Testing
-------

Run the test suite:

.. code-block:: console

   $ make test

Tests use in-memory SQLite databases, so no external database is required.


Getting Started
---------------

After cloning the repository, run ``make git-config`` to set up your local
clone with sensible defaults (subject prefix for ``git format-patch``,
``sendemail.to`` address, and a commit message hook):

.. code-block:: console

   $ make git-config
   git config format.subjectPrefix "PATCH patchwork"
   git config sendemail.to "patchwork@lists.ozlabs.org"
   ln -s ../../devtools/commit-msg .git/hooks/commit-msg

Commit Messages
---------------

Follow the Linux kernel commit message conventions. In particular:

- Use a ``component:`` prefix derived from the files changed (look at
  ``git log --oneline`` for examples).
- Imperative mood, lowercase after the prefix, no trailing period.
- Hard wrap the body at 72 columns.
- Explain *why*, not *what*.
- Include a ``Signed-off-by`` trailer.

Patches should be checked with ``make check-patches`` before submission.


Documentation
-------------

Documentation is authored in `reStructuredText`_ and built with `Sphinx`_.
To build the docs locally:

.. code-block:: console

   $ make docs

The output is in ``docs/_build/``. The live documentation is published at
https://getpatchwork.github.io/patchwork-next/.

.. _reStructuredText: https://www.sphinx-doc.org/en/master/usage/restructuredtext/basics.html
.. _Sphinx: https://www.sphinx-doc.org/en/master/


Release Notes
-------------

Patchwork uses `reno`_ for release note management. To create a new release
note:

.. code-block:: console

   $ pip install reno
   $ reno new <slugified-summary-of-change>

Edit the created file under ``releasenotes/notes/``, removing irrelevant
sections, and include it in your change.

.. _reno: https://docs.openstack.org/developer/reno/


Reporting Issues
----------------

Issues can be reported to the `mailing list`_ or the `GitHub issue tracker`_.

.. _GitHub issue tracker: https://github.com/getpatchwork/patchwork


Submitting Changes
------------------

All patches should be sent to the `mailing list`_. You must be subscribed to
the list in order to submit patches. Patches should be submitted using
``git send-email``.

Before submitting, ensure:

- All tests pass (``make test``).
- The linter is happy (``make lint``).
- Commit messages pass validation (``make check-patches``).
- Documentation has been updated if needed.
- A release note is included for user-visible changes.


.. _mailing-lists:

Mailing List
------------

Patchwork uses a single mailing list for development, questions and
announcements:

    patchwork@lists.ozlabs.org

Further information is available on `lists.ozlabs.org`_.

.. _mailing list: https://lists.ozlabs.org/listinfo/patchwork
.. _lists.ozlabs.org: https://lists.ozlabs.org/listinfo/patchwork
