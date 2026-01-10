package utils

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed data/portugal-admin-divisions.json
var portugalAdminDivisionsJSON []byte

type AdminDivision struct {
	Level int    `json:"level"` // 1=Distrito, 2=Concelho, 3=Freguesia
	Code  any    `json:"code"`  // Can be int or string (some codes are alphanumeric)
	Name  string `json:"name"`
}

type LocationValidator struct {
	districts          map[string]bool // Level 1: Distritos
	municipalities     map[string]bool // Level 2: Concelhos
	parishes           map[string]bool // Level 3: Freguesias
	districtsList      []string        // Sorted list of districts
	municipalitiesList []string        // Sorted list of municipalities
	parishesList       []string        // Sorted list of parishes
	postalCodeRe       *regexp.Regexp
	once               sync.Once
}

var (
	locationValidator *LocationValidator
	validatorOnce     sync.Once
)

// GetLocationValidator returns a singleton instance of the location validator
func GetLocationValidator() (*LocationValidator, error) {
	var initErr error
	validatorOnce.Do(func() {
		locationValidator = &LocationValidator{
			districts:      make(map[string]bool),
			municipalities: make(map[string]bool),
			parishes:       make(map[string]bool),
			postalCodeRe:   regexp.MustCompile(`^\d{4}-\d{3}$`),
		}
		initErr = locationValidator.loadData()
	})

	if initErr != nil {
		return nil, initErr
	}
	return locationValidator, nil
}

func (lv *LocationValidator) loadData() error {
	var divisions []AdminDivision
	if err := json.Unmarshal(portugalAdminDivisionsJSON, &divisions); err != nil {
		return fmt.Errorf("failed to parse Portugal admin divisions: %w", err)
	}

	districtSet := make(map[string]bool)
	municipalitySet := make(map[string]bool)
	parishSet := make(map[string]bool)

	for _, div := range divisions {
		normalizedName := normalizeName(div.Name)
		switch div.Level {
		case 1:
			lv.districts[normalizedName] = true
			if !districtSet[div.Name] {
				districtSet[div.Name] = true
				lv.districtsList = append(lv.districtsList, div.Name)
			}
		case 2:
			lv.municipalities[normalizedName] = true
			if !municipalitySet[div.Name] {
				municipalitySet[div.Name] = true
				lv.municipalitiesList = append(lv.municipalitiesList, div.Name)
			}
		case 3:
			lv.parishes[normalizedName] = true
			if !parishSet[div.Name] {
				parishSet[div.Name] = true
				lv.parishesList = append(lv.parishesList, div.Name)
			}
		}
	}

	return nil
}

// normalizeName normalizes a location name for comparison (lowercase, trimmed)
func normalizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

// ValidateCity validates if a city/municipality exists in Portugal
func (lv *LocationValidator) ValidateCity(city string) bool {
	if city == "" {
		return false
	}
	normalized := normalizeName(city)
	// Check both municipalities (level 2) and districts (level 1)
	return lv.municipalities[normalized] || lv.districts[normalized]
}

// ValidateMunicipality validates if a municipality (concelho) exists
func (lv *LocationValidator) ValidateMunicipality(municipality string) bool {
	if municipality == "" {
		return false
	}
	normalized := normalizeName(municipality)
	return lv.municipalities[normalized]
}

// ValidateDistrict validates if a district (distrito) exists
func (lv *LocationValidator) ValidateDistrict(district string) bool {
	if district == "" {
		return false
	}
	normalized := normalizeName(district)
	return lv.districts[normalized]
}

// ValidateParish validates if a parish (freguesia) exists
func (lv *LocationValidator) ValidateParish(parish string) bool {
	if parish == "" {
		return false
	}
	normalized := normalizeName(parish)
	return lv.parishes[normalized]
}

// ValidatePostalCode validates Portuguese postal code format (XXXX-XXX)
func (lv *LocationValidator) ValidatePostalCode(postalCode string) bool {
	if postalCode == "" {
		return false
	}
	return lv.postalCodeRe.MatchString(postalCode)
}

// GetStats returns statistics about loaded data
func (lv *LocationValidator) GetStats() map[string]int {
	return map[string]int{
		"districts":      len(lv.districts),
		"municipalities": len(lv.municipalities),
		"parishes":       len(lv.parishes),
	}
}

// GetDistricts returns a list of all Portuguese districts
func (lv *LocationValidator) GetDistricts() []string {
	return lv.districtsList
}

// GetMunicipalities returns a list of all Portuguese municipalities
func (lv *LocationValidator) GetMunicipalities() []string {
	return lv.municipalitiesList
}

// GetParishes returns a list of all Portuguese parishes
func (lv *LocationValidator) GetParishes() []string {
	return lv.parishesList
}

// GetCities returns a combined list of districts and municipalities for city selection
func (lv *LocationValidator) GetCities() []string {
	citySet := make(map[string]bool)
	cities := []string{}

	// Add all districts
	for _, district := range lv.districtsList {
		if !citySet[district] {
			citySet[district] = true
			cities = append(cities, district)
		}
	}

	// Add all municipalities
	for _, municipality := range lv.municipalitiesList {
		if !citySet[municipality] {
			citySet[municipality] = true
			cities = append(cities, municipality)
		}
	}

	return cities
}
