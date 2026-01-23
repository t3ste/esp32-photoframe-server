package service

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"github.com/aitjcize/photoframe-server/server/pkg/weather"
	"github.com/fogleman/gg"
)

type OverlayService struct {
	weatherClient *weather.Client
	settings      *SettingsService
}

func NewOverlayService(w *weather.Client, s *SettingsService) *OverlayService {
	return &OverlayService{weatherClient: w, settings: s}
}

func (s *OverlayService) ApplyOverlay(img image.Image) (image.Image, error) {
	dc := gg.NewContextForImage(img)
	w := float64(dc.Width())
	h := float64(dc.Height())

	// Dark gradient at bottom for legibility
	grad := gg.NewLinearGradient(0, h-100, 0, h)
	grad.AddColorStop(0, color.RGBA{0, 0, 0, 0})   // Transparent
	grad.AddColorStop(1, color.RGBA{0, 0, 0, 180}) // Black semi-transparent

	dc.SetFillStyle(grad)
	dc.DrawRectangle(0, h-100, w, 100)
	dc.Fill()

	// Load Font
	// Using Inter Variable font
	if err := dc.LoadFontFace("/usr/share/fonts/inter/InterVariable.ttf", 25); err != nil {
		fmt.Printf("Could not load font: %v\n", err)
	}

	// Draw Overlays based on Settings
	showDate, _ := s.settings.Get("show_date")
	if showDate == "" {
		showDate = "true"
	} // Default to true

	showWeather, _ := s.settings.Get("show_weather")
	if showWeather == "" {
		showWeather = "true"
	} // Default to true

	// Date
	if showDate == "true" {
		now := time.Now()
		dateStr := now.Format("Mon, Jan 02")
		// Draw Shadow
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(dateStr, 22, h-52, 0, 0.5)
		// Draw Text
		dc.SetRGB(1, 1, 1)
		dc.DrawStringAnchored(dateStr, 20, h-50, 0, 0.5)
	}

	// Weather
	if showWeather == "true" {
		lat, _ := s.settings.Get("weather_lat")
		lon, _ := s.settings.Get("weather_lon")

		if lat != "" && lon != "" {
			weather, err := s.weatherClient.GetWeather(lat, lon)
			if err == nil {
				weatherStr := fmt.Sprintf("%.1f°C %s", weather.Temperature, weather.Description())
				// Draw Shadow
				dc.SetRGB(0, 0, 0)
				dc.DrawStringAnchored(weatherStr, w-18, h-52, 1, 0.5)
				// Draw Text
				dc.SetRGB(1, 1, 1)
				dc.DrawStringAnchored(weatherStr, w-20, h-50, 1, 0.5)
			} else {
				// If weather fails, silently skip or maybe log
				fmt.Printf("Weather fetch failed: %v\n", err)
			}
		}
	}

	return dc.Image(), nil
}
