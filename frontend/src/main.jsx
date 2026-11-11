import React, { useCallback, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { CircleMarker, MapContainer, Polygon, Popup, TileLayer, useMapEvents } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import "./style.css";

const API = import.meta.env.VITE_API_URL || "http://localhost:8080";
const categories = ["all", "restricted_zone", "delivery_zone", "toll_zone", "customer_area"];
const emptyVehicle = { vehicle_number: "", driver_name: "", vehicle_type: "car", phone: "" };

async function request(path, options = {}) {
  const response = await fetch(`${API}${path}`, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || "Request failed");
  return body;
}

function MapClick({ onClick }) {
  useMapEvents({ click: (event) => onClick(event.latlng) });
  return null;
}

function formatDate(value) {
  return value ? new Date(value).toLocaleString() : "—";
}

function App() {
  const [geofences, setGeofences] = useState([]);
  const [vehicles, setVehicles] = useState([]);
  const [locations, setLocations] = useState({});
  const [alerts, setAlerts] = useState([]);
  const [violations, setViolations] = useState([]);
  const [events, setEvents] = useState([]);
  const [category, setCategory] = useState("all");
  const [historyFilters, setHistoryFilters] = useState({ vehicle_id: "", geofence_id: "", start_date: "", end_date: "" });
  const [historyPage, setHistoryPage] = useState(1);
  const [geo, setGeo] = useState({ name: "", description: "", category: "restricted_zone", coordinates: "37.78,-122.42\n37.79,-122.42\n37.79,-122.41\n37.78,-122.41\n37.78,-122.42" });
  const [vehicle, setVehicle] = useState(emptyVehicle);
  const [location, setLocation] = useState({ vehicle_id: "", latitude: "37.785", longitude: "-122.415", timestamp: new Date().toISOString().slice(0, 19) + "Z" });
  const [rule, setRule] = useState({ geofence_id: "", vehicle_id: "", event_type: "both" });
  const [loading, setLoading] = useState({});
  const [message, setMessage] = useState("");

  const busy = (key) => !!loading[key];
  const run = async (key, action) => {
    setLoading((current) => ({ ...current, [key]: true }));
    try { return await action(); } catch (error) { setMessage(error.message); return null; }
    finally { setLoading((current) => ({ ...current, [key]: false })); }
  };

  const loadLocations = useCallback(async (list) => {
    const results = await Promise.all(list.map(async (v) => {
      try { return [v.id, await request(`/vehicles/location/${v.id}`)]; } catch { return [v.id, null]; }
    }));
    setLocations(Object.fromEntries(results.filter(([, value]) => value)));
  }, []);

  const refresh = useCallback(async () => {
    const [g, v, a] = await Promise.all([request("/geofences"), request("/vehicles"), request("/alerts")]);
    setGeofences(g.geofences || []); setVehicles(v.vehicles || []); setAlerts(a.alerts || []);
    await loadLocations(v.vehicles || []);
  }, [loadLocations]);

  const loadHistory = useCallback(async (filters = historyFilters) => {
    const params = new URLSearchParams({ limit: "500" });
    Object.entries(filters).forEach(([key, value]) => {
      if (value) params.set(key, key.endsWith("date") ? new Date(`${value}T00:00:00Z`).toISOString() : value);
    });
    const data = await request(`/violations/history?${params}`);
    setViolations(data.violations || []); setHistoryPage(1);
  }, [historyFilters]);

  useEffect(() => {
    run("initial", async () => { await refresh(); await loadHistory(); });
    const websocket = new WebSocket(API.replace(/^http/, "ws") + "/ws/alerts");
    websocket.onmessage = (event) => {
      const alert = JSON.parse(event.data);
      setEvents((current) => [alert, ...current].slice(0, 30));
      setMessage(`${alert.event_type.toUpperCase()} alert: ${alert.vehicle?.vehicle_number} entered ${alert.geofence?.geofence_name}`);
      refresh().catch(() => {});
      loadHistory().catch(() => {});
    };
    websocket.onerror = () => setMessage("Live alerts are unavailable; the dashboard will continue to poll on refresh.");
    return () => websocket.close();
  }, [refresh, loadHistory]);

  const submit = async (key, path, body, done) => {
    const result = await run(key, async () => {
      await request(path, { method: "POST", body: JSON.stringify(body) });
      setMessage("Saved successfully"); done?.(); await refresh(); await loadHistory();
    });
    return result;
  };

  const filteredGeofences = useMemo(() => category === "all" ? geofences : geofences.filter((g) => g.category === category), [category, geofences]);
  const polygonCoordinates = useMemo(() => geo.coordinates.split("\n").map((line) => line.split(",").map(Number)).filter((point) => point.length === 2 && point.every(Number.isFinite)), [geo.coordinates]);
  const mapCenter = useMemo(() => {
    const first = filteredGeofences[0]?.coordinates?.[0] || [37.785, -122.415];
    return first;
  }, [filteredGeofences]);
  const pageSize = 8;
  const visibleViolations = violations.slice((historyPage - 1) * pageSize, historyPage * pageSize);
  const totalPages = Math.max(1, Math.ceil(violations.length / pageSize));

  const setMapLocation = (latlng) => setLocation((current) => ({ ...current, latitude: latlng.lat.toFixed(6), longitude: latlng.lng.toFixed(6) }));

  return <main>
    <header><div><h1>GeoPulse</h1><p>Real-time geofencing and fleet safety</p></div><span className="live">● LIVE</span></header>
    {message && <div className="notice" role="status" onClick={() => setMessage("")}>{message}</div>}
    {busy("initial") && <div className="loading-bar">Loading dashboard…</div>}

    <section className="map-panel">
      <div className="section-heading"><div><h2>Fleet map</h2><p>Click the map to set the next vehicle location.</p></div><select value={category} onChange={(e) => setCategory(e.target.value)} aria-label="Filter map geofences">{categories.map((item) => <option key={item} value={item}>{item === "all" ? "All categories" : item.replace("_", " ")}</option>)}</select></div>
      <MapContainer center={mapCenter} zoom={13} scrollWheelZoom className="map"><TileLayer attribution='&copy; OpenStreetMap contributors' url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" /><MapClick onClick={setMapLocation} />
        {filteredGeofences.map((g) => <Polygon key={g.id} positions={g.coordinates} pathOptions={{ color: g.category === "restricted_zone" ? "#ff7084" : "#4fc9a3", fillOpacity: .18 }}><Popup><b>{g.name}</b><br />{g.category}<br />{g.description}</Popup></Polygon>)}
        {Object.values(locations).map((item) => <CircleMarker key={item.vehicle_id} center={[item.current_location.latitude, item.current_location.longitude]} radius={8} pathOptions={{ color: "#ffd166", fillColor: "#ffd166", fillOpacity: .9 }}><Popup><b>{item.vehicle_number}</b><br />{formatDate(item.current_location.timestamp)}<br />{item.current_geofences?.length ? item.current_geofences.map((g) => g.geofence_name).join(", ") : "Outside geofences"}</Popup></CircleMarker>)}
      </MapContainer>
      <div className="map-legend"><span><i className="legend restricted" />Restricted</span><span><i className="legend zone" />Other geofence</span><span><i className="legend vehicle" />Vehicle</span></div>
    </section>

    <section className="grid">
      <article><h2>Create geofence</h2><input placeholder="Name" value={geo.name} onChange={(e) => setGeo({ ...geo, name: e.target.value })} /><input placeholder="Description" value={geo.description} onChange={(e) => setGeo({ ...geo, description: e.target.value })} /><select value={geo.category} onChange={(e) => setGeo({ ...geo, category: e.target.value })}>{categories.slice(1).map((x) => <option key={x}>{x}</option>)}</select><textarea value={geo.coordinates} onChange={(e) => setGeo({ ...geo, coordinates: e.target.value })} /><small>One latitude,longitude pair per line. Repeat the first point to close.</small><button disabled={busy("geofence")} onClick={() => submit("geofence", "/geofences", { ...geo, coordinates: polygonCoordinates }, () => setGeo({ ...geo, name: "", description: "" }))}>{busy("geofence") ? "Creating…" : "Create geofence"}</button></article>
      <article><h2>Register vehicle</h2>{Object.keys(vehicle).map((key) => <input key={key} placeholder={key.replace("_", " ")} value={vehicle[key]} onChange={(e) => setVehicle({ ...vehicle, [key]: e.target.value })} />)}<button disabled={busy("vehicle")} onClick={() => submit("vehicle", "/vehicles", vehicle, () => setVehicle(emptyVehicle))}>{busy("vehicle") ? "Registering…" : "Register vehicle"}</button></article>
      <article><h2>Update location</h2><select value={location.vehicle_id} onChange={(e) => setLocation({ ...location, vehicle_id: e.target.value })}><option value="">Select vehicle</option>{vehicles.map((v) => <option key={v.id} value={v.id}>{v.vehicle_number}</option>)}</select><input aria-label="Latitude" type="number" step="any" value={location.latitude} onChange={(e) => setLocation({ ...location, latitude: e.target.value })} /><input aria-label="Longitude" type="number" step="any" value={location.longitude} onChange={(e) => setLocation({ ...location, longitude: e.target.value })} /><input aria-label="Timestamp" type="datetime-local" value={location.timestamp.slice(0, 16)} onChange={(e) => setLocation({ ...location, timestamp: new Date(e.target.value).toISOString() })} /><button disabled={busy("location")} onClick={() => submit("location", "/vehicles/location", { ...location, latitude: Number(location.latitude), longitude: Number(location.longitude) })}>{busy("location") ? "Sending…" : "Send location"}</button></article>
      <article><h2>Configure alert</h2><select value={rule.geofence_id} onChange={(e) => setRule({ ...rule, geofence_id: e.target.value })}><option value="">Select geofence</option>{geofences.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}</select><select value={rule.vehicle_id} onChange={(e) => setRule({ ...rule, vehicle_id: e.target.value })}><option value="">Every vehicle</option>{vehicles.map((v) => <option key={v.id} value={v.id}>{v.vehicle_number}</option>)}</select><select value={rule.event_type} onChange={(e) => setRule({ ...rule, event_type: e.target.value })}><option>both</option><option>entry</option><option>exit</option></select><button disabled={busy("rule")} onClick={() => submit("rule", "/alerts/configure", rule)}>{busy("rule") ? "Activating…" : "Activate rule"}</button></article>
    </section>

    <section className="columns"><article><div className="section-heading"><h2>Geofences ({filteredGeofences.length})</h2><select value={category} onChange={(e) => setCategory(e.target.value)}>{categories.map((x) => <option key={x}>{x}</option>)}</select></div>{filteredGeofences.map((g) => <div className="row" key={g.id}><b>{g.name}</b><span>{g.category}</span></div>)}</article><article><h2>Vehicles ({vehicles.length})</h2>{vehicles.map((v) => { const current = locations[v.id]; return <div className="row" key={v.id}><div><b>{v.vehicle_number}</b><small>{v.driver_name}</small></div><span>{current?.current_geofences?.length ? current.current_geofences.map((g) => g.geofence_name).join(", ") : current ? "Outside geofences" : "No location"}</span></div>; })}</article><article><h2>Alert rules ({alerts.length})</h2>{alerts.map((a) => <div className="row rule-row" key={a.alert_id}><div><b>{a.event_type}</b><small>{a.geofence_name} · {a.vehicle_number || "Every vehicle"}</small></div><div className="actions"><button className="subtle" disabled={busy(a.alert_id)} onClick={() => run(a.alert_id, async () => { await request(`/alerts/${a.alert_id}`, { method: "PATCH", body: JSON.stringify({ status: a.status === "active" ? "inactive" : "active" }) }); await refresh(); })}>{a.status === "active" ? "Deactivate" : "Activate"}</button><button className="danger" disabled={busy(a.alert_id)} onClick={() => run(a.alert_id, async () => { await request(`/alerts/${a.alert_id}`, { method: "DELETE" }); await refresh(); })}>Delete</button></div></div>)}</article></section>

    <section className="columns"><article className="wide"><div className="section-heading"><h2>Live alerts</h2><span className="pill">{events.length}</span></div>{events.length ? events.map((event, index) => <div className="event" key={`${event.event_id || index}`}><strong>{event.event_type.toUpperCase()}</strong> {event.vehicle?.vehicle_number} → {event.geofence?.geofence_name}<small>{formatDate(event.timestamp)} · {event.location?.latitude?.toFixed?.(5)}, {event.location?.longitude?.toFixed?.(5)}</small></div>) : <p className="muted">Configure a rule, then send a vehicle into or out of a geofence.</p>}</article><article className="wide"><div className="section-heading"><h2>Violation history</h2><span className="pill">{violations.length}</span></div><div className="filters"><select value={historyFilters.vehicle_id} onChange={(e) => setHistoryFilters({ ...historyFilters, vehicle_id: e.target.value })}><option value="">All vehicles</option>{vehicles.map((v) => <option key={v.id} value={v.id}>{v.vehicle_number}</option>)}</select><select value={historyFilters.geofence_id} onChange={(e) => setHistoryFilters({ ...historyFilters, geofence_id: e.target.value })}><option value="">All geofences</option>{geofences.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}</select><input type="date" value={historyFilters.start_date} onChange={(e) => setHistoryFilters({ ...historyFilters, start_date: e.target.value })} /><input type="date" value={historyFilters.end_date} onChange={(e) => setHistoryFilters({ ...historyFilters, end_date: e.target.value })} /><button disabled={busy("history")} onClick={() => run("history", () => loadHistory())}>{busy("history") ? "Filtering…" : "Apply filters"}</button></div>{visibleViolations.map((v) => <div className="event" key={v.id}><strong>{v.event_type.toUpperCase()}</strong> {v.vehicle_number} / {v.geofence_name}<small>{formatDate(v.timestamp)} · {v.latitude.toFixed(5)}, {v.longitude.toFixed(5)}</small></div>)}{!visibleViolations.length && <p className="muted">No violations match these filters.</p>}<div className="pagination"><button disabled={historyPage === 1} onClick={() => setHistoryPage((page) => page - 1)}>Previous</button><span>Page {historyPage} of {totalPages}</span><button disabled={historyPage >= totalPages} onClick={() => setHistoryPage((page) => page + 1)}>Next</button></div></article></section>
  </main>;
}

createRoot(document.getElementById("root")).render(<App />);
