# cache

`cache` is a home inventory app written in Go, designed to be the "source of truth" for your physical assets.

It is a single binary that listens in 127.0.0.1, port 19970,
and provides a very simple REST API and a simple web app.

It needs a PostgreSQL connection string provided via an environment variable, `DATABASE_URL`.

It does not have any form of security or authentication:
it is meant to be used under a HTTP server such as Caddy with HTTPS and authentication.
