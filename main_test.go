//nolint:package-comments,revive,forbidigo,mnd,prealloc,exhaustruct,err113
package main

import (
	"context"
	"math"
	"testing"
	"time"
)

func getIntegrationTestScenarios() []struct {
	name     string
	journey  Journey
	expected expectedResult
} {
	return []struct {
		name     string
		journey  Journey
		expected expectedResult
	}{
		{
			name: "Jane: Brussels South Station → Stephanie/Louise (2h pause) → Flagey",
			journey: Journey{
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
			expected: expectedResult{
				shouldSucceed:    true,
				minCost:          25.0,
				maxCost:          40.0,
				expectedPauses:   120,
				hasWalkingTime:   true,
			},
		},
		{
			name: "John: Brussels Center → Dilbeek (1h pause) → Airport", 
			journey: Journey{
				Legs: []TripLeg{
					{
						StartLocation: Location{Lat: 50.8466, Lng: 4.3528},
						EndLocation:   Location{Lat: 50.7847, Lng: 4.2461},
						PauseMinutes:  60,
					},
					{
						StartLocation: Location{Lat: 50.7847, Lng: 4.2461},
						EndLocation:   Location{Lat: 50.9014, Lng: 4.4844},
						PauseMinutes:  0,
					},
				},
			},
			expected: expectedResult{
				shouldSucceed:    false,
				minCost:          0.0,
				maxCost:          0.0,
				expectedPauses:   60,
				hasWalkingTime:   true,
			},
		},
		{
			name: "Vicky: Wezembeek → Avenue de l'Observatoire, Uccle",
			journey: Journey{
				Legs: []TripLeg{
					{
						StartLocation: Location{Lat: 50.8466, Lng: 4.3928},
						EndLocation:   Location{Lat: 50.8098, Lng: 4.3542},
						PauseMinutes:  0,
					},
				},
			},
			expected: expectedResult{
				shouldSucceed:    true,
				minCost:          3.0,
				maxCost:          8.0,
				expectedPauses:   0,
				hasWalkingTime:   true,
			},
		},
	}
}

type expectedResult struct {
	shouldSucceed  bool
	minCost        float64
	maxCost        float64
	expectedPauses int
	hasWalkingTime bool
}

func TestPlanJourney_IntegrationScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newHTTPClient(10 * time.Second)
	scenarios := getIntegrationTestScenarios()

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			plan, err := planJourney(ctx, client, scenario.journey)

			if scenario.expected.shouldSucceed {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
					return
				}

				if plan == nil {
					t.Error("Expected plan but got nil")
					return
				}

				if plan.TotalCost < scenario.expected.minCost || plan.TotalCost > scenario.expected.maxCost {
					t.Errorf("Cost %.2f not in expected range [%.2f, %.2f]", 
						plan.TotalCost, scenario.expected.minCost, scenario.expected.maxCost)
				}

				if scenario.expected.hasWalkingTime && plan.CostBreakdown.WalkingTime <= 0 {
					t.Error("Expected walking time but got none")
				}

				if len(plan.Journey.Legs) != len(scenario.journey.Legs) {
					t.Errorf("Expected %d legs but got %d", 
						len(scenario.journey.Legs), len(plan.Journey.Legs))
				}

				totalPauses := 0
				for _, leg := range plan.Journey.Legs {
					totalPauses += leg.PauseMinutes
				}
				if totalPauses != scenario.expected.expectedPauses {
					t.Errorf("Expected %d total pause minutes but got %d", 
						scenario.expected.expectedPauses, totalPauses)
				}

			} else {
				if err == nil {
					t.Error("Expected error but got success")
				}
			}
		})
	}
}

func TestCalculateDistance(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lng1     float64
		lat2     float64
		lng2     float64
		expected float64
		delta    float64
	}{
		{
			name:     "Brussels South to Stephanie/Louise",
			lat1:     50.8355,
			lng1:     4.3573,
			lat2:     50.8245,
			lng2:     4.3635,
			expected: 1.25,
			delta:    0.1,
		},
		{
			name:     "Same location",
			lat1:     50.8466,
			lng1:     4.3528,
			lat2:     50.8466,
			lng2:     4.3528,
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "Brussels to Dilbeek",
			lat1:     50.8466,
			lng1:     4.3528,
			lat2:     50.7847,
			lng2:     4.2461,
			expected: 10.2,
			delta:    0.5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calculateDistance(test.lat1, test.lng1, test.lat2, test.lng2)
			if math.Abs(result-test.expected) > test.delta {
				t.Errorf("Expected %.2f ± %.2f but got %.2f", 
					test.expected, test.delta, result)
			}
		})
	}
}

func TestCalculateWalkingTime(t *testing.T) {
	tests := []struct {
		name         string
		from         Location
		to           Location
		expectedMin  float64
		expectedMax  float64
	}{
		{
			name:        "Short walk",
			from:        Location{Lat: 50.8355, Lng: 4.3573},
			to:          Location{Lat: 50.8365, Lng: 4.3583},
			expectedMin: 1.0,
			expectedMax: 3.0,
		},
		{
			name:        "Same location",
			from:        Location{Lat: 50.8466, Lng: 4.3528},
			to:          Location{Lat: 50.8466, Lng: 4.3528},
			expectedMin: 0.0,
			expectedMax: 0.1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calculateWalkingTime(test.from, test.to)
			if result < test.expectedMin || result > test.expectedMax {
				t.Errorf("Expected walking time in range [%.2f, %.2f] but got %.2f",
					test.expectedMin, test.expectedMax, result)
			}
		})
	}
}

func TestFindClosestVehicle(t *testing.T) {
	location := Location{Lat: 50.8466, Lng: 4.3528}
	
	vehicles := []Vehicle{
		{
			UUID: "vehicle1",
			LocationLatitude:  50.8500,
			LocationLongitude: 4.3600,
			Model: Model{Type: "car", Tier: "S"},
		},
		{
			UUID: "vehicle2", 
			LocationLatitude:  50.8450,
			LocationLongitude: 4.3500,
			Model: Model{Type: "car", Tier: "S"},
		},
		{
			UUID: "vehicle3",
			LocationLatitude:  50.9000,
			LocationLongitude: 4.4000,
			Model: Model{Type: "car", Tier: "S"},
		},
	}

	closest := findClosestVehicle(location, vehicles)
	if closest == nil {
		t.Fatal("Expected to find closest vehicle but got nil")
	}

	if closest.UUID != "vehicle2" {
		t.Errorf("Expected vehicle2 to be closest but got %s", closest.UUID)
	}

	emptyResult := findClosestVehicle(location, []Vehicle{})
	if emptyResult != nil {
		t.Error("Expected nil for empty vehicle list")
	}
}

func TestVehicleToLocation(t *testing.T) {
	vehicle := Vehicle{
		LocationLatitude:  50.8466,
		LocationLongitude: 4.3528,
	}

	location := vehicleToLocation(vehicle)
	
	if location.Lat != vehicle.LocationLatitude {
		t.Errorf("Expected lat %.6f but got %.6f", 
			vehicle.LocationLatitude, location.Lat)
	}
	
	if location.Lng != vehicle.LocationLongitude {
		t.Errorf("Expected lng %.6f but got %.6f", 
			vehicle.LocationLongitude, location.Lng)  
	}
}