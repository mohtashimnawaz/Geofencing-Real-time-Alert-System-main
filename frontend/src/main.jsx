import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";

const API = import.meta.env.VITE_API_URL || "http://localhost:8080";
async function request(path, options = {}) {
  const res = await fetch(API + path, { headers: { "Content-Type": "application/json" }, ...options });
  const body = await res.json();
  if (!res.ok) throw new Error(body.error || "Request failed");
  return body;
}
function App() {
  const [geofences, setGeofences] = useState([]), [vehicles, setVehicles] = useState([]), [alerts, setAlerts] = useState([]);
  const [violations, setViolations] = useState([]), [events, setEvents] = useState([]), [message, setMessage] = useState("");
  const [geo, setGeo] = useState({ name: "", description: "", category: "restricted_zone", coordinates: "37.78,-122.42\n37.79,-122.42\n37.79,-122.41\n37.78,-122.41\n37.78,-122.42" });
  const [vehicle, setVehicle] = useState({ vehicle_number: "", driver_name: "", vehicle_type: "car", phone: "" });
  const [location, setLocation] = useState({ vehicle_id: "", latitude: "37.785", longitude: "-122.415", timestamp: new Date().toISOString().slice(0, 19) + "Z" });
  const [rule, setRule] = useState({ geofence_id: "", vehicle_id: "", event_type: "both" });
  const refresh = async () => { try { const [g,v,a,h] = await Promise.all([request("/geofences"), request("/vehicles"), request("/alerts"), request("/violations/history")]); setGeofences(g.geofences); setVehicles(v.vehicles); setAlerts(a.alerts); setViolations(h.violations); } catch (e) { setMessage(e.message); } };
  useEffect(() => { refresh(); const ws = new WebSocket(API.replace(/^http/, "ws") + "/ws/alerts"); ws.onmessage = e => { setEvents(x => [JSON.parse(e.data), ...x].slice(0, 20)); refresh(); }; return () => ws.close(); }, []);
  const submit = async (path, body, done) => { try { await request(path, { method: "POST", body: JSON.stringify(body) }); setMessage("Saved successfully"); done?.(); refresh(); } catch (e) { setMessage(e.message); } };
  const coordinates = useMemo(() => geo.coordinates.split("\n").map(x => x.split(",").map(Number)), [geo.coordinates]);
  return <main><header><div><h1>GeoPulse</h1><p>Real-time geofencing and fleet safety</p></div><span className="live">● LIVE</span></header>
    {message && <div className="notice" onClick={() => setMessage("")}>{message}</div>}
    <section className="grid">
      <article><h2>Create geofence</h2><input placeholder="Name" value={geo.name} onChange={e => setGeo({...geo,name:e.target.value})}/><input placeholder="Description" value={geo.description} onChange={e => setGeo({...geo,description:e.target.value})}/><select value={geo.category} onChange={e => setGeo({...geo,category:e.target.value})}>{["restricted_zone","delivery_zone","toll_zone","customer_area"].map(x=><option key={x}>{x}</option>)}</select><textarea value={geo.coordinates} onChange={e => setGeo({...geo,coordinates:e.target.value})}/><small>One latitude,longitude pair per line. Repeat the first point to close.</small><button onClick={() => submit("/geofences",{...geo,coordinates},()=>setGeo({...geo,name:"",description:"",category:"restricted_zone",coordinates:geo.coordinates}))}>Create geofence</button></article>
      <article><h2>Register vehicle</h2>{Object.keys(vehicle).map(k=><input key={k} placeholder={k.replace("_"," ")} value={vehicle[k]} onChange={e=>setVehicle({...vehicle,[k]:e.target.value})}/>)}<button onClick={()=>submit("/vehicles",vehicle,()=>setVehicle({vehicle_number:"",driver_name:"",vehicle_type:"car",phone:""}))}>Register vehicle</button></article>
      <article><h2>Update location</h2><select value={location.vehicle_id} onChange={e=>setLocation({...location,vehicle_id:e.target.value})}><option value="">Select vehicle</option>{vehicles.map(v=><option key={v.id} value={v.id}>{v.vehicle_number}</option>)}</select><input type="number" step="any" value={location.latitude} onChange={e=>setLocation({...location,latitude:e.target.value})}/><input type="number" step="any" value={location.longitude} onChange={e=>setLocation({...location,longitude:e.target.value})}/><input type="datetime-local" value={location.timestamp.slice(0,16)} onChange={e=>setLocation({...location,timestamp:new Date(e.target.value).toISOString()})}/><button onClick={()=>submit("/vehicles/location",{...location,latitude:Number(location.latitude),longitude:Number(location.longitude)})}>Send location</button></article>
      <article><h2>Configure alert</h2><select value={rule.geofence_id} onChange={e=>setRule({...rule,geofence_id:e.target.value})}><option value="">Select geofence</option>{geofences.map(g=><option key={g.id} value={g.id}>{g.name}</option>)}</select><select value={rule.vehicle_id} onChange={e=>setRule({...rule,vehicle_id:e.target.value})}><option value="">Every vehicle</option>{vehicles.map(v=><option key={v.id} value={v.id}>{v.vehicle_number}</option>)}</select><select value={rule.event_type} onChange={e=>setRule({...rule,event_type:e.target.value})}><option>both</option><option>entry</option><option>exit</option></select><button onClick={()=>submit("/alerts/configure",rule)}>Activate rule</button></article>
    </section>
    <section className="columns"><article><h2>Geofences ({geofences.length})</h2>{geofences.map(g=><div className="row" key={g.id}><b>{g.name}</b><span>{g.category}</span></div>)}</article><article><h2>Vehicles ({vehicles.length})</h2>{vehicles.map(v=><div className="row" key={v.id}><b>{v.vehicle_number}</b><span>{v.driver_name}</span></div>)}</article><article><h2>Alert rules ({alerts.length})</h2>{alerts.map(a=><div className="row" key={a.alert_id}><b>{a.event_type}</b><span>{a.geofence_name}</span></div>)}</article></section>
    <section className="columns"><article className="wide"><h2>Live alerts</h2>{events.length ? events.map((e,i)=><div className="event" key={i}><strong>{e.event_type.toUpperCase()}</strong> {e.vehicle.vehicle_number} → {e.geofence.geofence_name}<small>{new Date(e.timestamp).toLocaleString()}</small></div>) : <p className="muted">Configure a rule, then send a vehicle into or out of a geofence.</p>}</article><article className="wide"><h2>Violation history</h2>{violations.map(v=><div className="event" key={v.id}><strong>{v.event_type}</strong> {v.vehicle_number} / {v.geofence_name}<small>{new Date(v.timestamp).toLocaleString()}</small></div>)}</article></section>
  </main>;
}
createRoot(document.getElementById("root")).render(<App />);
