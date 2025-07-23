# Grapher

This project provides a small example application combining a Go backend with a Svelte frontend.
It periodically polls an SNMP source for a numeric value, stores it as a time series and exposes
a graph to the user. OAuth2 authentication can be enabled via environment variables.

## Backend

The backend resides in `backend/` and reads its polling targets from a JSON configuration file.
The path can be specified via the `CONFIG_PATH` environment variable (default `config.json`).
The file should list one or more polling sources. Each source can optionally define a `version` field to select the SNMP protocol version (supported values are `1` and `2c`; default is `1`):

```json
{
  "sources": [
    {
      "name": "Internal sensor",
      "host": "localhost",
      "community": "public",
      "oid": ".1.3.6.1.2.1.1.3.0",
      "version": "2c",
      "units": "C",
      "type": "temperature",
      "color": "#ff0000"
    }
  ]
}
```
The optional `color` field controls the colour used for this source when drawing graphs.
Any CSS colour value is allowed.

You can optionally define comparison graphs which combine multiple sources in a
single plot:

```json
{
  "sources": [
    { "name": "Internal sensor", ... }
  ],
  "graphs": [
    { "name": "Inside vs Outside", "sources": ["Internal sensor", "External sensor"] }
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

## Database maintenance

The Docker image ships with a `dbtool` helper that can modify the database via the running server. It supports renaming, deleting, merging and listing data sources. The tool sends REST requests to the backend and therefore requires the server to be running. The target server can be specified with the `-addr` flag or `SERVER_ADDR` environment variable (default `http://localhost:8080`).

Examples:

```sh
docker run --rm --network host grapher ./dbtool rename old_name new_name
docker run --rm --network host grapher ./dbtool list
```

