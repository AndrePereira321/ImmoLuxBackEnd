package config

import "testing"

func TestAppVersionConstructor(t *testing.T) {
	t.Run("test simple app version constructor", func(t *testing.T) {
		version, err := NewAppVersion("v2.5.243")
		if err != nil {
			t.Error("failing app version constructor")
		}
		if version.FullVersion() != "v2.5.243" {
			t.Error("failing app version constructor")
		}
		if version.Major() != 2 {
			t.Error("failing parsing major version")
		}
		if version.Minor() != 5 {
			t.Error("failing parsing minor version")
		}
		if version.Patch() != 243 {
			t.Error("failing parsing patch version")
		}
		if version.PreRelease() != "" {
			t.Error("failing parsing pre release version")
		}
		if version.Build() != "" {
			t.Error("failing parsing build version")
		}
	})
	t.Run("test app version constructor with pre release", func(t *testing.T) {
		version, err := NewAppVersion("v2.5.243-beta5")
		if err != nil {
			t.Error("failing app version constructor")
		}
		if version.FullVersion() != "v2.5.243-beta5" {
			t.Error("failing app version constructor")
		}
		if version.Major() != 2 {
			t.Error("failing parsing major version")
		}
		if version.Minor() != 5 {
			t.Error("failing parsing minor version")
		}
		if version.Patch() != 243 {
			t.Error("failing parsing patch version")
		}
		if version.PreRelease() != "beta5" {
			t.Error("failing parsing pre release version")
		}
		if version.Build() != "" {
			t.Error("failing parsing build version")
		}
	})
	t.Run("test app version constructor with pre release and plus", func(t *testing.T) {
		version, err := NewAppVersion("v43.82.443-beta5+build123")
		if err != nil {
			t.Error("failing app version constructor")
		}
		if version.FullVersion() != "v43.82.443-beta5+build123" {
			t.Error("failing app version constructor")
		}
		if version.Major() != 43 {
			t.Error("failing parsing major version")
		}
		if version.Minor() != 82 {
			t.Error("failing parsing minor version")
		}
		if version.Patch() != 443 {
			t.Error("failing parsing patch version")
		}
		if version.PreRelease() != "beta5" {
			t.Error("failing parsing pre release version")
		}
		if version.Build() != "build123" {
			t.Error("failing parsing build version")
		}
	})
	t.Run("test invalid versions", func(t *testing.T) {
		_, err := NewAppVersion("v-2.5.243-beta5+build123")
		if err == nil {
			t.Error("failing parsing invalid version")
		}
		_, err1 := NewAppVersion("v2.-5.243-beta5+build123")
		if err1 == nil {
			t.Error("failing parsing invalid version")
		}
		_, err2 := NewAppVersion("va.b.c")
		if err2 == nil {
			t.Error("failing parsing invalid version")
		}
	})
}

func TestIsBigger(t *testing.T) {
	t.Run("test is bigger", func(t *testing.T) {
		v1, err := NewAppVersion("v2.5.243")
		if err != nil {
			t.Error("failing app version constructor")
		}
		v2, err := NewAppVersion("v2.5.243")
		if err != nil {
			t.Error("failing app version constructor")
		}
		v3, err := NewAppVersion("v2.5.244")
		if err != nil {
			t.Error("failing app version constructor")
		}
		v4, err := NewAppVersion("v3.5.243")
		if err != nil {
			t.Error("failing app version constructor")
		}
		v5, err := NewAppVersion("v1.5.245")
		if err != nil {
			t.Error("failing app version constructor")
		}

		if v1.IsBigger(v2) {
			t.Error("invalid version comparison")
		}
		if !v3.IsBigger(v1) {
			t.Error("invalid version comparison")
		}
		if !v4.IsBigger(v1) {
			t.Error("invalid version comparison")
		}
		if v5.IsBigger(v1) {
			t.Error("invalid version comparison")
		}
	})
}
