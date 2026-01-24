package handler

import (
	"net/http"

	"github.com/aitjcize/photoframe-server/server/internal/service"
	"github.com/aitjcize/photoframe-server/server/pkg/googlephotos"
	"github.com/labstack/echo/v4"
)

type GoogleHandler struct {
	client *googlephotos.Client
	picker *service.PickerService
}

func NewGoogleHandler(client *googlephotos.Client, picker *service.PickerService) *GoogleHandler {
	return &GoogleHandler{
		client: client,
		picker: picker,
	}
}

func (h *GoogleHandler) Login(c echo.Context) error {
	// Construct redirect URL from request
	scheme := "http"
	if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request().Host
	redirectURL := scheme + "://" + host + "/api/auth/google/callback"

	h.client.SetRedirectURL(redirectURL)
	url := h.client.GetAuthURL()
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *GoogleHandler) Callback(c echo.Context) error {
	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "code is required"})
	}

	if err := h.client.Exchange(code); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Redirect back to frontend
	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

// Deprecated: Library API is restricted
func (h *GoogleHandler) ListAlbums(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]string{"error": "This feature is no longer supported by Google Photos API"})
}

func (h *GoogleHandler) CreatePickerSession(c echo.Context) error {
	id, uri, err := h.picker.CreateSession()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"id":        id,
		"pickerUri": uri,
	})
}

func (h *GoogleHandler) PollPickerSession(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id required"})
	}

	complete, err := h.picker.PollSession(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if complete {
		// If complete, trigger download in background? Or do it synchronously?
		// Synchronously might time out.
		// Ideally: return "complete: true", frontend shows "Processing..."
		// Then frontend calls /api/google/picker/process/{id}
		return c.JSON(http.StatusOK, map[string]bool{"complete": true})
	}

	return c.JSON(http.StatusOK, map[string]bool{"complete": false})
}

func (h *GoogleHandler) ProcessPickerSession(c echo.Context) error {
	id := c.Param("id")

	// Check if already processing? For now blindly start.
	go func() {
		_, err := h.picker.ProcessSessionItems(id)
		if err != nil {
			// Error is recorded in progress state
			// fmt.Printf("ProcessSessionItems background error: %v\n", err)
		}
	}()

	return c.JSON(http.StatusAccepted, map[string]string{"status": "processing"})
}

func (h *GoogleHandler) PollPickerProgress(c echo.Context) error {
	id := c.Param("id")
	progress := h.picker.GetProgress(id)
	if progress == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
	}
	return c.JSON(http.StatusOK, progress)
}
