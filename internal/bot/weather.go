package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxWeatherLocationLength = 100
	maxWeatherResponseBytes  = 1 << 20
)

type WeatherClient struct {
	client            *http.Client
	geocodingEndpoint string
	forecastEndpoint  string
}

func newWeatherClient(client *http.Client) *WeatherClient {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &WeatherClient{
		client:            client,
		geocodingEndpoint: "https://geocoding-api.open-meteo.com/v1/search",
		forecastEndpoint:  "https://api.open-meteo.com/v1/forecast",
	}
}

func (w *WeatherClient) Current(ctx context.Context, location string) (map[string]any, error) {
	if hasControlCharacters(location) {
		return nil, errors.New("location must contain only printable characters")
	}
	location = strings.TrimSpace(location)
	if location == "" || utf8.RuneCountInString(location) > maxWeatherLocationLength {
		return nil, errors.New("location must contain 1 to 100 printable characters")
	}

	geocodingURL, err := url.Parse(w.geocodingEndpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Open-Meteo geocoding URL: %w", err)
	}
	query := geocodingURL.Query()
	query.Set("name", location)
	query.Set("count", "1")
	query.Set("language", "en")
	query.Set("format", "json")
	geocodingURL.RawQuery = query.Encode()

	var geocoding struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := w.getJSON(ctx, geocodingURL.String(), &geocoding); err != nil {
		return nil, fmt.Errorf("Open-Meteo location lookup: %w", err)
	}
	if len(geocoding.Results) == 0 {
		return nil, errors.New("no matching location was found")
	}
	place := geocoding.Results[0]
	if strings.TrimSpace(place.Name) == "" || !validCoordinates(place.Latitude, place.Longitude) {
		return nil, errors.New("Open-Meteo returned invalid coordinates")
	}

	forecastURL, err := url.Parse(w.forecastEndpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Open-Meteo forecast URL: %w", err)
	}
	forecastQuery := forecastURL.Query()
	forecastQuery.Set("latitude", fmt.Sprintf("%f", place.Latitude))
	forecastQuery.Set("longitude", fmt.Sprintf("%f", place.Longitude))
	forecastQuery.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m")
	forecastQuery.Set("temperature_unit", "celsius")
	forecastQuery.Set("wind_speed_unit", "kmh")
	forecastQuery.Set("timezone", "auto")
	forecastURL.RawQuery = forecastQuery.Encode()

	var forecast struct {
		Current *struct {
			Time                string   `json:"time"`
			Temperature         *float64 `json:"temperature_2m"`
			RelativeHumidity    *int     `json:"relative_humidity_2m"`
			ApparentTemperature *float64 `json:"apparent_temperature"`
			WeatherCode         *int     `json:"weather_code"`
			WindSpeed           *float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}
	if err := w.getJSON(ctx, forecastURL.String(), &forecast); err != nil {
		return nil, fmt.Errorf("Open-Meteo current forecast: %w", err)
	}
	if forecast.Current == nil || forecast.Current.Temperature == nil ||
		forecast.Current.RelativeHumidity == nil || forecast.Current.ApparentTemperature == nil ||
		forecast.Current.WeatherCode == nil || forecast.Current.WindSpeed == nil ||
		!validWeatherNumber(*forecast.Current.Temperature) ||
		!validWeatherNumber(*forecast.Current.ApparentTemperature) ||
		!validWeatherNumber(*forecast.Current.WindSpeed) ||
		*forecast.Current.RelativeHumidity < 0 || *forecast.Current.RelativeHumidity > 100 {
		return nil, errors.New("Open-Meteo returned invalid weather data")
	}

	return map[string]any{
		"location":       formatPlace(place.Name, place.Admin1, place.Country),
		"observed_at":    forecast.Current.Time,
		"conditions":     weatherDescription(*forecast.Current.WeatherCode),
		"temperature_c":  *forecast.Current.Temperature,
		"feels_like_c":   *forecast.Current.ApparentTemperature,
		"humidity_pct":   *forecast.Current.RelativeHumidity,
		"wind_speed_kmh": *forecast.Current.WindSpeed,
		"source":         "Open-Meteo",
	}, nil
}

func (w *WeatherClient) getJSON(ctx context.Context, endpoint string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	response, err := w.client.Do(req)
	if err != nil {
		return errors.New("request failed")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxWeatherResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxWeatherResponseBytes {
		return errors.New("response exceeded the size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("service returned %s", response.Status)
	}
	if err := json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func validCoordinates(latitude, longitude float64) bool {
	return !math.IsNaN(latitude) && !math.IsInf(latitude, 0) &&
		!math.IsNaN(longitude) && !math.IsInf(longitude, 0) &&
		latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}

func validWeatherNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func formatPlace(name, region, country string) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{name, region, country} {
		part = strings.TrimSpace(part)
		if part == "" || len(parts) > 0 && strings.EqualFold(parts[len(parts)-1], part) {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func weatherDescription(code int) string {
	switch {
	case code == 0:
		return "clear sky"
	case code == 1:
		return "mainly clear"
	case code == 2:
		return "partly cloudy"
	case code == 3:
		return "overcast"
	case code == 45 || code == 48:
		return "fog"
	case code >= 51 && code <= 57:
		return "drizzle"
	case code >= 61 && code <= 67:
		return "rain"
	case code >= 71 && code <= 77:
		return "snow"
	case code >= 80 && code <= 82:
		return "rain showers"
	case code == 85 || code == 86:
		return "snow showers"
	case code == 95 || code == 96 || code == 99:
		return "thunderstorm"
	default:
		return "unknown conditions"
	}
}
