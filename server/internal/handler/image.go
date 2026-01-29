package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"log"

	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aitjcize/photoframe-server/server/internal/model"
	"github.com/aitjcize/photoframe-server/server/internal/service"
	"github.com/aitjcize/photoframe-server/server/pkg/googlephotos"
	"github.com/aitjcize/photoframe-server/server/pkg/imageops"
	"github.com/aitjcize/photoframe-server/server/pkg/photoframe"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ImageHandler struct {
	settings  *service.SettingsService
	overlay   *service.OverlayService
	processor *service.ProcessorService
	google    *googlephotos.Client
	synology  *service.SynologyService
	db        *gorm.DB
	dataDir   string
}

func NewImageHandler(
	s *service.SettingsService,
	o *service.OverlayService,
	p *service.ProcessorService,
	g *googlephotos.Client,
	synology *service.SynologyService,
	db *gorm.DB,
	dataDir string,
) *ImageHandler {
	return &ImageHandler{
		settings:  s,
		overlay:   o,
		processor: p,
		google:    g,
		synology:  synology,
		db:        db,
		dataDir:   dataDir,
	}
}

func (h *ImageHandler) ServeImage(c echo.Context) error {
	// Get source from route parameter
	source := c.Param("source")

	// Validate source is one of the allowed values
	if source != "google_photos" && source != "synology" && source != "telegram" {
		return c.NoContent(http.StatusNotFound)
	}

	// 1. Identify Device and Determine Settings
	// Try to find device by Hostname (X-Hostname header) first, then IP
	var device model.Device
	var result *gorm.DB

	hostname := c.Request().Header.Get("X-Hostname")
	if hostname != "" {
		// Try matching Host or Name? Host in DB is often hostname.
		result = h.db.Where("host = ?", hostname).First(&device)
	}

	// If not found by hostname, try by IP
	deviceFound := result != nil && result.Error == nil
	if !deviceFound {
		clientIP := c.RealIP()
		result = h.db.Where("host = ?", clientIP).First(&device)
		deviceFound = result.Error == nil
	}

	// Native resolution of the device panel
	nativeW, nativeH := 800, 480
	// Logical resolution for image generation (respects orientation)
	logicalW, logicalH := 800, 480

	enableCollage := false
	showDate := false
	showWeather := false
	var lat, lon float64

	if deviceFound {
		nativeW = device.Width
		nativeH = device.Height
		logicalW, logicalH = nativeW, nativeH

		enableCollage = device.EnableCollage
		showDate = device.ShowDate
		showWeather = device.ShowWeather
		lat = device.WeatherLat
		lon = device.WeatherLon
	}

	// ALWAYS overrides logical resolution/orientation from Headers if present
	if wStr := c.Request().Header.Get("X-Display-Width"); wStr != "" {
		if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
			logicalW = w
			nativeW = w
			if deviceFound && device.Width != w {
				device.Width = w
				h.db.Model(&device).Update("width", w)
			}
		}
	}
	if hStr := c.Request().Header.Get("X-Display-Height"); hStr != "" {
		if he, err := strconv.Atoi(hStr); err == nil && he > 0 {
			logicalH = he
			nativeH = he
			if deviceFound && device.Height != he {
				device.Height = he
				h.db.Model(&device).Update("height", he)
			}
		}
	}
	if oStr := c.Request().Header.Get("X-Display-Orientation"); oStr != "" {
		if oStr == "portrait" && logicalW > logicalH {
			logicalW, logicalH = logicalH, logicalW
		} else if oStr == "landscape" && logicalW < logicalH {
			logicalW, logicalH = logicalH, logicalW
		}
		// Persist orientation update to database if it changed
		if deviceFound && device.Orientation != oStr {
			device.Orientation = oStr
			h.db.Model(&device).Update("orientation", oStr)
		}
	} else if deviceFound && device.Orientation != "" {
		// Use device orientation preference if no header provided
		if device.Orientation == "portrait" && logicalW > logicalH {
			logicalW, logicalH = logicalH, logicalW
		} else if device.Orientation == "landscape" && logicalW < logicalH {
			logicalW, logicalH = logicalH, logicalW
		}
	}

	var img image.Image
	var err error

	if source == "telegram" {
		if enableCollage {
			// Smart Collage for Telegram (requires DB entries)
			img, _, err = h.fetchSmartCollage(logicalW, logicalH, source)
			if err != nil {
				// Fallback to single telegram photo if no DB entries
				imgPath := filepath.Join(h.dataDir, "photos", "telegram_last.jpg")
				f, fsErr := os.Open(imgPath)
				if fsErr != nil {
					img, err = h.fetchPlaceholder()
				} else {
					defer f.Close()
					img, _, err = image.Decode(f)
				}
			}
		} else {
			// Single photo from DB (newest telegram photo)
			img, _, err = h.fetchRandomPhoto(source)
			if err != nil {
				// Fallback to telegram_last.jpg
				imgPath := filepath.Join(h.dataDir, "photos", "telegram_last.jpg")
				f, fsErr := os.Open(imgPath)
				if fsErr != nil {
					img, err = h.fetchPlaceholder()
				} else {
					defer f.Close()
					img, _, err = image.Decode(f)
				}
			}
		}
	} else if enableCollage {
		img, _, err = h.fetchSmartCollage(logicalW, logicalH, source)
	} else {
		img, _, err = h.fetchRandomPhoto(source)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch photo: " + err.Error()})
	}

	// 1.5. Resize/Crop to Target Dimensions
	dst := image.NewRGBA(image.Rect(0, 0, logicalW, logicalH))
	imageops.DrawCover(dst, dst.Bounds(), img)
	img = dst

	// 2. Overlay
	overlayOpts := service.OverlayOptions{
		ShowDate:    showDate,
		ShowWeather: showWeather,
		WeatherLat:  lat,
		WeatherLon:  lon,
	}

	imgWithOverlay, err := h.overlay.ApplyOverlay(img, overlayOpts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "overlay failed: " + err.Error()})
	}

	// 3. Tone Mapping + Thumbnail (CLI)
	// Pass NATIVE dimensions to CLI.
	// The CLI will detect Source (logicalW/H) vs Target (nativeW/H) orientation mismatch and rotate if needed.
	procOptions := map[string]string{
		"dimension": fmt.Sprintf("%dx%d", nativeW, nativeH),
	}

	// 3.5. Parse X-Processing-Settings header if present
	var settings *photoframe.ProcessingSettings
	if settingsStr := c.Request().Header.Get("X-Processing-Settings"); settingsStr != "" {
		settings = &photoframe.ProcessingSettings{}
		if err := json.Unmarshal([]byte(settingsStr), settings); err != nil {
			fmt.Printf("Failed to parse X-Processing-Settings header: %v\n", err)
			settings = nil
		}
	}

	// 3.6. Parse X-Color-Palette header if present
	var palette *photoframe.Palette
	if paletteStr := c.Request().Header.Get("X-Color-Palette"); paletteStr != "" {
		palette = &photoframe.Palette{}
		if err := json.Unmarshal([]byte(paletteStr), palette); err != nil {
			fmt.Printf("Failed to parse X-Color-Palette header: %v\n", err)
			palette = nil
		}
	}

	headerOpts := h.processor.MapProcessingSettings(settings, palette)
	for k, v := range headerOpts {
		procOptions[k] = v
	}

	log.Println("Processing image with options: ", procOptions)
	processedBytes, thumbBytes, err := h.processor.ProcessImage(imgWithOverlay, procOptions)
	if err != nil {
		fmt.Printf("Processor failed: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "processor service failed: " + err.Error()})
	}

	// 4. Cache Thumbnail & Set Headers
	if thumbBytes != nil {
		thumbID := fmt.Sprintf("%d", time.Now().UnixNano())
		thumbPath := filepath.Join(h.dataDir, fmt.Sprintf("thumb_%s.jpg", thumbID))

		if err := os.WriteFile(thumbPath, thumbBytes, 0644); err == nil {
			thumbnailUrl := fmt.Sprintf("http://%s/served-image-thumbnail/%s", c.Request().Host, thumbID)
			c.Response().Header().Set("X-Thumbnail-URL", thumbnailUrl)
		} else {
			fmt.Printf("Failed to save served thumbnail: %v\n", err)
		}
	}

	// Set Content-Length header
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(processedBytes)))

	return c.Blob(http.StatusOK, "image/png", processedBytes)
}

func (h *ImageHandler) GetServedImageThumbnail(c echo.Context) error {
	id := c.Param("id")
	// Prevent directory traversal
	if id == "" || id == "." || id == ".." {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	thumbPath := filepath.Join(h.dataDir, fmt.Sprintf("thumb_%s.jpg", id))
	data, err := os.ReadFile(thumbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "thumbnail not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read thumbnail"})
	}

	// Delete after 5 minutes instead of immediately
	go func() {
		time.Sleep(5 * time.Minute)
		os.Remove(thumbPath)
	}()

	// Set Content-Length header
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	return c.Blob(http.StatusOK, "image/jpeg", data)
}

// Helper to retrieve settings safely
func (h *ImageHandler) getOrientation() string {
	val, err := h.settings.Get("orientation")
	if err != nil || val == "" {
		return "landscape"
	}
	return val
}

// Fetch smart photo (Single or Collage)
func (h *ImageHandler) fetchSmartCollage(screenW, screenH int, sourceFilter string) (image.Image, uint, error) {
	// Decide if Device is Landscape or Portrait
	devicePortrait := screenH > screenW

	// Fetch first image
	img1, id1, err := h.fetchRandomPhoto(sourceFilter)
	if err != nil {
		return nil, 0, err
	}

	bounds := img1.Bounds()
	w, h_img := bounds.Dx(), bounds.Dy()
	isPhotoPortrait := h_img > w

	// Case 1: Match
	if isPhotoPortrait == devicePortrait {
		return img1, id1, nil
	}

	// Case 2: Mismatch
	// Device Portrait, Photo Landscape -> Vertical Stack
	if devicePortrait && !isPhotoPortrait {
		// Try fetch second landscape
		// 1. Try DB first
		img2, id2, err := h.fetchRandomPhotoWithType("landscape", sourceFilter)
		if err == nil && id2 != id1 {
			return h.createVerticalCollage(img1, img2), 0, nil
		}
		// 2. Fallback: Try random loop
		for i := 0; i < 5; i++ {
			cand, candID, err := h.fetchRandomPhoto(sourceFilter)
			if err == nil && candID != id1 {
				b := cand.Bounds()
				if b.Dx() > b.Dy() { // Is Landscape
					// fmt.Printf("SmartCollage: Found match via random!\n")
					return h.createVerticalCollage(img1, cand), 0, nil
				}
			}
		}
		// Fallback: Use same photo twice
		return h.createVerticalCollage(img1, img1), 0, nil
	}

	// Device Landscape, Photo Portrait -> Horizontal Side-by-Side
	if !devicePortrait && isPhotoPortrait {
		// Try fetch second portrait
		img2, id2, err := h.fetchRandomPhotoWithType("portrait", sourceFilter)
		if err == nil && id2 != id1 {
			return h.createHorizontalCollage(img1, img2), 0, nil
		}
		// 2. Fallback
		for i := 0; i < 5; i++ {
			cand, candID, err := h.fetchRandomPhoto(sourceFilter)
			if err == nil && candID != id1 {
				b := cand.Bounds()
				if b.Dy() > b.Dx() { // Is Portrait
					// fmt.Printf("SmartCollage: Found match via random!\n")
					return h.createHorizontalCollage(img1, cand), 0, nil
				}
			}
		}
		// Fallback: Use same photo twice
		return h.createHorizontalCollage(img1, img1), 0, nil
	}

	return img1, id1, nil
}

func (h *ImageHandler) fetchRandomPhotoWithType(targetType string, sourceFilter string) (image.Image, uint, error) {
	var item model.Image
	query := h.db.Order("RANDOM()").Where("orientation = ?", targetType)

	if sourceFilter == "google_photos" {
		query = query.Where("source = ?", "google")
	} else if sourceFilter == "synology" {
		query = query.Where("source = ?", "synology")
	} else if sourceFilter == "telegram" {
		query = query.Where("source = ?", "telegram")
	} else {
		return nil, 0, fmt.Errorf("invalid source filter: %s", sourceFilter)
	}

	if err := query.First(&item).Error; err != nil {
		return nil, 0, err
	}

	resolvedPath := h.resolvePath(item.FilePath)
	f, err := os.Open(resolvedPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, err
	}
	return img, item.ID, nil
}

func (h *ImageHandler) createVerticalCollage(img1, img2 image.Image) image.Image {
	// Target Dimension: 480x800 (Portrait)
	// Each slot: 480x400
	width := 480
	height := 800
	slotHeight := 400

	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw Top
	imageops.DrawCover(dst, image.Rect(0, 0, width, slotHeight), img1)

	// Draw Bottom
	imageops.DrawCover(dst, image.Rect(0, slotHeight, width, height), img2)

	return dst
}

func (h *ImageHandler) createHorizontalCollage(img1, img2 image.Image) image.Image {
	// Target Dimension: 800x480 (Landscape)
	// Each slot: 400x480
	width := 800
	height := 480
	slotWidth := 400

	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw Left
	imageops.DrawCover(dst, image.Rect(0, 0, slotWidth, height), img1)

	// Draw Right
	imageops.DrawCover(dst, image.Rect(slotWidth, 0, width, height), img2)

	return dst
}

// fetchSynologyPhoto retrieves the photo from Synology Service
func (h *ImageHandler) fetchSynologyPhoto(item model.Image) (image.Image, uint, error) {
	// Try fetching cache first? Or direct from Service which handles fetching
	data, err := h.synology.GetPhoto(item.SynologyPhotoID, item.ThumbnailKey, "large")
	if err != nil {
		return nil, 0, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	return img, item.ID, nil
}

// resolvePath handles path differences between Docker (/data/...) and local dev
func (h *ImageHandler) resolvePath(path string) string {
	// 1. If path exists as is, return it
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// 2. If path starts with /data/, try replacing it with h.dataDir
	// Docker uses /data, local uses whatever DATA_DIR is (e.g. ./data)
	if strings.HasPrefix(path, "/data/") {
		relPath := strings.TrimPrefix(path, "/data/")
		newPath := filepath.Join(h.dataDir, relPath)
		if _, err := os.Stat(newPath); err == nil {
			return newPath
		}
	}

	// 3. Similar check for /app/data/ just in case
	if strings.HasPrefix(path, "/app/data/") {
		relPath := strings.TrimPrefix(path, "/app/data/")
		newPath := filepath.Join(h.dataDir, relPath)
		if _, err := os.Stat(newPath); err == nil {
			return newPath
		}
	}

	return path
}

func (h *ImageHandler) fetchRandomPhoto(sourceFilter string) (image.Image, uint, error) {
	// Source logic: if "google_photos" (default), we include source="google" OR source="" (legacy)
	// If "synology", source="synology"
	// If "telegram", source="telegram"

	// Note: settings uses "google_photos" but DB uses "google"? Or "local"?
	// Legacy: empty source is usually local or google.
	// We need to check data model.

	var item model.Image
	query := h.db.Order("RANDOM()")

	if sourceFilter == "google_photos" {
		query = query.Where("source = ?", "google")
	} else if sourceFilter == "synology" {
		query = query.Where("source = ?", "synology")
	} else if sourceFilter == "telegram" {
		query = query.Where("source = ?", "telegram")
	} else {
		return nil, 0, fmt.Errorf("invalid source filter: %s", sourceFilter)
	}

	result := query.First(&item)
	if result.Error != nil {
		img, err := h.fetchPlaceholder()
		return img, 0, err
	}

	if item.Source == "synology" {
		img, _, err := h.fetchSynologyPhoto(item)
		if err != nil {
			fmt.Printf("Warning: Failed to fetch Synology photo: %v\n", err)
			img, err := h.fetchPlaceholder()
			return img, 0, err
		}
		return img, item.ID, nil
	}

	resolvedPath := h.resolvePath(item.FilePath)
	f, err := os.Open(resolvedPath)
	if err != nil {
		// Do NOT delete the record just because file is missing locally
		// h.db.Delete(&item)
		fmt.Printf("Warning: Failed to open image: %s (resolved: %s): %v\n", item.FilePath, resolvedPath, err)
		img, err := h.fetchPlaceholder()
		return img, 0, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		img, err := h.fetchPlaceholder()
		return img, 0, err
	}
	return img, item.ID, nil
}

func (h *ImageHandler) fetchPlaceholder() (image.Image, error) {
	resp, err := http.Get("https://picsum.photos/800/480")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	return img, err
}

// ServeTelegramImageAfter returns the next Telegram image with update_id greater than the given updateID.
// Returns 204 No Content if no new image is available.
//
// Behavior:
// - updateID = 0: Returns the NEWEST image (for initial download)
// - updateID > 0: Returns the NEXT image (smallest update_id > given) for sequential download
//
// This allows the client to download all new images one by one:
//  1. Initial: /after/0 -> gets newest (update_id=104)
//  2. Next:   /after/104 -> gets next new (update_id=103) [if exists]
//  3. Next:   /after/103 -> gets next new (update_id=102) [if exists]
//  4. Next:   /after/102 -> 204 No Content (no more new images)
func (h *ImageHandler) ServeTelegramImageAfter(c echo.Context) error {
	updateIDStr := c.Param("updateID")
	updateID, err := strconv.ParseInt(updateIDStr, 10, 64)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// Check if collage mode is enabled via header
	enableCollage := c.Request().Header.Get("X-Collage-Enabled") == "true"

	// Get display dimensions from headers
	wStr := c.Request().Header.Get("X-Display-Width")
	hStr := c.Request().Header.Get("X-Display-Height")
	logicalW, logicalH := 800, 480

	if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
		logicalW = w
	}
	if he, err := strconv.Atoi(hStr); err == nil && he > 0 {
		logicalH = he
	}

	// Fetch all images with telegram_update_id > given updateID
	var items []model.Image
	query := h.db.Where("source = ? AND telegram_update_id > ?", "telegram", updateID)

	// Special case: if updateID is 0 (initial download), get the newest image directly
	// This avoids the need for polling to get the latest image
	if updateID == 0 {
		var newestImage model.Image
		result := h.db.Where("source = ?", "telegram").
			Order("telegram_update_id DESC").
			First(&newestImage)

		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				return c.NoContent(http.StatusNoContent)
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "database error"})
		}

		// Load and return the newest image
		img, maxUpdateID, err := h.loadImageFromItem(newestImage)
		if err != nil {
			return c.NoContent(http.StatusNoContent)
		}

		// Process single image (collage not applicable for initial download)
		dst := image.NewRGBA(image.Rect(0, 0, logicalW, logicalH))
		imageops.DrawCover(dst, dst.Bounds(), img)
		img = dst

		procOptions := map[string]string{
			"dimension": fmt.Sprintf("%dx%d", logicalW, logicalH),
		}
		processedBytes, thumbBytes, err := h.processor.ProcessImage(img, procOptions)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "processor failed"})
		}

		c.Response().Header().Set("X-Update-ID", fmt.Sprintf("%d", maxUpdateID))

		if thumbBytes != nil {
			thumbID := fmt.Sprintf("%d", time.Now().UnixNano())
			thumbPath := filepath.Join(h.dataDir, fmt.Sprintf("thumb_%s.jpg", thumbID))
			if err := os.WriteFile(thumbPath, thumbBytes, 0644); err == nil {
				thumbnailUrl := fmt.Sprintf("http://%s/served-image-thumbnail/%s", c.Request().Host, thumbID)
				c.Response().Header().Set("X-Thumbnail-URL", thumbnailUrl)
			}
		}

		c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(processedBytes)))
		return c.Blob(http.StatusOK, "image/png", processedBytes)
	}

	result := query.Order("telegram_update_id ASC").Find(&items)

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "database error"})
	}

	if len(items) == 0 {
		// No new images available
		return c.NoContent(http.StatusNoContent)
	}

	var img image.Image
	var maxUpdateID int64

	if enableCollage && len(items) >= 2 {
		// Smart Collage mode: fetch first image and create collage if needed
		img, err = h.fetchSmartCollageWithItems(logicalW, logicalH, items)
		if err != nil {
			// Fallback to single image (first/newest available)
			img, maxUpdateID, err = h.loadImageFromItem(items[0])
			if err != nil {
				return c.NoContent(http.StatusNoContent)
			}
		} else {
			// Use the first image's update ID for collage
			maxUpdateID = items[0].TelegramUpdateID
		}
	} else {
		// Single image mode: return the NEXT image (smallest update_id > given)
		// This allows the client to download all new images one by one
		img, maxUpdateID, err = h.loadImageFromItem(items[0])
		if err != nil {
			return c.NoContent(http.StatusNoContent)
		}
	}

	// Resize/Crop to target dimensions
	dst := image.NewRGBA(image.Rect(0, 0, logicalW, logicalH))
	imageops.DrawCover(dst, dst.Bounds(), img)
	img = dst

	// Process image
	procOptions := map[string]string{
		"dimension": fmt.Sprintf("%dx%d", logicalW, logicalH),
	}
	processedBytes, thumbBytes, err := h.processor.ProcessImage(img, procOptions)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "processor failed"})
	}

	// Set X-Update-ID header so client knows the update ID of this image
	c.Response().Header().Set("X-Update-ID", fmt.Sprintf("%d", maxUpdateID))

	// Handle thumbnail
	if thumbBytes != nil {
		thumbID := fmt.Sprintf("%d", time.Now().UnixNano())
		thumbPath := filepath.Join(h.dataDir, fmt.Sprintf("thumb_%s.jpg", thumbID))
		if err := os.WriteFile(thumbPath, thumbBytes, 0644); err == nil {
			thumbnailUrl := fmt.Sprintf("http://%s/served-image-thumbnail/%s", c.Request().Host, thumbID)
			c.Response().Header().Set("X-Thumbnail-URL", thumbnailUrl)
		}
	}

	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(processedBytes)))
	return c.Blob(http.StatusOK, "image/png", processedBytes)
}

// loadImageFromItem loads an image from a model.Image item
func (h *ImageHandler) loadImageFromItem(item model.Image) (image.Image, int64, error) {
	resolvedPath := h.resolvePath(item.FilePath)
	f, err := os.Open(resolvedPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, item.TelegramUpdateID, err
}

// fetchSmartCollageWithItems creates a collage from available items
func (h *ImageHandler) fetchSmartCollageWithItems(screenW, screenH int, items []model.Image) (image.Image, error) {
	devicePortrait := screenH > screenW

	// Load all items as images
	var images []image.Image
	var updateIDs []int64
	for _, item := range items {
		img, updateID, err := h.loadImageFromItem(item)
		if err != nil {
			log.Printf("Failed to load image %s: %v", item.FilePath, err)
			continue
		}
		images = append(images, img)
		updateIDs = append(updateIDs, updateID)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no valid images for collage")
	}

	// First image
	img1 := images[0]
	bounds := img1.Bounds()
	w, h_img := bounds.Dx(), bounds.Dy()
	isPhotoPortrait := h_img > w

	// Case 1: Match - single photo is sufficient
	if isPhotoPortrait == devicePortrait {
		// Return the last image (newest) as single photo
		return images[len(images)-1], nil
	}

	// Case 2: Mismatch - need second image
	if len(images) < 2 {
		// Not enough images, return first
		return img1, nil
	}

	// Find matching second image
	for i := 1; i < len(images); i++ {
		b := images[i].Bounds()
		isPortrait := b.Dy() > b.Dx()

		if devicePortrait && !isPortrait {
			// Device Portrait, need Landscape -> Vertical Stack
			return h.createVerticalCollage(img1, images[i]), nil
		}

		if !devicePortrait && isPortrait {
			// Device Landscape, need Portrait -> Horizontal Side-by-Side
			return h.createHorizontalCollage(img1, images[i]), nil
		}
	}

	// No matching pair found, return first image
	return img1, nil
}
