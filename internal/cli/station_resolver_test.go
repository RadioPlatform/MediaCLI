package cli

import (
	"testing"

	"radioplatform-media-ci/pkg/api"
)

func TestResolveByExactUUID(t *testing.T) {
	stations := []api.Station{
		{UUID: "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100", Name: "Accra Radio"},
		{UUID: "3a82b7dc-9d42-5d6a-ada9-932e98e0b211", Name: "Kumasi FM"},
	}

	s, err := resolveStationByIDOrName(stations, "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Accra Radio" {
		t.Errorf("expected Accra Radio, got %s", s.Name)
	}
}

func TestResolveByUUIDPrefix(t *testing.T) {
	stations := []api.Station{
		{UUID: "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100", Name: "Accra Radio"},
		{UUID: "3a82b7dc-9d42-5d6a-ada9-932e98e0b211", Name: "Kumasi FM"},
	}

	s, err := resolveStationByIDOrName(stations, "2f71a6cb")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Accra Radio" {
		t.Errorf("expected Accra Radio, got %s", s.Name)
	}
}

func TestResolveByExactNameCaseInsensitive(t *testing.T) {
	stations := []api.Station{
		{UUID: "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100", Name: "Accra Radio"},
	}

	s, err := resolveStationByIDOrName(stations, "accra radio")
	if err != nil {
		t.Fatal(err)
	}
	if s.UUID != "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100" {
		t.Errorf("expected matching UUID, got %s", s.UUID)
	}
}

func TestResolveByPartialName(t *testing.T) {
	stations := []api.Station{
		{UUID: "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100", Name: "Accra Radio"},
		{UUID: "3a82b7dc-9d42-5d6a-ada9-932e98e0b211", Name: "Kumasi FM"},
	}

	s, err := resolveStationByIDOrName(stations, "accra")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Accra Radio" {
		t.Errorf("expected Accra Radio, got %s", s.Name)
	}
}

func TestResolveAmbiguousUUID(t *testing.T) {
	stations := []api.Station{
		{UUID: "aaaaaaaa-1111-1111-1111-111111111111", Name: "Station A"},
		{UUID: "aaaaaaab-2222-2222-2222-222222222222", Name: "Station B"},
	}

	_, err := resolveStationByIDOrName(stations, "aaaa")
	if err == nil {
		t.Fatal("expected error for ambiguous UUID prefix")
	}
}

func TestResolveAmbiguousPartialName(t *testing.T) {
	stations := []api.Station{
		{UUID: "uuid-1", Name: "Accra Radio"},
		{UUID: "uuid-2", Name: "Accra FM"},
	}

	_, err := resolveStationByIDOrName(stations, "accra")
	if err == nil {
		t.Fatal("expected error for ambiguous name")
	}
}

func TestResolveUnknownStation(t *testing.T) {
	stations := []api.Station{
		{UUID: "uuid-1", Name: "Accra Radio"},
	}

	_, err := resolveStationByIDOrName(stations, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown station")
	}
}

func TestResolveEmptyStations(t *testing.T) {
	_, err := resolveStationByIDOrName([]api.Station{}, "anything")
	if err == nil {
		t.Fatal("expected error for empty stations")
	}
}

func TestResolveUUIDPrefixUnique(t *testing.T) {
	stations := []api.Station{
		{UUID: "aaaa1111-1111-1111-1111-111111111111", Name: "Station A"},
		{UUID: "bbbb2222-2222-2222-2222-222222222222", Name: "Station B"},
	}

	s, err := resolveStationByIDOrName(stations, "aaaa1111")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Station A" {
		t.Errorf("expected Station A, got %s", s.Name)
	}
}

func TestResolveFullUUIDPersistence(t *testing.T) {
	stations := []api.Station{
		{UUID: "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100", Name: "Accra Radio"},
	}

	s, err := resolveStationByIDOrName(stations, "2f71a6cb")
	if err != nil {
		t.Fatal(err)
	}
	if s.UUID != "2f71a6cb-8c31-4c5b-9cb9-821d87d9a100" {
		t.Errorf("expected full UUID, got %s", s.UUID)
	}
}

func TestResolveCaseInsensitiveNameWithSpaces(t *testing.T) {
	stations := []api.Station{
		{UUID: "uuid-1", Name: "Accra Radio FM"},
		{UUID: "uuid-2", Name: "New Test Station"},
	}

	s, err := resolveStationByIDOrName(stations, "NEW TEST STATION")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "New Test Station" {
		t.Errorf("expected New Test Station, got %s", s.Name)
	}
}
