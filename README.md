# Full-Stack Developer Assessment: Geofencing & Real-time Alert System

## 📋 Overview

This assessment evaluates your ability to build a complete full-stack application with real-time capabilities. You will develop:

1. **Backend API (Go)** - RESTful API for geofencing and vehicle tracking
2. **Real-time Alert System (Go)** - Alert notification system (WebSocket or alternative approach)
3. **Frontend Application (React)** - Web interface to interact with the system and receive live alerts

You will build a geofencing and vehicle tracking system where users can define virtual boundaries (geofences) around specific areas and track vehicles in real-time. When a vehicle enters or exits a geofenced zone—especially restricted areas—the system sends immediate alerts. Users can create geofences by drawing boundaries on a map or providing coordinates, register and track vehicles, view historical movements, configure alert rules for different vehicles and zones, and access everything through an intuitive web dashboard with live updates.

## 🎯 Objective

Build a complete geofencing and vehicle tracking system that:
- Manages geofences (virtual boundaries) and vehicle locations
- Tracks when vehicles enter or exit geofenced areas
- Sends real-time alerts when vehicles enter restricted zones
- Provides a user-friendly web interface for system interaction

## 🛠️ Technology Stack

### Backend
- **Language**: GoLang
- **Database**: Your choice (PostgreSQL with PostGIS recommended for geospatial operations)
- **Real-time Alerts**: WebSocket or alternative approach of your choice (Server-Sent Events, polling, etc.)

### Frontend
- **Framework**: React (you may use Next.js, Create React App, or Vite)
- **Styling**: Your choice (Tailwind CSS, Material-UI, CSS Modules, etc.)
- **Map Library**: Your choice (Leaflet, Mapbox, or Google Maps for interactive geofence creation)

### Infrastructure
- **Containerization**: Docker & Docker Compose
- **Deployment**: Backend (containerized), Frontend (Vercel, Netlify, etc.)

---

## 📡 Backend Requirements

### API Response Format

**IMPORTANT**: Every API response must include execution time in nanoseconds:

```json
{
  // ... response data
  "time_ns": "1234567"
}
```

Measure time using Go's `time` package at the start and end of each request handler.

---

## 🔌 API Endpoints

> **Note**: Do not modify endpoint paths, HTTP methods, or naming conventions. Assessment evaluation is strict on endpoint structure.

### 1. POST /geofences
Create a new geofence with polygonal boundaries.

**Request Body**:
```json
{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area for downtown customers",
  "coordinates": [
    [37.7749, -122.4194],
    [37.7849, -122.4194],
    [37.7849, -122.4094],
    [37.7749, -122.4094],
    [37.7749, -122.4194]
  ],
  "category": "delivery_zone"
}
```

**Field Descriptions**:
- `name` (string, required): Human-readable name for the geofence
- `description` (string, optional): Detailed description of the geofence purpose
- `coordinates` (array, required): Array of `[latitude, longitude]` coordinate pairs defining the polygon boundary
- `category` (string, required): Type of geofence - one of: `delivery_zone`, `restricted_zone`, `toll_zone`, `customer_area`

**Validation Rules**:
- `coordinates`: Array of `[latitude, longitude]` pairs
- First and last coordinates must be identical (closed polygon)
- Minimum 4 points (3 unique + 1 closing point)
- Latitude must be between -90 and 90
- Longitude must be between -180 and 180

**Response**:
```json
{
  "id": "geo_123",
  "name": "Downtown Delivery Zone",
  "status": "active",
  "time_ns": "1234567"
}
```

---

### 2. GET /geofences
Retrieve all geofences with optional filtering.

**Query Parameters**:
- `category` (optional): Filter by geofence category

**Example**: `GET /geofences?category=delivery_zone`

**Response**:
```json
{
  "geofences": [
    {
      "id": "geo_123",
      "name": "Downtown Delivery Zone",
      "description": "Main delivery area for downtown customers",
      "coordinates": [[37.7749, -122.4194], ...],
      "category": "delivery_zone",
      "created_at": "2025-01-15T10:30:00Z"
    }
  ],
  "time_ns": "987654"
}
```

---

### 3. POST /vehicles
Register a new vehicle in the system.

**Request Body**:
```json
{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}
```

**Field Descriptions**:
- `vehicle_number` (string, required): Unique vehicle registration/identification number
- `driver_name` (string, required): Name of the assigned driver
- `vehicle_type` (string, required): Type of vehicle (e.g., truck, car, van, motorcycle)
- `phone` (string, required): Contact phone number for the driver

**Response**:
```json
{
  "id": "veh_456",
  "vehicle_number": "KA-01-AB-1234",
  "status": "active",
  "time_ns": "1123456"
}
```

---

### 4. GET /vehicles
Retrieve all registered vehicles.

**Response**:
```json
{
  "vehicles": [
    {
      "id": "veh_456",
      "vehicle_number": "KA-01-AB-1234",
      "driver_name": "John Doe",
      "vehicle_type": "truck",
      "phone": "+1234567890",
      "status": "active",
      "created_at": "2025-01-15T09:00:00Z"
    }
  ],
  "time_ns": "876543"
}
```

---

### 5. POST /vehicles/location
Update vehicle location and check geofence status. **This endpoint triggers alerts when vehicles enter restricted zones.**

**Request Body**:
```json
{
  "vehicle_id": "veh_456",
  "latitude": 37.7849,
  "longitude": -122.4194,
  "timestamp": "2025-01-15T10:35:00Z"
}
```

**Field Descriptions**:
- `vehicle_id` (string, required): ID of the vehicle being updated
- `latitude` (float, required): Current latitude position (-90 to 90)
- `longitude` (float, required): Current longitude position (-180 to 180)
- `timestamp` (string, required): ISO 8601 timestamp of when the location was recorded

**Response**:
```json
{
  "vehicle_id": "veh_456",
  "location_updated": true,
  "current_geofences": [
    {
      "geofence_id": "geo_123",
      "geofence_name": "Downtown Delivery Zone",
      "status": "inside"
    }
  ],
  "time_ns": "2345678"
}
```

**Business Logic**:
- Store location update in database
- Check if vehicle is inside any geofences
- Detect entry/exit events by comparing with previous location state
- **Trigger real-time alerts for configured geofence events** (especially for restricted zones)
- Return all current geofences containing the vehicle

---

### 6. GET /vehicles/location/{vehicle_id}
Get current location and geofence status for a specific vehicle.

**Response**:
```json
{
  "vehicle_id": "veh_456",
  "vehicle_number": "KA-01-AB-1234",
  "current_location": {
    "latitude": 37.7849,
    "longitude": -122.4194,
    "timestamp": "2025-01-15T10:35:00Z"
  },
  "current_geofences": [
    {
      "geofence_id": "geo_123",
      "geofence_name": "Downtown Delivery Zone",
      "category": "delivery_zone"
    }
  ],
  "time_ns": "876543"
}
```

---

### 7. POST /alerts/configure
Configure alert rules for geofence events.

**Request Body**:
```json
{
  "geofence_id": "geo_123",
  "vehicle_id": "veh_456",
  "event_type": "entry"
}
```

**Field Descriptions**:
- `geofence_id` (string, required): ID of the geofence to monitor
- `vehicle_id` (string, optional): ID of specific vehicle to monitor. If omitted, alert applies to all vehicles
- `event_type` (string, required): Type of event to trigger alert - one of: `entry`, `exit`, `both`

**Response**:
```json
{
  "alert_id": "alert_789",
  "geofence_id": "geo_123",
  "vehicle_id": "veh_456",
  "event_type": "entry",
  "status": "active",
  "time_ns": "1567890"
}
```

---

### 8. GET /alerts
Retrieve all configured alert rules.

**Query Parameters**:
- `geofence_id` (optional): Filter alerts by geofence
- `vehicle_id` (optional): Filter alerts by vehicle

**Response**:
```json
{
  "alerts": [
    {
      "alert_id": "alert_789",
      "geofence_id": "geo_123",
      "geofence_name": "Downtown Delivery Zone",
      "vehicle_id": "veh_456",
      "vehicle_number": "KA-01-AB-1234",
      "event_type": "entry",
      "status": "active",
      "created_at": "2025-01-15T09:15:00Z"
    }
  ],
  "time_ns": "654321"
}
```

---

### 9. GET /violations/history
Retrieve historical geofence entry/exit events.

**Query Parameters**:
- `vehicle_id` (optional): Filter by vehicle
- `geofence_id` (optional): Filter by geofence
- `start_date` (optional): ISO 8601 format (e.g., `2025-01-01T00:00:00Z`)
- `end_date` (optional): ISO 8601 format
- `limit` (optional): Number of records (default: 50, max: 500)

**Example**: `GET /violations/history?vehicle_id=veh_456&limit=100`

**Response**:
```json
{
  "violations": [
    {
      "id": "viol_111",
      "vehicle_id": "veh_456",
      "vehicle_number": "KA-01-AB-1234",
      "geofence_id": "geo_123",
      "geofence_name": "Downtown Delivery Zone",
      "event_type": "entry",
      "latitude": 37.7849,
      "longitude": -122.4194,
      "timestamp": "2025-01-15T10:35:00Z"
    }
  ],
  "total_count": 245,
  "time_ns": "3456789"
}
```

---

## 🔴 Real-time Alert System

You need to implement a real-time alert notification system. When `POST /vehicles/location` detects that a vehicle has entered or exited a geofenced area (based on configured alert rules), the system should immediately notify connected clients.

### Recommended Approach: WebSocket

**WebSocket Endpoint**: `/ws/alerts`

Implement a WebSocket server that:
1. Accepts connections from frontend clients
2. Broadcasts real-time alerts when geofence events occur
3. Supports multiple concurrent connections
4. Handles connection lifecycle (connect, disconnect, reconnect)

**WebSocket Message Format**:
```json
{
  "event_id": "evt_999",
  "event_type": "entry",
  "timestamp": "2025-01-15T10:35:00Z",
  "vehicle": {
    "vehicle_id": "veh_456",
    "vehicle_number": "KA-01-AB-1234",
    "driver_name": "John Doe"
  },
  "geofence": {
    "geofence_id": "geo_123",
    "geofence_name": "Downtown Delivery Zone",
    "category": "delivery_zone"
  },
  "location": {
    "latitude": 37.7849,
    "longitude": -122.4194
  }
}
```

**WebSocket Libraries**:
- Gorilla WebSocket (popular choice for Go)
- nhooyr.io/websocket

### Alternative Approaches

You may use alternative real-time communication methods if you prefer

<!-- 1. **Server-Sent Events (SSE)**: One-way server-to-client streaming
2. **Long Polling**: Client repeatedly polls for new events
3. **HTTP/2 Server Push**: Push notifications over HTTP/2
4. **Message Queue + Polling**: Store alerts in queue, client polls periodically -->

**Note**: WebSocket is recommended for true bi-directional real-time communication, but you're free to choose the approach that best fits your architecture.

### Integration Requirements

Regardless of the approach chosen:
- When `POST /vehicles/location` detects an entry/exit event, check for matching alert configurations
- Generate an alert with the format shown above
- Deliver the alert to connected clients in real-time
- Handle the alert delivery asynchronously (don't block the HTTP response)
- Store all alerts in the database for historical records

---

## 🎨 Frontend Requirements

Build a React-based web application with the following capabilities:

### Required Features

#### 1. **Geofence Management**
- Provide a way to create new geofences with name, description, category, and coordinates
- Display list of all geofences with filtering by category
- **Preferred approach**: Interactive map where users can see polygons that define geofence boundaries and vehicle locations.

#### 2. **Vehicle Management**
- Provide a way to register new vehicles with vehicle number, driver name, vehicle type, and phone
- Display list of all registered vehicles
- Show current location and geofence status for each vehicle

#### 3. **Location Updates**
- Provide a way to update vehicle locations with latitude, longitude, and timestamp
- Display which geofences the vehicle is currently inside
- **Preferred approach**: Interactive map where users can click to set vehicle location

#### 4. **Alert Configuration**
- Provide a way to configure alert rules by selecting geofence, vehicle (optional), and event type
- Display all configured alert rules
- Ability to view and manage existing alerts

#### 5. **Real-time Alert Notifications** ⭐
- Connect to your backend's real-time alert system
- Display incoming alerts as they occur with:
  - Vehicle information
  - Geofence name and category
  - Event type (entry/exit)
  - Timestamp and location
- Recommended: Use toast notifications, alert banners, or a dedicated alerts feed

#### 6. **Violation History**
- Display historical geofence events
- Support filtering by vehicle, geofence, and date range
- Handle pagination for large datasets

### Technical Guidelines

- **Clean, intuitive interface** with clear visual feedback
- **Loading states** for all API operations
- **User friendly** with user-friendly UI and messages
- **Responsive design** that works on different screen sizes

### Recommended Map Integration

For the best user experience, integrate a map library to:
- Visualize geofences as polygons on a map
- Show vehicle locations as markers
- Allow users to input geofences and show directly on the map
- Allow users to click on the map or throgh inputs to set vehicle locations

---

## 📦 Submission Requirements

### 1. Code Repository (GitHub)
- Push your complete source code to GitHub (backend and frontend in separate folders or repos)
- Add the following as collaborators (**do not make repo public**):
  - `vedantp@mapup.ai`
  - `ajayap@mapup.ai`
  - `jyotis@mapup.ai`
  - `tanishqs@mapup.ai`
  - `naveenn@mapup.ai`
  - `asijitp@mapup.ai`
  - `dewanshp@mapup.ai`
  - `karkuvelpandip@mapup.ai`
  - `parask@mapup.ai`

### 2. Docker Hub
- Build and push your backend Docker image

### 3. Deployment

**Backend**: Deploy your containerized backend to any platform that supports Docker

**Frontend**: Deploy your React application to:
- Vercel
- Netlify
- Cloudflare Pages

### 4. Documentation

Include a `SETUP.md` file with:
- Prerequisites and dependencies
- Local setup instructions
- How to run with Docker Compose
- API testing guide with example curl commands
- Frontend usage guide
- Architecture overview (optional)

### 5. Video Submission Requirements

**Duration: 30-45 minutes**

Your video submission is a critical part of this assessment. Structure your video roughly as follows:

| Section                          | Duration  | What to Cover                                                                                                                                                                                                                                                       |
| -------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **App Demo**                     | 10-12 min | Full working demo — create a geofence on the map, register a vehicle, update its location to trigger an entry/exit event, show the real-time alert arriving in the frontend, view violation history with filters, and demonstrate the Docker setup running locally |
| **Implementation Deep Dive**     | 12-18 min | Walk through the codebase — project structure, architecture choices, Go backend design, geospatial point-in-polygon logic, WebSocket/SSE alert delivery, database schema, React frontend structure and real-time integration. Show actual code, not slides.         |
| **Gaps, Improvements & Roadmap** | 10-15 min | What's incomplete and your plan to finish it with time estimates. If complete: edge cases not handled, what you'd refactor with more time, how you'd make it production-ready, and how the system could scale to thousands of vehicles and geofences.                |

**Recording:** Use any screen recording tool (Loom, OBS, etc.). Upload to YouTube/Vimeo (unlisted), Loom, or Google Drive (shared with the collaborator emails listed above) and include the link in your README.

#### Finally, please fill out the google form that you received via email to submit the assessment for review

---

## 🎯 Evaluation Criteria

### Functionality
- ✅ All API endpoints working correctly
- ✅ Accurate geofence detection (point-in-polygon)
- ✅ Real-time alerts delivered properly
- ✅ Frontend integrated with backend APIs
- ✅ Proper handling of edge cases

### Code Quality 
- ✅ Clean, readable, well-organized code
- ✅ Go best practices and idioms
- ✅ React best practices (hooks, component structure)
- ✅ Proper error handling
- ✅ Meaningful variable and function names

### User Experience 
- ✅ Intuitive interface
- ✅ Clear visual feedback
- ✅ Proper error messages
- ✅ Real-time notifications work smoothly

### Performance
- ✅ Efficient geospatial queries
- ✅ Fast location update processing
- ✅ Proper indexing

### Dockerization & Deployment
- ✅ Clean Dockerfile
- ✅ Working docker-compose setup
- ✅ Successfully deployed and accessible

### Video submission
- ✅ Clear communication
- ✅ Technical depth and understanding about the code
- ✅ Self-awareness about limitations

---

## ⚠️ Code Originality

**Your submission must be your original work.** If your code is found to be similar to another candidate's submission, both parties will be disqualified. Do not share your code with others during the assessment period.

---

## 🌟 Bonus Points (Optional)

These are not required but will be considered favorably:

- Implement rate limiting on location update endpoints
- Add API authentication (JWT or API keys)
- Implement pagination for all list endpoints
- Add unit tests for critical functions

---

## 📚 Resources

- [PostGIS Documentation](https://postgis.net/documentation/)

---

## ❓ Questions?

If you have any questions about the assessment, please reach out to the hiring team at MapUp.

**Good luck! We're excited to see what you build! 🚀**

## Local implementation

This repository contains the working implementation described above. See
[SETUP.md](./SETUP.md) for local and Docker Compose instructions.
