package handler

import (
	"net/http"

	"github.com/aitjcize/photoframe-server/server/internal/service"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	settings *service.SettingsService
	telegram *service.TelegramService
}

func NewHandler(s *service.SettingsService, t *service.TelegramService) *Handler {
	return &Handler{settings: s, telegram: t}
}

func (h *Handler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) GetSettings(c echo.Context) error {
	settings, err := h.settings.GetAll()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, settings)
}

type UpdateSettingsRequest struct {
	Settings map[string]string `json:"settings"`
}

func (h *Handler) UpdateSettings(c echo.Context) error {
	var req UpdateSettingsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	for k, v := range req.Settings {
		if err := h.settings.Set(k, v); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		// Dynamic Telegram Restart
		if k == "telegram_bot_token" {
			go h.telegram.Restart(v)
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}
