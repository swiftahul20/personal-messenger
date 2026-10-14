package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeatherToolUsesValidatedOpenMeteoRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if r.URL.Query().Get("name") != "New York" || r.URL.Query().Get("count") != "1" {
				t.Errorf("unexpected geocoding query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"results":[{"name":"New York","admin1":"New York","country":"United States","latitude":40.7,"longitude":-74.0}]}`))
		case "/forecast":
			if r.URL.Query().Get("temperature_unit") != "celsius" || r.URL.Query().Get("wind_speed_unit") != "kmh" {
				t.Errorf("unexpected forecast units: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"current":{"time":"2026-10-12T12:00","temperature_2m":18.5,"relative_humidity_2m":55,"apparent_temperature":18.1,"weather_code":2,"wind_speed_10m":10.2}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	weather := newWeatherClient(server.Client())
	weather.geocodingEndpoint = server.URL + "/search"
	weather.forecastEndpoint = server.URL + "/forecast"
	result, err := weather.Current(context.Background(), " New York ")
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if result["location"] != "New York, United States" {
		t.Errorf("location = %v", result["location"])
	}
	if result["conditions"] != "partly cloudy" {
		t.Errorf("conditions = %v", result["conditions"])
	}
	if result["temperature_c"] != 18.5 {
		t.Errorf("temperature = %v", result["temperature_c"])
	}
	if result["source"] != "Open-Meteo" {
		t.Errorf("source = %v", result["source"])
	}
}

func TestWeatherToolRejectsInvalidLocationBeforeRequest(t *testing.T) {
	weather := newWeatherClient(http.DefaultClient)
	for _, location := range []string{"", " \t ", "bad\nlocation", string(make([]rune, maxWeatherLocationLength+1))} {
		if _, err := weather.Current(context.Background(), location); err == nil {
			t.Errorf("Current(%q) unexpectedly succeeded", location)
		}
	}
}

func TestWeatherToolRejectsInvalidCoordinatesAndWeatherData(t *testing.T) {
	responses := []string{
		`{"results":[{"name":"Nowhere","latitude":100,"longitude":0}]}`,
	}
	for _, body := range responses {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		weather := newWeatherClient(server.Client())
		weather.geocodingEndpoint = server.URL
		weather.forecastEndpoint = server.URL
		if _, err := weather.Current(context.Background(), "Nowhere"); err == nil {
			t.Errorf("Current() with invalid response %s unexpectedly succeeded", body)
		}
		server.Close()
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			_, _ = w.Write([]byte(`{"results":[{"name":"Nowhere","latitude":1,"longitude":2}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"current":{"temperature_2m":1,"relative_humidity_2m":101,"apparent_temperature":1,"weather_code":0,"wind_speed_10m":1}}`))
	}))
	defer server.Close()
	weather := newWeatherClient(server.Client())
	weather.geocodingEndpoint = server.URL + "/search"
	weather.forecastEndpoint = server.URL + "/forecast"
	if _, err := weather.Current(context.Background(), "Nowhere"); err == nil {
		t.Fatal("Current() with invalid weather data unexpectedly succeeded")
	}
}

func TestToolBoxWeatherRejectsUnknownParameters(t *testing.T) {
	tools := NewToolBox(nil)
	_, err := tools.Execute(context.Background(), 1, "Tokyo weather", toolGetWeather, json.RawMessage(`{"location":"Tokyo","endpoint":"http://localhost"}`))
	if err == nil {
		t.Fatal("weather tool accepted an unrecognized endpoint parameter")
	}
}
