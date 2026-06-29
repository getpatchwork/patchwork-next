.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Autodelegation
==============

Autodelegation allows patches to be automatically delegated to a user based on
the files modified by the patch. To do this, a number of rules can be
configured using the ``pw`` command line tool.

.. note::

   Autodelegation can only be configured by Patchwork administrators. If you
   require configuration of autodelegation rules on a local instance, contact
   your Patchwork administrator.

Managing Rules
--------------

Rules are managed using the ``pw admin delegate-rule`` subcommand.

To list existing rules for a project::

    $ pw admin delegate-rule list my-project

To create a new rule::

    $ pw admin delegate-rule create \
        --project my-project \
        --user reviewer \
        --path "drivers/net/*" \
        --priority 10

To delete a rule::

    $ pw admin delegate-rule delete 42

Rule Fields
-----------

User
  The patchwork user that should be autodelegated to the patch.

Priority
  The priority of the rule relative to other rules. Higher values indicate
  higher priority. If two rules have the same priority, ordering will be based
  on the path.

Path
  A path in `fnmatch`__ format. The fnmatch library allows for limited, Unix
  shell-style wildcarding. Filenames are extracted from patch lines beginning
  with ``---`` or ``+++``.

  You can simply use a bare path::

      drivers/net/ethernet/intel/ice/ice_main.c

  Or it is also possible to use relative paths, such as::

      */manage.py

Rules are applied at patch parse time.

__ https://docs.python.org/3/library/fnmatch.html
