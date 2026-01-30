package telegram

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aitjcize/photoframe-server/server/internal/model"
	"github.com/aitjcize/photoframe-server/server/pkg/imageops"
	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm"
)

type SettingsProvider interface {
	Get(key string) (string, error)
}

type Pusher interface {
	PushToHost(device *model.Device, imagePath string, extraOpts map[string]string) error
}

type Bot struct {
	b        *tele.Bot
	db       *gorm.DB
	dataDir  string
	settings SettingsProvider
	pusher   Pusher
}

func NewBot(token string, db *gorm.DB, dataDir string, settings SettingsProvider, pusher Pusher) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		b:        b,
		db:       db,
		dataDir:  dataDir,
		settings: settings,
		pusher:   pusher,
	}
	bot.registerHandlers()

	return bot, nil
}

func (bot *Bot) Start() {
	log.Println("Telegram bot started")
	go bot.b.Start()
}

func (bot *Bot) Stop() {
	bot.b.Stop()
}

func (bot *Bot) registerHandlers() {
	bot.b.Handle("/start", func(c tele.Context) error {
		return c.Send("Hello! Send me a photo to display on your frame.")
	})

	bot.b.Handle(tele.OnPhoto, bot.handlePhoto)
}

func (bot *Bot) handlePhoto(c tele.Context) error {
	// Download photo
	photo := c.Message().Photo

	// Get the Telegram update ID for deduplication
	telegramUpdateID := int64(c.Update().ID)

	// Create directory if not exists
	photosDir := filepath.Join(bot.dataDir, "photos")
	if err := os.MkdirAll(photosDir, 0755); err != nil {
		return c.Send("Failed to create photos directory.")
	}

	// Target file path
	destPath := filepath.Join(photosDir, "telegram_last.jpg")

	// Download
	if err := bot.b.Download(&photo.File, destPath); err != nil {
		return c.Send("Failed to download photo: " + err.Error())
	}

	// Also save as unique file for collage support
	timestamp := time.Now().UnixNano()
	uniquePath := filepath.Join(photosDir, fmt.Sprintf("telegram_%d.jpg", timestamp))
	if err := copyFile(destPath, uniquePath); err != nil {
		log.Printf("Failed to create unique copy for collage: %v", err)
	}

	// Create DB entry for smart collage support
	imageEntry := model.Image{
		FilePath:         uniquePath,
		Source:           "telegram",
		Orientation:      getImageOrientation(uniquePath),
		CreatedAt:        time.Now(),
		TelegramUpdateID: telegramUpdateID,
	}
	if err := bot.db.Create(&imageEntry).Error; err != nil {
		log.Printf("Failed to create DB entry for Telegram photo: %v", err)
	}

	// Update Caption Setting
	caption := c.Message().Caption
	var setting model.Setting
	setting.Key = "telegram_caption"
	setting.Value = caption
	bot.db.Save(&setting)

	// Check if Push to Device is enabled
	pushEnabled, _ := bot.settings.Get("telegram_push_enabled")
	targetDeviceIDStr, _ := bot.settings.Get("telegram_target_device_id")

	if pushEnabled == "true" && targetDeviceIDStr != "" {
		// Send initial status
		statusMsg, err := bot.b.Send(c.Recipient(), "Connecting to device...")
		if err != nil {
			log.Printf("Failed to send status message: %v", err)
			return err
		}

		// Look up device
		var device model.Device
		if err := bot.db.First(&device, targetDeviceIDStr).Error; err != nil {
			log.Printf("Failed to find target device (ID: %s): %v", targetDeviceIDStr, err)
			bot.b.Edit(statusMsg, "Error: Configured target device not found.")
			return nil
		}

		err = bot.pusher.PushToHost(&device, destPath, nil)
		if err != nil {
			log.Printf("Failed to push to device: %v", err)
			_, editErr := bot.b.Edit(statusMsg, "Photo updated! Device is offline/unreachable, so it will show up next time the device awakes.")
			if editErr != nil {
				return c.Send("Photo updated! Device is offline/unreachable, so it will show up next time the device awakes.")
			}
			return nil
		}

		_, editErr := bot.b.Edit(statusMsg, "Photo updated and displayed on device!")
		if editErr != nil {
			return c.Send("Photo updated and displayed on device!")
		}
		return nil
	}

	return c.Send("Photo updated! It will show up next time the device awakes.")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// getImageOrientation determines if an image is portrait or landscape
func getImageOrientation(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "landscape"
	}
	defer f.Close()

	img, _, err := image.DecodeConfig(f)
	if err != nil {
		return "landscape"
	}

	if img.Height > img.Width {
		return "portrait"
	}
	return "landscape"
}

// createVerticalCollage creates a vertical collage (portrait: 480x800)
func createVerticalCollage(img1, img2 image.Image) image.Image {
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

// createHorizontalCollage creates a horizontal collage (landscape: 800x480)
func createHorizontalCollage(img1, img2 image.Image) image.Image {
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

// tryCreateCollage attempts to create a collage with an unpaired previous image
func (bot *Bot) tryCreateCollage(newImage model.Image) {
	var pairedImage model.Image

	result := bot.db.Where("source = ? AND orientation = ? AND status != ?", "telegram", newImage.Orientation, "collage_paired").
		Order("created_at ASC").
		First(&pairedImage)

	if result.Error != nil {
		if result.Error != gorm.ErrRecordNotFound {
			log.Printf("Failed to find paired image: %v", result.Error)
		}
		return
	}

	// Load both images
	newImg, err := loadImageForCollage(newImage.FilePath)
	if err != nil {
		log.Printf("Failed to load new image for collage: %v", err)
		return
	}

	pairedImg, err := loadImageForCollage(pairedImage.FilePath)
	if err != nil {
		log.Printf("Failed to load paired image for collage: %v", err)
		return
	}

	// Create collage
	var collage image.Image
	if newImage.Orientation == "portrait" {
		collage = createHorizontalCollage(pairedImg, newImg)
	} else {
		collage = createVerticalCollage(pairedImg, newImg)
	}

	// Save collage
	collagePath := filepath.Join(bot.dataDir, "photos", fmt.Sprintf("telegram_collage_%d.jpg", time.Now().UnixNano()))
	f, err := os.Create(collagePath)
	if err != nil {
		log.Printf("Failed to create collage file: %v", err)
		return
	}
	defer f.Close()

	if err := jpeg.Encode(f, collage, nil); err != nil {
		log.Printf("Failed to save collage: %v", err)
		return
	}

	// Mark as paired
	bot.db.Model(&newImage).Update("status", "collage_paired")
	bot.db.Model(&pairedImage).Update("status", "collage_paired")

	// Create collage entry
	collageEntry := model.Image{
		FilePath:         collagePath,
		Source:           "telegram",
		Orientation:      "collage",
		CreatedAt:        time.Now(),
		TelegramUpdateID: newImage.TelegramUpdateID,
	}
	if err := bot.db.Create(&collageEntry).Error; err != nil {
		log.Printf("Failed to create collage DB entry: %v", err)
	}

	log.Printf("Created collage from images %d and %d", newImage.ID, pairedImage.ID)
}

// loadImageForCollage loads an image from file
func loadImageForCollage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// drawCover is now using imageops.DrawCover from the imageops package
