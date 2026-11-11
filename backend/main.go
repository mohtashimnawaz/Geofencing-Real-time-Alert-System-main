package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	CreatedAt   time.Time   `json:"created_at"`
}
type Vehicle struct {
	ID            string    `json:"id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	VehicleType   string    `json:"vehicle_type"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
type Location struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}
type AlertRule struct {
	ID            string    `json:"alert_id"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name,omitempty"`
	VehicleID     string    `json:"vehicle_id,omitempty"`
	VehicleNumber string    `json:"vehicle_number,omitempty"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
type Violation struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	EventType     string    `json:"event_type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timestamp     time.Time `json:"timestamp"`
}
type store struct {
	mu         sync.RWMutex
	geofences  map[string]Geofence
	vehicles   map[string]Vehicle
	locations  map[string]Location
	inside     map[string]map[string]bool
	rules      map[string]AlertRule
	violations []Violation
	next       uint64
	clients    map[*websocket.Conn]struct{}
	dataFile   string
	alerts     chan map[string]any
}

type persistedStore struct {
	Geofences  map[string]Geofence
	Vehicles   map[string]Vehicle
	Locations  map[string]Location
	Inside     map[string]map[string]bool
	Rules      map[string]AlertRule
	Violations []Violation
	Next       uint64
}

func newStore(dataFile string) *store {
	s := &store{geofences: map[string]Geofence{}, vehicles: map[string]Vehicle{}, locations: map[string]Location{}, inside: map[string]map[string]bool{}, rules: map[string]AlertRule{}, clients: map[*websocket.Conn]struct{}{}, dataFile: dataFile, alerts: make(chan map[string]any, 128)}
	if dataFile != "" {
		data, err := os.ReadFile(dataFile)
		if err == nil {
			var saved persistedStore
			if json.Unmarshal(data, &saved) == nil {
				s.geofences, s.vehicles, s.locations, s.inside, s.rules, s.violations, s.next = saved.Geofences, saved.Vehicles, saved.Locations, saved.Inside, saved.Rules, saved.Violations, saved.Next
			} else {
				log.Printf("unable to parse persisted data %q", dataFile)
			}
		} else if !os.IsNotExist(err) {
			log.Printf("unable to read persisted data %q: %v", dataFile, err)
		}
	}
	return s
}
func (s *store) persistLocked() {
	if s.dataFile == "" {
		return
	}
	data, err := json.Marshal(persistedStore{s.geofences, s.vehicles, s.locations, s.inside, s.rules, s.violations, s.next})
	if err != nil {
		log.Printf("unable to serialize persisted data: %v", err)
		return
	}
	tmp := s.dataFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		log.Printf("unable to write persisted data: %v", err)
		return
	}
	if err := os.Rename(tmp, s.dataFile); err != nil {
		log.Printf("unable to replace persisted data: %v", err)
	}
}
func (s *store) id(prefix string) string { s.next++; return fmt.Sprintf("%s_%d", prefix, s.next) }
func writeJSON(w http.ResponseWriter, status int, value any, started time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if m, ok := value.(map[string]any); ok {
		m["time_ns"] = strconv.FormatInt(time.Since(started).Nanoseconds(), 10)
		_ = json.NewEncoder(w).Encode(m)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": value, "time_ns": strconv.FormatInt(time.Since(started).Nanoseconds(), 10)})
}
func errorJSON(w http.ResponseWriter, started time.Time, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message}, started)
}
func decode(r *http.Request, dst any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(dst)
}
func validCategory(v string) bool {
	switch v {
	case "delivery_zone", "restricted_zone", "toll_zone", "customer_area":
		return true
	}
	return false
}
func validPolygon(p [][]float64) bool {
	if len(p) < 4 || len(p[0]) != 2 || len(p[len(p)-1]) != 2 {
		return false
	}
	for _, point := range p {
		if len(point) != 2 || point[0] < -90 || point[0] > 90 || point[1] < -180 || point[1] > 180 {
			return false
		}
	}
	return p[0][0] == p[len(p)-1][0] && p[0][1] == p[len(p)-1][1]
}
func pointInPolygon(lat, lon float64, polygon [][]float64) bool {
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		yi, xi := polygon[i][0], polygon[i][1]
		yj, xj := polygon[j][0], polygon[j][1]
		intersects := ((yi > lat) != (yj > lat)) && (lon < (xj-xi)*(lat-yi)/(yj-yi)+xi)
		if intersects {
			inside = !inside
		}
	}
	return inside
}

type server struct {
	db       *store
	upgrader websocket.Upgrader
}

func (s *server) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch {
	case r.URL.Path == "/geofences":
		s.geofences(w, r)
	case r.URL.Path == "/vehicles":
		s.vehicles(w, r)
	case r.URL.Path == "/vehicles/location":
		s.updateLocation(w, r)
	case strings.HasPrefix(r.URL.Path, "/vehicles/location/"):
		s.currentLocation(w, r)
	case r.URL.Path == "/alerts/configure":
		s.configureAlert(w, r)
	case r.URL.Path == "/alerts":
		s.alerts(w, r)
	case strings.HasPrefix(r.URL.Path, "/alerts/"):
		s.manageAlert(w, r)
	case r.URL.Path == "/violations/history":
		s.history(w, r)
	case r.URL.Path == "/ws/alerts":
		s.websocket(w, r)
	default:
		errorJSON(w, time.Now(), http.StatusNotFound, "endpoint not found")
	}
}
func (s *server) geofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method == "POST" {
		var in struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			Coordinates [][]float64 `json:"coordinates"`
			Category    string      `json:"category"`
		}
		if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" || !validCategory(in.Category) || !validPolygon(in.Coordinates) {
			errorJSON(w, start, 400, "invalid geofence: name, category, and a closed polygon are required")
			return
		}
		s.db.mu.Lock()
		g := Geofence{ID: s.db.id("geo"), Name: strings.TrimSpace(in.Name), Description: in.Description, Coordinates: in.Coordinates, Category: in.Category, CreatedAt: time.Now().UTC()}
		s.db.geofences[g.ID] = g
		s.db.persistLocked()
		s.db.mu.Unlock()
		writeJSON(w, 201, map[string]any{"id": g.ID, "name": g.Name, "status": "active"}, start)
		return
	}
	if r.Method != "GET" {
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	s.db.mu.RLock()
	out := []Geofence{}
	for _, g := range s.db.geofences {
		if c := r.URL.Query().Get("category"); c != "" && c != g.Category {
			continue
		}
		out = append(out, g)
	}
	s.db.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"geofences": out}, start)
}
func (s *server) vehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method == "POST" {
		var in struct {
			VehicleNumber string `json:"vehicle_number"`
			DriverName    string `json:"driver_name"`
			VehicleType   string `json:"vehicle_type"`
			Phone         string `json:"phone"`
		}
		if decode(r, &in) != nil || in.VehicleNumber == "" || in.DriverName == "" || in.VehicleType == "" || in.Phone == "" {
			errorJSON(w, start, 400, "vehicle_number, driver_name, vehicle_type, and phone are required")
			return
		}
		s.db.mu.Lock()
		for _, v := range s.db.vehicles {
			if v.VehicleNumber == in.VehicleNumber {
				s.db.mu.Unlock()
				errorJSON(w, start, 409, "vehicle number already exists")
				return
			}
		}
		v := Vehicle{ID: s.db.id("veh"), VehicleNumber: in.VehicleNumber, DriverName: in.DriverName, VehicleType: in.VehicleType, Phone: in.Phone, Status: "active", CreatedAt: time.Now().UTC()}
		s.db.vehicles[v.ID] = v
		s.db.persistLocked()
		s.db.mu.Unlock()
		writeJSON(w, 201, map[string]any{"id": v.ID, "vehicle_number": v.VehicleNumber, "status": v.Status}, start)
		return
	}
	if r.Method != "GET" {
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	s.db.mu.RLock()
	out := []Vehicle{}
	for _, v := range s.db.vehicles {
		out = append(out, v)
	}
	s.db.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"vehicles": out}, start)
}
func parseTimestamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("timestamp is required")
	}
	return time.Parse(time.RFC3339, raw)
}
func (s *server) updateLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != "POST" {
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	var in struct {
		VehicleID           string `json:"vehicle_id"`
		Latitude, Longitude float64
		Timestamp           string
	}
	if decode(r, &in) != nil || in.VehicleID == "" || in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180 {
		errorJSON(w, start, 400, "invalid location payload")
		return
	}
	ts, e := parseTimestamp(in.Timestamp)
	if e != nil {
		errorJSON(w, start, 400, e.Error())
		return
	}
	s.db.mu.Lock()
	v, ok := s.db.vehicles[in.VehicleID]
	if !ok {
		s.db.mu.Unlock()
		errorJSON(w, start, 404, "vehicle not found")
		return
	}
	old := s.db.inside[in.VehicleID]
	if old == nil {
		old = map[string]bool{}
	}
	current := map[string]bool{}
	matches := []map[string]string{}
	for id, g := range s.db.geofences {
		if pointInPolygon(in.Latitude, in.Longitude, g.Coordinates) {
			current[id] = true
			matches = append(matches, map[string]string{"geofence_id": id, "geofence_name": g.Name, "status": "inside"})
		}
	}
	s.db.locations[in.VehicleID] = Location{in.Latitude, in.Longitude, ts}
	s.db.inside[in.VehicleID] = current
	events := []map[string]any{}
	for id := range current {
		if !old[id] {
			events = append(events, s.makeEventLocked(v, s.db.geofences[id], "entry", in.Latitude, in.Longitude, ts))
		}
	}
	for id := range old {
		if old[id] && !current[id] {
			events = append(events, s.makeEventLocked(v, s.db.geofences[id], "exit", in.Latitude, in.Longitude, ts))
		}
	}
	s.db.persistLocked()
	s.db.mu.Unlock()
	for _, event := range events {
		s.queueAlert(event)
	}
	writeJSON(w, 200, map[string]any{"vehicle_id": in.VehicleID, "location_updated": true, "current_geofences": matches}, start)
}
func (s *server) makeEventLocked(v Vehicle, g Geofence, typ string, lat, lon float64, ts time.Time) map[string]any {
	matched := false
	for _, r := range s.db.rules {
		if r.Status == "active" && r.GeofenceID == g.ID && (r.VehicleID == "" || r.VehicleID == v.ID) && (r.EventType == "both" || r.EventType == typ) {
			matched = true
			break
		}
	}
	vi := Violation{ID: s.db.id("viol"), VehicleID: v.ID, VehicleNumber: v.VehicleNumber, GeofenceID: g.ID, GeofenceName: g.Name, EventType: typ, Latitude: lat, Longitude: lon, Timestamp: ts}
	s.db.violations = append(s.db.violations, vi)
	if !matched {
		return nil
	}
	return map[string]any{"event_id": s.db.id("evt"), "event_type": typ, "timestamp": ts, "vehicle": map[string]string{"vehicle_id": v.ID, "vehicle_number": v.VehicleNumber, "driver_name": v.DriverName}, "geofence": map[string]string{"geofence_id": g.ID, "geofence_name": g.Name, "category": g.Category}, "location": map[string]float64{"latitude": lat, "longitude": lon}}
}
func (s *server) currentLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := strings.TrimPrefix(r.URL.Path, "/vehicles/location/")
	s.db.mu.RLock()
	v, ok := s.db.vehicles[id]
	loc, has := s.db.locations[id]
	gs := []map[string]string{}
	for gid := range s.db.inside[id] {
		g := s.db.geofences[gid]
		gs = append(gs, map[string]string{"geofence_id": gid, "geofence_name": g.Name, "category": g.Category})
	}
	s.db.mu.RUnlock()
	if !ok || !has {
		errorJSON(w, start, 404, "location not found")
		return
	}
	writeJSON(w, 200, map[string]any{"vehicle_id": id, "vehicle_number": v.VehicleNumber, "current_location": loc, "current_geofences": gs}, start)
}
func (s *server) configureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != "POST" {
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	var in struct {
		GeofenceID string `json:"geofence_id"`
		VehicleID  string `json:"vehicle_id"`
		EventType  string `json:"event_type"`
	}
	if decode(r, &in) != nil || in.GeofenceID == "" || (in.EventType != "entry" && in.EventType != "exit" && in.EventType != "both") {
		errorJSON(w, start, 400, "invalid alert rule")
		return
	}
	s.db.mu.Lock()
	_, gok := s.db.geofences[in.GeofenceID]
	_, vok := s.db.vehicles[in.VehicleID]
	if !gok || (in.VehicleID != "" && !vok) {
		s.db.mu.Unlock()
		errorJSON(w, start, 404, "geofence or vehicle not found")
		return
	}
	a := AlertRule{ID: s.db.id("alert"), GeofenceID: in.GeofenceID, VehicleID: in.VehicleID, EventType: in.EventType, Status: "active", CreatedAt: time.Now().UTC()}
	s.db.rules[a.ID] = a
	s.db.persistLocked()
	s.db.mu.Unlock()
	writeJSON(w, 201, map[string]any{"alert_id": a.ID, "geofence_id": a.GeofenceID, "vehicle_id": a.VehicleID, "event_type": a.EventType, "status": a.Status}, start)
}
func (s *server) manageAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := strings.TrimPrefix(r.URL.Path, "/alerts/")
	if id == "" {
		errorJSON(w, start, 404, "alert not found")
		return
	}
	s.db.mu.Lock()
	a, ok := s.db.rules[id]
	if !ok {
		s.db.mu.Unlock()
		errorJSON(w, start, 404, "alert not found")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		delete(s.db.rules, id)
	case http.MethodPatch:
		var in struct {
			Status string `json:"status"`
		}
		if decode(r, &in) != nil || (in.Status != "active" && in.Status != "inactive") {
			s.db.mu.Unlock()
			errorJSON(w, start, 400, "status must be active or inactive")
			return
		}
		a.Status = in.Status
		s.db.rules[id] = a
	default:
		s.db.mu.Unlock()
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	s.db.persistLocked()
	s.db.mu.Unlock()
	writeJSON(w, 200, map[string]any{"alert_id": id, "status": a.Status, "deleted": r.Method == http.MethodDelete}, start)
}
func (s *server) alerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != "GET" {
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	s.db.mu.RLock()
	out := []AlertRule{}
	for _, a := range s.db.rules {
		if q := r.URL.Query().Get("geofence_id"); q != "" && q != a.GeofenceID {
			continue
		}
		if q := r.URL.Query().Get("vehicle_id"); q != "" && q != a.VehicleID {
			continue
		}
		a.GeofenceName = s.db.geofences[a.GeofenceID].Name
		if a.VehicleID != "" {
			a.VehicleNumber = s.db.vehicles[a.VehicleID].VehicleNumber
		}
		out = append(out, a)
	}
	s.db.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"alerts": out}, start)
}
func (s *server) history(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != "GET" {
		errorJSON(w, start, 405, "method not allowed")
		return
	}
	limit := 50
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n > 0 {
		limit = n
	}
	if limit > 500 {
		limit = 500
	}
	var from, to time.Time
	var e error
	if q := r.URL.Query().Get("start_date"); q != "" {
		from, e = time.Parse(time.RFC3339, q)
		if e != nil {
			errorJSON(w, start, 400, "invalid start_date")
			return
		}
	}
	if q := r.URL.Query().Get("end_date"); q != "" {
		to, e = time.Parse(time.RFC3339, q)
		if e != nil {
			errorJSON(w, start, 400, "invalid end_date")
			return
		}
	}
	s.db.mu.RLock()
	out := []Violation{}
	for i := len(s.db.violations) - 1; i >= 0 && len(out) < limit; i-- {
		v := s.db.violations[i]
		if q := r.URL.Query().Get("vehicle_id"); q != "" && q != v.VehicleID {
			continue
		}
		if q := r.URL.Query().Get("geofence_id"); q != "" && q != v.GeofenceID {
			continue
		}
		if !from.IsZero() && v.Timestamp.Before(from) || !to.IsZero() && v.Timestamp.After(to) {
			continue
		}
		out = append(out, v)
	}
	total := len(out)
	s.db.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"violations": out, "total_count": total}, start)
}
func (s *server) websocket(w http.ResponseWriter, r *http.Request) {
	c, e := s.upgrader.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	s.db.mu.Lock()
	s.db.clients[c] = struct{}{}
	s.db.mu.Unlock()
	defer func() { s.db.mu.Lock(); delete(s.db.clients, c); s.db.mu.Unlock(); c.Close() }()
	for {
		if _, _, e = c.ReadMessage(); e != nil {
			return
		}
	}
}
func (s *server) broadcast(event map[string]any) {
	if event == nil {
		return
	}
	s.db.mu.RLock()
	defer s.db.mu.RUnlock()
	for c := range s.db.clients {
		_ = c.WriteJSON(event)
	}
}
func (s *server) queueAlert(event map[string]any) {
	if event == nil {
		return
	}
	select {
	case s.db.alerts <- event:
	default:
		log.Printf("dropping alert because the delivery queue is full")
	}
}
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "geopulse-data.json"
	}
	s := &server{db: newStore(dataFile), upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}}
	go func() {
		for event := range s.db.alerts {
			s.broadcast(event)
		}
	}()
	log.Printf("geofencing API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, http.HandlerFunc(s.handler)))
}
