package telegram

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aitjcize/photoframe-server/server/internal/model"
	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm"
)

type Bot struct {
	b       *tele.Bot
	db      *gorm.DB
	dataDir string
}

func NewBot(token string, db *gorm.DB, dataDir string) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	bot := &Bot{b: b, db: db, dataDir: dataDir}
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

	// Update Caption Setting (Simple KV store)
	caption := c.Message().Caption
	// We'll use a direct DB query or need a settings service access.
	// Since we only have *gorm.DB, let's just do a raw upsert or use model.Setting
	var setting model.Setting
	setting.Key = "telegram_caption"
	setting.Value = caption
	bot.db.Save(&setting)

	return c.Send("Photo updated! It will be displayed cleanly on the frame.")
}
