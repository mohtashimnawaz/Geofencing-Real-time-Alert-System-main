# GeoPulse setup

## Prerequisites

- Go 1.22 or newer
- Node.js 20 or newer and npm
- Docker and Docker Compose (optional)

## Run locally

Start the API:

```bash
cd backend
go mod tidy
go run .
```

In another terminal, start the dashboard:

```bash
cd frontend
npm install
npm run dev
```

Open <http://localhost:5173>. The API listens on `http://localhost:8080`.
The backend persists its in-memory repository to `geopulse-data.json` by default.
Set `DATA_FILE` to change the location, or set it to an empty value to disable
persistence. Docker Compose stores this file in the named `geopulse-data`
volume. For production scale, replace the store with PostgreSQL/PostGIS.

## Docker Compose

```bash
docker compose up --build
```

Open <http://localhost:3000>. Stop the services with `docker compose down`.

## API examples

Create a closed polygon:

```bash
curl -X POST http://localhost:8080/geofences \
  -H 'Content-Type: application/json' \
  -d '{"name":"Restricted zone","description":"Test area","category":"restricted_zone","coordinates":[[37.78,-122.42],[37.79,-122.42],[37.79,-122.41],[37.78,-122.41],[37.78,-122.42]]}'
```

Register a vehicle, then use its returned ID:

```bash
curl -X POST http://localhost:8080/vehicles \
  -H 'Content-Type: application/json' \
  -d '{"vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}'
```

Configure an alert with the geofence and vehicle IDs:

```bash
curl -X POST http://localhost:8080/alerts/configure \
  -H 'Content-Type: application/json' \
  -d '{"geofence_id":"geo_1","vehicle_id":"veh_2","event_type":"both"}'
```

Send a location update:

```bash
curl -X POST http://localhost:8080/vehicles/location \
  -H 'Content-Type: application/json' \
  -d '{"vehicle_id":"veh_2","latitude":37.785,"longitude":-122.415,"timestamp":"2025-01-15T10:35:00Z"}'
```

Other endpoints are `GET /geofences`, `GET /vehicles`,
`GET /vehicles/location/{vehicle_id}`, `GET /alerts`, and
`GET /violations/history`. Alert rules can be disabled with
`PATCH /alerts/{alert_id}` and removed with `DELETE /alerts/{alert_id}`.
Every JSON response includes `time_ns`. Connect a WebSocket client to
`/ws/alerts` to receive matching entry and exit events.

The dashboard includes a Leaflet map, category and history filters, vehicle
status cards, alert rule management, loading states, and paginated violation
history. The map uses OpenStreetMap tiles and needs network access in the
browser.

## Architecture

The Go HTTP service validates request payloads, performs latitude/longitude
point-in-polygon checks, tracks transitions per vehicle, and broadcasts matching
events to all connected WebSocket clients. The React/Vite dashboard provides
forms for each operation and a live event feed. CORS is enabled for local
development.
