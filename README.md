# ESP32 PhotoFrame Server

A middleware server for the [ESP32 PhotoFrame](https://github.com/aitjcize/esp32-photoframe) project. This server acts as a bridge between the E-Ink display and various photo sources (Google Photos, Telegram), handling image processing, resizing, dithering, and overlay generation.

## Features

-   **Dual Data Sources**:
    -   **Google Photos**: Uses the Picker API to securely select albums and photos.
    -   **Telegram Bot**: Send photos directly to your frame via a Telegram bot.
-   **Smart Image Processing**:
    -   Automatic cropping to device aspect ratio (800x480 or 480x800).
    -   **Smart Collage**: Automatically combines two landscape photos in portrait mode (or vice versa) to maximize screen usage.
    -   **Dithering**: Applies Floyd-Steinberg dithering optimized for 7-color E-Ink displays (reusing the same logic as the firmware).
-   **Overlays**:
    -   Customizable Date/Time display.
    -   Real-time Weather status (Temperature + Condition) based on location.
    -   "iPhone Lockscreen" style aesthetics with Inter font and drop shadows.
-   **Web Interface**:
    -   Modern Vue 3 + Tailwind CSS dashboard.
    -   Manage settings: Orientation, Weather location, Collage mode.
    -   Manage gallery: View and delete imported photos.
    -   Import photos via Google Photos Picker.

## Deployment (Docker)

The easiest way to run the server is using Docker.

### 1. Build & Run locally

```bash
# Build the image
make build

# Run the container
# -p 9607:9607 : Expose web UI
# -v $(pwd)/data:/data : Persist database and photos
make run
```

### 2. Manual Docker Run

```bash
docker run -d \
  -p 9607:9607 \
  -v /path/to/data:/data \
  --name photoframe-server \
  aitjcize/esp32-photoframe-server:latest
```

## Configuration

Access the dashboard at `http://localhost:9607` (or your server IP).

### Google Photos Setup
1.  Go to **Settings**.
2.  Select **Source: Google Photos**.
3.  You need to provide your own Google Cloud Project credentials:
    -   Create a project in [Google Cloud Console](https://console.cloud.google.com/).
    -   Enable the **Google Photos Picker API**.
    -   Create OAuth 2.0 Credentials.
    -   Set **Authorized Javascript Origins** to your server URL (e.g., `http://localhost:9607`).
    -   Set **Authorized Redirect URIs** to `http://localhost:9607/api/auth/google/callback`.
4.  Enter the **Client ID** and **Client Secret** in the dashboard settings and click save.
5.  Go to the **Gallery** tab and click **Add Photos**.

### Telegram Setup
1.  Create a new bot via [@BotFather](https://t.me/botfather) on Telegram.
2.  Get the **Bot Token**.
3.  Go to **Settings** in the dashboard.
4.  Select **Source: Telegram Bot**.
5.  Enter your Bot Token and save.
6.  Send a photo to your bot on Telegram. The frame will update to show this photo immediately.

## API Endpoints (For ESP32)

-   `GET /image`: Returns the processed, dithered image ready for display.
    -   The ESP32 firmware should point to this endpoint.
    -   Headers: `X-Thumbnail-URL` (optional link to original).

## Development

### Tech Stack
-   **Backend**: Go (Golang) + Echo Framework + GORM (SQLite).
-   **Frontend**: Vue 3 + Vite + Tailwind CSS.
-   **Image Processing**: Node.js script (embedded) using `canvas` and `ditherjs` logic.

### Commands
-   `make build`: Build Docker image.
-   `make run`: Run Docker container.
-   `make format`: Format Go and Frontend code.
