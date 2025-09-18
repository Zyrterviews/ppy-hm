//nolint:package-comments,revive,forbidigo,mnd,prealloc,exhaustruct,err113
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Vehicle struct {
	UUID                   string  `json:"uuid"`
	Plate                  string  `json:"plate"`
	LocationLatitude       float64 `json:"locationLatitude"`
	LocationLongitude      float64 `json:"locationLongitude"`
	Model                  Model   `json:"model"`
	Autonomy               float64 `json:"autonomy"`
	AutonomyPercentage     float64 `json:"autonomyPercentage"`
	DiscountAmount         int     `json:"discountAmount"`
	PictureURL             string  `json:"pictureUrl"`
	IsElligibleForFueling  bool    `json:"isElligibleForFueling"`
	IsElligibleForCharging bool    `json:"isElligibleForCharging"`
	FuelingReward          int     `json:"fuelingReward"`
	ChargingReward         int     `json:"chargingReward"`
}

type Model struct {
	Type   string `json:"type"`
	Make   string `json:"make"`
	Name   string `json:"name"`
	Energy string `json:"energy"`
	Tier   string `json:"tier"`
}

type PricingModel struct {
	UUID               string `json:"uuid"`
	Tier               string `json:"tier"`
	ModelType          string `json:"modelType"`
	UnlockFee          int    `json:"unlockFee"`
	MinutePrice        int    `json:"minutePrice"`
	PauseUnitPrice     int    `json:"pauseUnitPrice"`
	KilometerPrice     int    `json:"kilometerPrice"`
	BookUnitPrice      int    `json:"bookUnitPrice"`
	HourCapPrice       int    `json:"hourCapPrice"`
	DayCapPrice        int    `json:"dayCapPrice"`
	IncludedKilometers int    `json:"includedKilometers"`
	Type               string `json:"type"`
	MoveUnitPrice      int    `json:"moveUnitPrice"`
	OverKilometerPrice int    `json:"overKilometerPrice"`
}

type PricingResponse struct {
	PricingPerMinute    PricingModel `json:"pricingPerMinute"`
	PricingPerKilometer PricingModel `json:"pricingPerKilometer"`
	SmartPricing        PricingModel `json:"smartPricing"`
}

type GeoZoneItem struct {
	GeofencingType string     `json:"geofencingType"`
	ModelType      string     `json:"modelType"`
	Geom           GeoFeature `json:"geom"`
}

type GeoFeature struct {
	Type     string           `json:"type"`
	Geometry geojson.Geometry `json:"geometry"`
}

type GeoZone []GeoZoneItem

type TripLeg struct {
	StartLocation Location  `json:"startLocation"`
	EndLocation   Location  `json:"endLocation"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	PauseMinutes  int       `json:"pauseMinutes"`
}

type Journey struct {
	Legs []TripLeg `json:"legs"`
}

type JourneyPlan struct {
	Vehicle       Vehicle       `json:"vehicle"`
	Journey       Journey       `json:"journey"`
	TotalCost     float64       `json:"totalCost"`
	CostBreakdown CostBreakdown `json:"costBreakdown"`
	PricingModel  string        `json:"pricingModel"`
}

type CostBreakdown struct {
	UnlockFee   float64 `json:"unlockFee"`
	BookingCost float64 `json:"bookingCost"`
	TravelCost  float64 `json:"travelCost"`
	PauseCost   float64 `json:"pauseCost"`
	WalkingTime float64 `json:"walkingTimeMinutes"`
}

const (
	walkingSpeedKmh    = 5.0
	drivingSpeedKmh    = 25.0
	freeBookingMinutes = 15
	priceUnitFactor    = 1000.0
	brusselsUUID       = "a88ea9d0-3d5e-4002-8bbf-775313a5973c"
	apiURL             = "https://poppy.red/api/v3"
)

func fetchVehicles(
	ctx context.Context,
	client *http.Client,
) ([]Vehicle, error) {
	targetURL, err := url.JoinPath(apiURL, "cities", brusselsUUID, "vehicles")
	if err != nil {
		return nil, fmt.Errorf("[fetchVehicles] could not parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"[fetchVehicles] could not create request: %w",
			err,
		)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"[fetchVehicles] could not perform request: %w",
			err,
		)
	}

	defer func() { _ = res.Body.Close() }()

	var vehicles []Vehicle

	if err := json.NewDecoder(res.Body).Decode(&vehicles); err != nil {
		return nil, fmt.Errorf(
			"[fetchVehicles] error decoding vehicles: %w",
			err,
		)
	}

	var cars []Vehicle

	for _, vehicle := range vehicles {
		if vehicle.Model.Type != "car" {
			continue
		}

		cars = append(cars, vehicle)
	}

	return cars, nil
}

func fetchPricing(
	ctx context.Context,
	client *http.Client,
	modelType, tier string,
) (*PricingResponse, error) {
	targetURL, err := url.JoinPath(apiURL, "pricing", "pay-per-use")
	if err != nil {
		return nil, fmt.Errorf("[fetchPricing] could not parse URL: %w", err)
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("[fetchPricing] could not parse URL: %w", err)
	}

	query := parsedURL.Query()
	query.Set("modelType", modelType)
	query.Set("tier", tier)
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		parsedURL.String(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"[fetchPricing] could not create request: %w",
			err,
		)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"[fetchPricing] could not perform request: %w",
			err,
		)
	}

	defer func() { _ = res.Body.Close() }()

	var pricing PricingResponse

	if err := json.NewDecoder(res.Body).Decode(&pricing); err != nil {
		return nil, fmt.Errorf("[fetchPricing] error decoding pricing: %w", err)
	}

	return &pricing, nil
}

func fetchGeoZone(
	ctx context.Context,
	client *http.Client,
	vehicleUUID string,
) (*GeoZone, error) {
	targetURL, err := url.JoinPath(apiURL, "geozones", vehicleUUID)
	if err != nil {
		return nil, fmt.Errorf("[fetchGeoZone] could not parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"[fetchGeoZone] could not create request: %w",
			err,
		)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"[fetchGeoZone] could not perform request: %w",
			err,
		)
	}

	defer func() { _ = res.Body.Close() }()

	var geozone GeoZone

	if err := json.NewDecoder(res.Body).Decode(&geozone); err != nil {
		return nil, fmt.Errorf("[fetchGeoZone] error decoding geozone: %w", err)
	}

	return &geozone, nil
}

func calculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371

	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func isInParkingZone(location Location, geozone *GeoZone) bool {
	if geozone == nil {
		return false
	}

	point := orb.Point{location.Lng, location.Lat}

	for _, item := range *geozone {
		if item.GeofencingType == "parking" && item.ModelType == "car" {
			switch geom := item.Geom.Geometry.Geometry().(type) {
			case orb.Polygon:
				if planar.PolygonContains(geom, point) {
					return true
				}
			case orb.MultiPolygon:
				for _, polygon := range geom {
					if planar.PolygonContains(polygon, point) {
						return true
					}
				}
			}
		}
	}

	return false
}

func findClosestVehicle(location Location, vehicles []Vehicle) *Vehicle {
	if len(vehicles) == 0 {
		return nil
	}

	var closest *Vehicle

	minDistance := math.Inf(1)

	for i := range vehicles {
		distance := calculateDistance(
			location.Lat, location.Lng,
			vehicles[i].LocationLatitude, vehicles[i].LocationLongitude,
		)
		if distance < minDistance {
			minDistance = distance
			closest = &vehicles[i]
		}
	}

	return closest
}

func vehicleToLocation(vehicle Vehicle) Location {
	return Location{
		Lat: vehicle.LocationLatitude,
		Lng: vehicle.LocationLongitude,
	}
}

func calculateWalkingTime(fromLocation, toLocation Location) float64 {
	distance := calculateDistance(
		fromLocation.Lat,
		fromLocation.Lng,
		toLocation.Lat,
		toLocation.Lng,
	)

	return (distance / walkingSpeedKmh) * 60
}

func calculateDrivingTime(fromLocation, toLocation Location) float64 {
	distance := calculateDistance(
		fromLocation.Lat,
		fromLocation.Lng,
		toLocation.Lat,
		toLocation.Lng,
	)

	return (distance / drivingSpeedKmh) * 60
}

func calculateCost(
	journey Journey,
	vehicle Vehicle,
	pricing *PricingResponse,
	geozone *GeoZone,
) (*JourneyPlan, error) {
	plans := []JourneyPlan{}

	perMinutePlan := calculateCostForModel(
		journey,
		vehicle,
		pricing.PricingPerMinute,
		"per-minute",
		geozone,
	)
	if perMinutePlan != nil {
		plans = append(plans, *perMinutePlan)
	}

	perKilometerPlan := calculateCostForModel(
		journey,
		vehicle,
		pricing.PricingPerKilometer,
		"per-kilometer",
		geozone,
	)
	if perKilometerPlan != nil {
		plans = append(plans, *perKilometerPlan)
	}

	smartPlan := calculateCostForModel(
		journey,
		vehicle,
		pricing.SmartPricing,
		"smart",
		geozone,
	)
	if smartPlan != nil {
		plans = append(plans, *smartPlan)
	}

	if len(plans) == 0 {
		return nil, errors.New("[calculateCost] no valid pricing plans found")
	}

	cheapest := plans[0]
	for _, plan := range plans[1:] {
		if plan.TotalCost < cheapest.TotalCost {
			cheapest = plan
		}
	}

	return &cheapest, nil
}

func calculateCostForModel(
	journey Journey,
	vehicle Vehicle,
	pricing PricingModel,
	modelName string,
	geozone *GeoZone,
) *JourneyPlan {
	if len(journey.Legs) == 0 {
		return nil
	}

	breakdown := CostBreakdown{}

	unlockFee := float64(pricing.UnlockFee) / priceUnitFactor
	breakdown.UnlockFee = unlockFee

	var (
		totalBookingMinutes float64
		totalTravelMinutes  float64
		totalPauseMinutes   float64
		totalDistanceKm     float64
		walkingTime         float64
	)

	startLocation := journey.Legs[0].StartLocation
	vehicleLocation := vehicleToLocation(vehicle)
	walkingTime = calculateWalkingTime(startLocation, vehicleLocation)
	breakdown.WalkingTime = walkingTime

	currentLocation := vehicleLocation

	for _, leg := range journey.Legs {
		walkToVehicleTime := calculateWalkingTime(
			currentLocation,
			leg.StartLocation,
		)
		totalBookingMinutes += walkToVehicleTime

		drivingTime := calculateDrivingTime(leg.StartLocation, leg.EndLocation)
		totalTravelMinutes += drivingTime

		distance := calculateDistance(
			leg.StartLocation.Lat,
			leg.StartLocation.Lng,
			leg.EndLocation.Lat,
			leg.EndLocation.Lng,
		)
		totalDistanceKm += distance

		totalPauseMinutes += float64(leg.PauseMinutes)

		currentLocation = leg.EndLocation
	}

	bookingMinutesToCharge := math.Max(
		0,
		totalBookingMinutes-freeBookingMinutes,
	)
	breakdown.BookingCost = bookingMinutesToCharge * float64(
		pricing.BookUnitPrice,
	) / priceUnitFactor

	switch pricing.Type {
	case "minute":
		breakdown.TravelCost = totalTravelMinutes * float64(
			pricing.MinutePrice,
		) / priceUnitFactor
	case "kilometer":
		includedKm := float64(pricing.IncludedKilometers)
		chargeableKm := math.Max(0, totalDistanceKm-includedKm)
		breakdown.TravelCost = chargeableKm * float64(
			pricing.KilometerPrice,
		) / priceUnitFactor
	case "smart":
		minuteCost := totalTravelMinutes * float64(
			pricing.MinutePrice,
		) / priceUnitFactor
		includedKm := float64(pricing.IncludedKilometers)
		chargeableKm := math.Max(0, totalDistanceKm-includedKm)
		kmCost := chargeableKm * float64(
			pricing.KilometerPrice,
		) / priceUnitFactor
		breakdown.TravelCost = minuteCost + kmCost
	}

	breakdown.PauseCost = totalPauseMinutes * float64(
		pricing.PauseUnitPrice,
	) / priceUnitFactor

	totalCost := breakdown.UnlockFee + breakdown.BookingCost + breakdown.TravelCost + breakdown.PauseCost

	dayCapCost := float64(pricing.DayCapPrice) / priceUnitFactor
	if totalCost > dayCapCost {
		totalCost = dayCapCost
	}

	return &JourneyPlan{
		Vehicle:       vehicle,
		Journey:       journey,
		TotalCost:     totalCost,
		CostBreakdown: breakdown,
		PricingModel:  modelName,
	}
}

func planJourney(
	ctx context.Context,
	client *http.Client,
	journey Journey,
) (*JourneyPlan, error) {
	vehicles, err := fetchVehicles(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vehicles: %w", err)
	}

	if len(vehicles) == 0 {
		return nil, errors.New("[planJourney] no vehicles available")
	}

	if len(journey.Legs) == 0 {
		return nil, errors.New("[planJourney] journey has no legs")
	}

	startLocation := journey.Legs[0].StartLocation

	closestVehicle := findClosestVehicle(startLocation, vehicles)
	if closestVehicle == nil {
		return nil, errors.New("[planJourney] no vehicle found")
	}

	pricing, err := fetchPricing(
		ctx,
		client,
		closestVehicle.Model.Type,
		closestVehicle.Model.Tier,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pricing: %w", err)
	}

	geozone, err := fetchGeoZone(ctx, client, closestVehicle.UUID)
	if err != nil {
		fmt.Printf(
			"Warning: failed to fetch geozone for vehicle %s: %v\n",
			closestVehicle.UUID,
			err,
		)

		geozone = nil
	}

	plan, err := calculateCost(journey, *closestVehicle, pricing, geozone)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate cost: %w", err)
	}

	return plan, nil
}

func printJourneyPlan(plan *JourneyPlan) {
	fmt.Printf("\n=== OPTIMAL JOURNEY PLAN ===\n")
	fmt.Printf(
		"Vehicle: %s %s (%s)\n",
		plan.Vehicle.Model.Make,
		plan.Vehicle.Model.Name,
		plan.Vehicle.Plate,
	)
	fmt.Printf(
		"Location: %.6f, %.6f\n",
		plan.Vehicle.LocationLatitude,
		plan.Vehicle.LocationLongitude,
	)
	fmt.Printf("Pricing Model: %s\n", plan.PricingModel)
	fmt.Printf("Total Cost: €%.2f\n", plan.TotalCost)

	fmt.Printf("\n--- Cost Breakdown ---\n")
	fmt.Printf("Unlock Fee: €%.2f\n", plan.CostBreakdown.UnlockFee)
	fmt.Printf("Booking Cost: €%.2f\n", plan.CostBreakdown.BookingCost)
	fmt.Printf("Travel Cost: €%.2f\n", plan.CostBreakdown.TravelCost)
	fmt.Printf("Pause Cost: €%.2f\n", plan.CostBreakdown.PauseCost)
	fmt.Printf("Walking Time: %.1f minutes\n", plan.CostBreakdown.WalkingTime)

	fmt.Printf("\n--- Journey Details ---\n")

	for i, leg := range plan.Journey.Legs {
		fmt.Printf("Leg %d: (%.6f, %.6f) → (%.6f, %.6f)\n",
			i+1, leg.StartLocation.Lat, leg.StartLocation.Lng,
			leg.EndLocation.Lat, leg.EndLocation.Lng)

		if leg.PauseMinutes > 0 {
			fmt.Printf("  Pause: %d minutes\n", leg.PauseMinutes)
		}
	}
}

func getTestScenarios() []Journey {
	return []Journey{
		{
			Legs: []TripLeg{
				{
					StartLocation: Location{Lat: 50.8355, Lng: 4.3573},
					EndLocation:   Location{Lat: 50.8245, Lng: 4.3635},
					PauseMinutes:  120,
				},
				{
					StartLocation: Location{Lat: 50.8245, Lng: 4.3635},
					EndLocation:   Location{Lat: 50.8275, Lng: 4.3745},
					PauseMinutes:  0,
				},
			},
		},
		{
			Legs: []TripLeg{
				{
					StartLocation: Location{Lat: 50.8466, Lng: 4.3528},
					EndLocation:   Location{Lat: 50.7847, Lng: 4.2461},
					PauseMinutes:  60,
				},
				{
					StartLocation: Location{Lat: 50.7847, Lng: 4.2461},
					EndLocation:   Location{Lat: 50.9011, Lng: 4.4844},
					PauseMinutes:  0,
				},
			},
		},
		{
			Legs: []TripLeg{
				{
					StartLocation: Location{Lat: 50.8466, Lng: 4.3928},
					EndLocation:   Location{Lat: 50.8098, Lng: 4.3542},
					PauseMinutes:  0,
				},
			},
		},
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	dialer := net.Dialer{
		Timeout:   timeout,
		KeepAlive: timeout,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		Dial:                  dialer.Dial,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       timeout * 3,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return client
}

func main() {
	fmt.Println("Poppy Journey Planner")
	fmt.Println("====================")

	scenarios := getTestScenarios()
	scenarioNames := []string{
		"Jane: Brussels South Station → Stephanie/Louise (2h pause) → Flagey",
		"John: Brussels Center → Dilbeek (1h pause) → Airport",
		"Vicky: Wezembeek → Avenue de l'Observatoire, Uccle",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	client := newHTTPClient(10 * time.Second)

	for i, scenario := range scenarios {
		fmt.Printf("\n\n🚗 SCENARIO %d: %s\n", i+1, scenarioNames[i])
		fmt.Println("=" + fmt.Sprintf("%*s", len(scenarioNames[i])+20, "="))

		plan, err := planJourney(ctx, client, scenario)
		if err != nil {
			fmt.Printf("❌ Error planning journey: %v\n", err)

			continue
		}

		printJourneyPlan(plan)
	}

	fmt.Println("\n\n✅ All scenarios completed!")
}
