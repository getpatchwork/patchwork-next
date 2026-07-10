.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

The REST API
============

Patchwork provides a REST API. This API can be used to retrieve and modify
information about patches, projects and more.

This guide provides an overview of how one can interact with the REST API. For
detailed information on type and response format of the various resources
exposed by the API, refer to the auto-generated OpenAPI documentation at::

    https://patchwork.example.com/api/docs

where `patchwork.example.com` refers to the URL of your Patchwork instance.

.. versionchanged:: 4.0

   The OpenAPI schema is now auto-generated at runtime and served at
   `/api/docs`. Static schema files are no longer shipped.

.. versionchanged:: 4.0

   The XML-RPC API has been removed. Use the REST API with clients like
   :program:`git-pw` instead.

Getting Started
---------------

The easiest way to start experimenting with the API is to visit the
auto-generated documentation at `/api/docs`.

REST APIs run over plain HTTP(S), thus, the API can be interfaced using
applications or libraries that support this widespread protocol. One such
application is `curl`_, which can be used to both retrieve and send information
to the REST API. For example, to get the version of the REST API for a
Patchwork instance hosted at `patchwork.example.com`, run:

.. code-block:: shell

    $ curl -s 'https://patchwork.example.com/api/1.4/' | python -m json.tool
    {
        "bundles": "https://patchwork.example.com/api/1.4/bundles/",
        "covers": "https://patchwork.example.com/api/1.4/covers/",
        "events": "https://patchwork.example.com/api/1.4/events/",
        "patches": "https://patchwork.example.com/api/1.4/patches/",
        "people": "https://patchwork.example.com/api/1.4/people/",
        "projects": "https://patchwork.example.com/api/1.4/projects/",
        "series": "https://patchwork.example.com/api/1.4/series/",
        "users": "https://patchwork.example.com/api/1.4/users/"
    }

Tools like `curl` and libraries like `requests` can be used to build anything
from small utilities to full-fledged clients targeting the REST API. For an
overview of existing API clients, refer to :doc:`/usage/clients`.

Versioning
----------

By default, all requests will receive the latest version of the API: currently
`1.4`:

.. code-block:: http

    GET /api HTTP/1.1

You should explicitly request this version through the URL to prevent API
changes breaking your application:

.. code-block:: http

    GET /api/1.4 HTTP/1.1

Older API versions will be deprecated and removed over time.

.. csv-table::
   :header: "API Version", "Supported?"

   1.0, yes
   1.1, yes
   1.2, yes
   1.3, yes
   1.4, yes

Schema
------

Responses are returned as JSON. Blank fields are returned as `null`, rather than
being omitted. Timestamps use the ISO 8601 format, times are in UTC::

    YYYY-MM-DDTHH:MM:SSZ

Requests should use either query parameters or form-data, depending on the
method.

The auto-generated OpenAPI schema is available at `/api/docs` and describes all
endpoints, parameters, and response formats.

Parameters
----------

For `GET` requests, parameters should be passed as HTTP query string parameters:

.. code-block:: shell

    $ curl 'https://patchwork.example.com/api/patches?state=under-review'

For `POST` and `PATCH` requests, parameters should be encoded as JSON with
a `Content-Type` of `application/json`:

.. code-block:: shell

    $ curl -X PATCH \
      --header "Content-Type: application/json" \
      --data '{"state":"under-review"}' \
      'https://patchwork.example.com/api/patches/123/'

Authentication
--------------

Patchwork supports authentication using bearer token authentication. To
authenticate, first generate a token from your user profile page, then::

    $ curl -H "Authorization: Bearer ${token}" \
        'https://patchwork.example.com/api/'

The legacy `Token` scheme is also accepted for backward compatibility::

    $ curl -H "Authorization: Token ${token}" \
        'https://patchwork.example.com/api/'

Not all resources require authentication. Those that do will return
`401 (Unauthorized)` if authentication is not provided.

Pagination
----------

Requests that return multiple items are paginated by 30 items by default. You
can change page using the `?page` parameter and set custom page sizes up to
250 using the `?per_page` parameter:

.. code-block:: shell

    $ curl 'https://patchwork.example.com/api/patches?page=2&per_page=100'

The `Link` header includes pagination information::

    Link: <https://patchwork.example.com/api/patches?page=3&per_page=100>; rel="next",
      <https://patchwork.example.com/api/patches?page=50&per_page=100>; rel="last"


.. _curl: https://curl.haxx.se/
