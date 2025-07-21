# Grapher

This project provides a small example application combining a Go backend with a Svelte frontend.
It periodically polls an SNMP source for a numeric value, stores it as a time series and exposes
a graph to the user. OAuth2 authentication can be enabled via environment variables.

## Backend

The backend resides in `backend/` and reads its polling targets from a JSON configuration file.
The path can be specified via the `CONFIG_PATH` environment variable (default `config.json`).
The file should list one or more polling sources:

```json
{
  "sources": [
    {
      "host": "localhost",
      "community": "public",
      "oid": ".1.3.6.1.2.1.1.3.0"
    }
  ]
}
```

Additional options can still be set through environment variables:

- `POLL_INTERVAL` – polling interval (e.g. `30s`)
- `DB_PATH` – path to the BoltDB file (`data.db`)
- `ADDR` – HTTP listen address (`:8080`)
- `OAUTH2_INTROSPECT_URL` – token introspection endpoint
- `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` – credentials for introspection

Static frontend files are served from `backend/public`.

## Frontend

A simple Svelte application in `frontend/` uses D3 to plot the time series returned from `/api/data`.
It renders a dashboard where all configured sources are shown on their own chart simultaneously.
Run `npm install` and `npm run build` in the `frontend` directory to build the assets. They will be
placed into `backend/public` and served by the Go backend.

## Docker

A multi-stage `Dockerfile` is provided to build and run the entire stack:

```sh
# Build and run
docker build -t grapher .
docker run -p 8080:8080 grapher
```

Environment variables can be passed to configure the application in the container.


Alternatively you can start the application using Docker Compose:

```sh
docker compose up
```

This will build the image and run the service on port 8080 while persisting the database in a named volume.
