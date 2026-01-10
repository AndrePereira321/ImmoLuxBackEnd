package utils

import (
	"testing"
)

func TestLocationValidator(t *testing.T) {
	validator, err := GetLocationValidator()
	if err != nil {
		t.Fatalf("Failed to initialize validator: %v", err)
	}

	// Test valid cities/municipalities
	validCities := []string{"Lisboa", "Porto", "Braga", "Faro", "Coimbra", "Aveiro"}
	for _, city := range validCities {
		if !validator.ValidateCity(city) {
			t.Errorf("Expected city '%s' to be valid", city)
		}
	}

	// Test case insensitivity
	if !validator.ValidateCity("LISBOA") {
		t.Error("Expected case insensitive validation to work")
	}
	if !validator.ValidateCity("porto") {
		t.Error("Expected case insensitive validation to work")
	}

	// Test invalid cities
	invalidCities := []string{"InvalidCity", "New York", "London", ""}
	for _, city := range invalidCities {
		if validator.ValidateCity(city) {
			t.Errorf("Expected city '%s' to be invalid", city)
		}
	}

	// Test valid postal codes
	validPostalCodes := []string{"1000-001", "4000-123", "9000-456"}
	for _, code := range validPostalCodes {
		if !validator.ValidatePostalCode(code) {
			t.Errorf("Expected postal code '%s' to be valid", code)
		}
	}

	// Test invalid postal codes
	invalidPostalCodes := []string{"1000", "1000-", "1000-1", "1000-12", "1000-1234", "10000-123", "100-123", "abcd-efg", ""}
	for _, code := range invalidPostalCodes {
		if validator.ValidatePostalCode(code) {
			t.Errorf("Expected postal code '%s' to be invalid", code)
		}
	}

	// Test valid districts
	validDistricts := []string{"Lisboa", "Porto", "Aveiro", "Faro"}
	for _, district := range validDistricts {
		if !validator.ValidateDistrict(district) {
			t.Errorf("Expected district '%s' to be valid", district)
		}
	}

	// Test statistics
	stats := validator.GetStats()
	if stats["districts"] == 0 {
		t.Error("Expected districts to be loaded")
	}
	if stats["municipalities"] == 0 {
		t.Error("Expected municipalities to be loaded")
	}
	if stats["parishes"] == 0 {
		t.Error("Expected parishes to be loaded")
	}

	t.Logf("Loaded: %d districts, %d municipalities, %d parishes",
		stats["districts"], stats["municipalities"], stats["parishes"])
}
