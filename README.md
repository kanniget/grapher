# Grapher

A lightweight SNMP time-series graphing application. It periodically polls SNMP-enabled devices for numeric values, stores them in a local BoltDB database, and renders interactive charts in a web dashboard.

## Features

- **SNMP Polling** — Polls one or more SNMP devices at configurable intervals (supports v1, v2c, and v3)
- **Time-Series Storage** — Persists samples in a local BoltDB database
- **Interactive Dashboard** — Svelte + D3.js frontend with tabbed graph groups, auto-refresh, and custom colours
- **Comparison Graphs** — Plot multiple sources on a single chart
- **Cumulative Value Handling** — Automatically computes deltas for counter-type OIDs (e.g. interface traffic)
- **OAuth2 Authentication** — Optional token introspection via RFC 7662
- **Database Maintenance** — CLI tool to rename, delete, merge, and list data sources
- **Trap Value Decoding** — Decodes byte-array SNMP trap values into human-readable strings
- **JSON & YAML Config** — Flexible configuration file formats
- **Docker Support** — Multi-stage build with Docker Compose

## Quick Start

### Docker Compose (recommended)

```sh
docker compose up
```

The service starts on **port 8088** and persists the database in a named volume.

### Docker

```sh
docker build -t grapher .
docker run -p 8088:8080 grapher
```

### From Source

```sh
# Build frontend
cd frontend && npm install && npm run build

# Build and run backend
cd ../backend && go build -o grapher .
./grapher
```

## Configuration

The application reads its polling targets from a configuration file. The path is set via `CONFIG_PATH` (default `config.json`). Both JSON and YAML formats are supported.

### Source Definition

Each source represents an SNMP polling target:

```json
{
  "sources": [
    {
      "name": "Internal sensor - Temp(C)",
      "host": "192.168.1.253",
      "community": "public",
      "oid": "1.3.6.1.4.1.20916.1.10.1.1.1.0",
      "version": "1",
      "units": "C",
      "cumulative": false,
      "type": "temperature",
      "color": "#ff0000"
    }
  ]
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `name` | Display name for the source | Auto-generated from host + OID |
| `host` | SNMP device IP or hostname | `localhost` |
| `community` | SNMP community string | `public` |
| `oid` | SNMP OID to poll | `.1.3.6.1.2.1.1.3.0` (system uptime) |
| `version` | SNMP protocol version (`1`, `2c`, or `3`) | `1` |
| `units` | Display unit label (e.g. `C`, `%`, `Bytes`) | — |
| `type` | Data type label shown on graphs | — |
| `color` | Custom CSS colour for the graph line | Auto-assigned |
| `cumulative` | `true` for counter/running-total OIDs | `false` |

### Graph Definitions

Graphs combine one or more sources into a single chart:

```json
{
  "graphs": [
    {
      "name": "Room vs Switch Output Temp",
      "sources": ["Internal sensor - Temp(C)", "External sensor - Temp(C)"],
      "timespan": "24h"
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `name` | Graph title shown in the dashboard |
| `sources` | List of source names to plot |
| `timespan` | Go duration limiting historical data (e.g. `24h`, `72h`) |

### Graph Groups

Groups organise graphs into tabs in the frontend:

```json
{
  "groups": [
    {
      "name": "Temperatures",
      "graphs": ["Room Temp", "Switch Output Temp"]
    }
  ]
}
```

### Legacy Single-Source Config

For simple setups, a single source can be defined at the top level:

```json
{
  "host": "192.168.1.253",
  "community": "public",
  "oid": "1.3.6.1.2.1.1.3.0"
}
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFIG_PATH` | Path to the configuration file | `config.json` |
| `POLL_INTERVAL` | SNMP polling interval (Go duration) | `1m` |
| `DB_PATH` | Path to the BoltDB database file | `data.db` |
| `ADDR` | HTTP listen address | `:8080` |
| `OAUTH2_INTROSPECT_URL` | OAuth2 token introspection endpoint (enables auth) | — |
| `OAUTH2_CLIENT_ID` | OAuth2 client ID for introspection | — |
| `OAUTH2_CLIENT_SECRET` | OAuth2 client secret for introspection | — |
| `SERVER_ADDR` | Server address for the `dbtool` CLI | `http://localhost:8080` |

## API Endpoints

All API endpoints (except `/api/db/list`) require a `Bearer` token when OAuth2 is enabled.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/data?graphs=a,b,c` | Returns time-series data for the specified graphs |
| `GET` | `/api/graphs` | Returns graph names and group definitions |
| `GET` | `/api/db/list` | Lists all data source names |
| `POST` | `/api/db/rename` | Renames a data source (`{"from":"old","to":"new"}`) |
| `POST` | `/api/db/delete` | Deletes a data source (`{"name":"source"}`) |
| `POST` | `/api/db/merge` | Merges one source into another (`{"from":"old","to":"new"}`) |
| `GET` | `/` | Serves the static frontend dashboard |

### API Response Examples

**`/api/data`**

```json
{
  "graphs": {
    "Room Temp": {
      "data": [
        {"timestamp": 1717400000, "value": 2350, "source": "Internal sensor - Temp(C)"}
      ],
      "units": "C",
      "type": "temperature",
      "colors": {"Internal sensor - Temp(C)": "#ff0000"}
    }
  },
  "groups": [
    {"name": "Temperatures", "graphs": ["Room Temp"]}
  ]
}
```

**`/api/graphs`**

```json
{
  "groups": [
    {"name": "Temperatures", "graphs": ["Room Temp", "Switch Output Temp"]}
  ],
  "graphs": ["Room Temp", "Switch Output Temp", "Humidity", "HP9000 - Port 1 Traffic"]
}
```

## Database Maintenance CLI

The `dbtool` binary is included in the Docker image and can manage the database via the running server:

```sh
# List all data sources
docker run --rm --network host grapher ./dbtool list

# Rename a data source
docker run --rm --network host grapher ./dbtool rename old_name new_name

# Delete a data source
docker run --rm --network host grapher ./dbtool delete source_name

# Merge two data sources (combines data, keeps destination)
docker run --rm --network host grapher ./dbtool merge old_name new_name
```

The target server can be specified with the `-addr` flag or `SERVER_ADDR` environment variable.

## Frontend

The dashboard is built with:

- **Svelte 4** — Reactive UI framework
- **D3.js 7** — Chart rendering
- **Flowbite-Svelte** — UI component library
- **Tailwind CSS 4** — Utility-first styling

The frontend supports:

- **Tabbed navigation** — Graphs are organised into groups as tabs
- **Auto-refresh** — Configurable intervals (5s, 10s, 30s, 60s, or off)
- **Manual refresh** — Click the Refresh button to reload data
- **Custom colours** — Per-source colours defined in the config
- **Cumulative delta display** — Counter-type sources show the rate of change

Build the frontend assets:

```sh
cd frontend
npm install
npm run build
```

The built files are placed in `frontend/dist/` and copied into `backend/public/` during the Docker build.

## Docker

### Multi-stage Build

The `Dockerfile` uses three stages:

1. **frontend** (Node 20) — Builds the Svelte application
2. **backend** (Go 1.23) — Compiles the server and dbtool
3. **final** (Debian slim) — Minimal runtime image

### Docker Compose

```yaml
version: '3'
services:
  grapher:
    build: .
    ports:
      - "8088:8080"
    environment:
      CONFIG_PATH: "/app/config.json"
      POLL_INTERVAL: "1m"
      DB_PATH: "/data/data.db"
    volumes:
      - data:/data
      - ./config.json:/app/config.json:ro
volumes:
  data:
```

The database is persisted in a named volume (`data`) and the config file is mounted read-only.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                     Frontend (Svelte)                │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │  Tabs    │  │  Charts  │  │  Auto-refresh     │  │
│  │ (Groups) │  │  (D3.js) │  │  (Configurable)   │  │
│  └──────────┘  └──────────┘  └───────────────────┘  │
└─────────────────────────────────────────────────────┘
                        │ HTTP
                        ▼
┌─────────────────────────────────────────────────────┐
│                   Go Backend                         │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │  SNMP    │  │  BoltDB  │  │  OAuth2 Auth      │  │
│  │  Poller  │  │  Storage │  │  Middleware       │  │
│  └──────────┘  └──────────┘  └───────────────────┘  │
│  ┌───────────────────────────────────────────────┐  │
│  │  REST API (/api/data, /api/graphs, /api/db/*) │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
                        │ SNMP
                        ▼
┌─────────────────────────────────────────────────────┐
│              SNMP Devices (v1/v2c/v3)               │
│  FortiGate sensors · HP switches · Network devices  │
└─────────────────────────────────────────────────────┘
```
