package config

import (
	"immo-lux/internal/server_error"
	"strconv"
	"strings"
)

type AppVersion struct {
	fullVersion string
	major       uint
	minor       uint
	patch       uint
	preRelease  string
	build       string
}

func NewAppVersion(textVersion string) (*AppVersion, error) {
	fullVersion := strings.Clone(textVersion)
	if fullVersion == "" {
		return nil, server_error.New("APP_VERSION", "app version is empty")
	}

	version := strings.TrimPrefix(fullVersion, "v")

	var preRelease, build string
	if dashIndex := strings.Index(version, "-"); dashIndex != -1 {
		preRelease = version[dashIndex+1:]
		version = version[:dashIndex]

		if plusIndex := strings.Index(preRelease, "+"); plusIndex != -1 {
			build = preRelease[plusIndex+1:]
			preRelease = preRelease[:plusIndex]
		}
	}

	if plusIndex := strings.Index(version, "+"); plusIndex != -1 {
		version = version[:plusIndex]
	}

	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return nil, server_error.New("APP_VERSION", "invalid version format: expected MAJOR.MINOR.PATCH, got "+textVersion)
	}

	major, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, server_error.New("APP_VERSION", "invalid major version: "+parts[0])
	}

	minor, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, server_error.New("APP_VERSION", "invalid minor version: "+parts[1])
	}

	patch, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return nil, server_error.New("APP_VERSION", "invalid patch version: "+parts[2])
	}

	return &AppVersion{
		fullVersion: textVersion,
		major:       uint(major),
		minor:       uint(minor),
		patch:       uint(patch),
		preRelease:  preRelease,
		build:       build,
	}, nil

}

func (v *AppVersion) FullVersion() string {
	return v.fullVersion
}

func (v *AppVersion) Major() uint {
	return v.major
}

func (v *AppVersion) Minor() uint {
	return v.minor
}

func (v *AppVersion) Patch() uint {
	return v.patch
}

func (v *AppVersion) PreRelease() string {
	return v.preRelease
}

func (v *AppVersion) Build() string {
	return v.build
}

func (v *AppVersion) String() string {
	return v.fullVersion
}

func (v *AppVersion) IsBigger(other *AppVersion) bool {
	return v.major > other.major ||
		(v.major == other.major && v.minor > other.minor) ||
		(v.major == other.major && v.minor == other.minor && v.patch > other.patch)
}
