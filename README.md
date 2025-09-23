# Poppy Journey Planner

A journey planning and cost estimation tool for Poppy car sharing service. The application calculates optimal multi-leg journeys with real-time vehicle data and accurate pricing estimates.

## Features

- **Multi-leg journey planning** with pause optimization
- **Real-time vehicle location data** from Poppy API
- **Multiple pricing models** (per-minute, per-kilometer, smart pricing)
- **Parking zone validation** with cost penalties for non-compliant parking
- **OpenRouteService integration** with fallback to crow-flies calculations
- **Web interface** for journey input and result visualization
- **REST API** for programmatic access

## Architecture

The application is built in Go with a classic separation between business logic and presentation layers:

- **Backend API client** for Poppy JSON data endpoints (vehicles, pricing, geozones)
- **Journey planning engine** with cost optimization algorithms
- **Parking zone validation** using geometric point-in-polygon calculations  
- **Route calculation** via OpenRouteService with graceful fallback
- **Web frontend** using templ and htmx for reactive UX
- **Basic test suite** covering integration scenarios

## Quick Start

### Prerequisites

- Go 1.21 or later
- Optional: OpenRouteService API key for realistic routing

### Installation

```bash
git clone <repository-url>
cd poppy
go mod download
```

### Running the Application

```bash
go run main.go templates_templ.go
```

The application will start on http://localhost:8080

### Environment Configuration

Copy the example environment file:

```bash
cp .env.example .env
```

For enhanced routing accuracy, add your OpenRouteService API key to `.env`:

```
ORS_API_KEY=your_api_key_here
```

## OpenRouteService Setup (Optional)

The application works without an API key using fallback calculations. For production-quality routing:

1. Sign up at https://openrouteservice.org/dev/#/signup
2. Generate an API key in your dashboard
3. Add the key to your `.env` file
4. Restart the application

**Free tier limitations**: 2000 requests per day

## API Reference

### Plan Journey

**POST** `/api/v1/plan-journey`

Request body:
```json
{
  "journey": {
    "legs": [
      {
        "startLocation": {"lat": 50.8355, "lng": 4.3573},
        "endLocation": {"lat": 50.8245, "lng": 4.3635},
        "pauseMinutes": 120
      }
    ]
  }
}
```

Response:
```json
{
  "success": true,
  "data": {
    "vehicle": {
      "plate": "2HFP336",
      "model": {"make": "Opel", "name": "CORSA"}
    },
    "totalCost": 32.30,
    "pricingModel": "smart",
    "costBreakdown": {
      "unlockFee": 0.826,
      "travelCost": 1.72,
      "pauseCost": 29.76,
      "walkingTimeMinutes": 1.14
    },
    "usedFallbackRouting": false
  }
}
```

### Other Endpoints

- **GET** `/api/v1/vehicles` - List available vehicles
- **GET** `/api/v1/health` - Service health check
- **GET** `/` - Web interface

## Testing

Run the test suite:

```bash
go test -v
```

The tests cover:
- Integration scenarios from the specification
- Distance calculation accuracy
- Core business logic functions
- Error handling and edge cases

## Implementation Notes

### Pricing Logic

The application implements three pricing models as specified:
- **Per-minute**: Charges based on driving time
- **Per-kilometer**: Charges based on distance with included kilometers
- **Smart pricing**: Hybrid model choosing the most economical option

### Parking Zone Enforcement

- Journey endpoints must be within designated parking zones
- Pausing outside parking zones incurs a 1.5x cost penalty
- Real-time geozone data validation using point-in-polygon algorithms

### Routing Fallback

When OpenRouteService is unavailable, the system falls back to:
- Crow-flies distance calculations
- Fixed speed assumptions (25 km/h driving, 5 km/h walking)
- Clear user notifications about approximate routing

## Development

### Code Structure

- `main.go` - Core application logic and HTTP handlers
- `templates.templ` - Web interface templates
- `main_test.go` - Test suite
- `.env.example` - Environment configuration template

### Dependencies

- `github.com/paulmach/orb` - Geometric calculations
- `github.com/a-h/templ` - Type-safe HTML templates
- `github.com/joho/godotenv` - Environment variable loading

## Acknowledgments

The frontend interface was generated using Claude Code (Anthropic's AI assistant) to rapidly prototype the user experience. The core business logic, API integration, and architectural decisions were developed collaboratively with AI assistance while maintaining hands-on oversight of implementation details and requirements compliance.

The README is 99.9% AI generated (hey I wrote this line by hand!)
