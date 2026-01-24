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

> [!IMPORTANT]
> **Google OAuth Restriction**: Google does not allow `.local` domains or private IP addresses in OAuth redirect URIs. If running on Home Assistant, you must use one of these methods:
> - **Port Forwarding** (recommended for one-time setup): `ssh -L 9607:localhost:9607 root@homeassistant.local -p 22222`
> - **Public Domain**: Use a domain name with Cloudflare Tunnel or similar

#### Steps:

1.  **Create OAuth Credentials**:
    -   Go to [Google Cloud Console](https://console.cloud.google.com/)
    -   Create a new project or select an existing one
    -   Enable the **Google Photos Picker API**
    -   Go to **Credentials** → **Create Credentials** → **OAuth 2.0 Client ID**
    -   Application type: **Web application**
    -   **Authorized JavaScript Origins**: `http://localhost:9607`
    -   **Authorized Redirect URIs**: `http://localhost:9607/api/auth/google/callback`
    -   Click **Create** and save your Client ID and Client Secret

2.  **Configure the Server**:
    -   If running on Home Assistant, set up port forwarding first:
        ```bash
        ssh -L 9607:localhost:9607 root@homeassistant.local -p 22222
        ```
    -   Access the dashboard at `http://localhost:9607`
    -   Go to **Settings**
    -   Select **Source: Google Photos**
    -   Enter your **Client ID** and **Client Secret**
    -   Click **Save All Settings**

3.  **Authenticate and Import Photos**:
    -   Go to the **Gallery** tab
    -   Click **Add Photos**
    -   You'll be redirected to Google OAuth (sign in if needed)
    -   Select the photos you want to display
    -   Click **Add** to import them

4.  **After Setup**:
    -   The OAuth token is saved in the database
    -   You can close the SSH tunnel (if used)
    -   Access the server normally via `http://homeassistant.local:9607` or your regular URL
    -   Re-authentication is only needed if you revoke access or want to add more photos

### Telegram Setup
1.  Create a new bot via [@BotFather](https://t.me/botfather) on Telegram.
2.  Get the **Bot Token**.
3.  Go to **Settings** in the dashboard.
4.  Select **Source: Telegram Bot**.
5.  Enter your Bot Token and save.
6.  Send a photo to your bot on Telegram. The frame will update to show this photo immediately.

## API Endpoints (For ESP32)

-   `GET /image`: Returns the processed, dithered image ready for display.
    -   The ESP32 firmware's auto rotate URL should point to this endpoint.
    -   Headers: `X-Thumbnail-URL` (optional link to thumbnail).

## Development

### Tech Stack
-   **Backend**: Go (Golang) + Echo Framework + GORM (SQLite).
-   **Frontend**: Vue 3 + Vite + Tailwind CSS.

### Commands
-   `make build`: Build Docker image.
-   `make run`: Run Docker container.
-   `make format`: Format Go and Frontend code.
