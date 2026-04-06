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
	districts          map[string]bool
	municipalities     map[string]bool
	parishes           map[string]bool
	districtsList      []string
	municipalitiesList []string
	parishesList       []string
	postalCodeRe       *regexp.Regexp
}

var (
	locationValidator *LocationValidator
	validatorOnce     sync.Once
)

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

func normalizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func (lv *LocationValidator) ValidateCity(city string) bool {
	if city == "" {
		return false
	}
	normalized := normalizeName(city)
	return lv.municipalities[normalized] || lv.districts[normalized]
}

func (lv *LocationValidator) ValidateMunicipality(municipality string) bool {
	if municipality == "" {
		return false
	}
	normalized := normalizeName(municipality)
	return lv.municipalities[normalized]
}

func (lv *LocationValidator) ValidateDistrict(district string) bool {
	if district == "" {
		return false
	}
	normalized := normalizeName(district)
	return lv.districts[normalized]
}

func (lv *LocationValidator) ValidateParish(parish string) bool {
	if parish == "" {
		return false
	}
	normalized := normalizeName(parish)
	return lv.parishes[normalized]
}

func (lv *LocationValidator) ValidatePostalCode(postalCode string) bool {
	if postalCode == "" {
		return false
	}
	return lv.postalCodeRe.MatchString(postalCode)
}

func (lv *LocationValidator) GetStats() map[string]int {
	return map[string]int{
		"districts":      len(lv.districts),
		"municipalities": len(lv.municipalities),
		"parishes":       len(lv.parishes),
	}
}

func (lv *LocationValidator) GetDistricts() []string {
	return lv.districtsList
}

func (lv *LocationValidator) GetMunicipalities() []string {
	return lv.municipalitiesList
}

func (lv *LocationValidator) GetParishes() []string {
	return lv.parishesList
}

func (lv *LocationValidator) GetCities() []string {
	citySet := make(map[string]bool)
	var cities []string

	for _, district := range lv.districtsList {
		if !citySet[district] {
			citySet[district] = true
			cities = append(cities, district)
		}
	}

	for _, municipality := range lv.municipalitiesList {
		if !citySet[municipality] {
			citySet[municipality] = true
			cities = append(cities, municipality)
		}
	}

	return cities
}
