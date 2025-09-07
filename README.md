# Cobblepod

This is a Go rewrite of the original Python M3U8 audio processor. It processes M3U8 playlists from Google Drive, downloads and processes audio files at configurable speeds, and generates podcast RSS feeds.

## Features

- Monitors Google Drive for M3U8 playlist files
- Downloads and processes audio files from M3U8 playlists
- Adjustable audio playback speed using FFmpeg
- Generates podcast RSS feeds with processed audio
- Uploads processed files back to Google Drive
- Reuses existing processed files when possible

## Requirements

- Docker
- Your Google 

## Running with Docker

First, setup `gcloud` (I'll assume you can manage this package on your own):

```bash
# Auth
gcloud auth login

# Set up Application Default Credentials
gcloud auth application-default login

# Set your project ID
gcloud config set project YOUR_PROJECT_ID

# Enable required APIs
gcloud services enable drive.googleapis.com

# Enable scopes
gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform

# Ensure the container can read (I KNOW, first thing I'll fix a frontend)
chmod +r .config/gcloud/application_default_credentials.json
```

Then, pull the compose file:

```bash 
curl "https://raw.githubusercontent.com/micahg/cobblepod/refs/heads/main/docker-compose.yml" -o cobblepod-compose.yml
```

Run `crontab -e` to edit your user's crontab, and add the following:

```
@reboot docker compose -f cobblepod-compose.yml up -d
```

or start manually by running the docker compose command above.

To check your logs, you can run:

```bash
docker compose -f cobblepod-compose.yml logs -f cobblepod
```

### Updating

```
docker compose -f cobblepod-compose.yml pull
docker compose -f cobblepod-compose.yml up --force-recreate cobblepod
```


### Without Compose

```
docker run -v "$HOME/.config/gcloud:/home/appuser/.config/gcloud" cobblepod
```

Note, you need to make `$HOME/.config/gcloud/application_default_credentials.json` readable inside the docker container. *THIS IS A SECURITY PROBLEM AND I KNOW IT*. I'm hoping to make a proper auth fix for this in the future (where you'd sign in as a client) -- might not work though because I think google wants a reachable URL. Sadly, device code doesn't work with the google cloud permissions we need 😭

## Development

1. Install dependencies:
   ```bash
   go mod tidy
   ```

2. Set up Google Cloud authentication:
   ```bash
   gcloud auth application-default login
   ```

3. Set environment variables (optional):
   ```bash
   export GOOGLE_CLOUD_PROJECT_ID=your-project-id
   export DEFAULT_SPEED=1.5
   export MAX_WORKERS=4
   ```

4. Valkey
   ```bash
   docker run -d --name valkey -p 6379:6379 valkey/valkey:8.1.3
   ```
## Usage

Build and run:
```bash
go build -o cobblepod main.go
./cobblepod
```

Or run directly:
```bash
go run main.go
```

## Project Structure

```
.
├── main.go                 # Main application entry point
├── go.mod                  # Go module definition
├── pkg/
│   ├── audio/
│   │   └── processor.go    # Audio processing logic
│   ├── config/
│   │   └── config.go       # Configuration management
│   ├── gdrive/
│   │   └── gdrive.go       # Google Drive API interactions
│   └── podcast/
│       └── rss.go          # RSS feed generation
```

## Key Components

### Audio Processor (`pkg/audio/processor.go`)
- Handles M3U8 playlist parsing
- Downloads audio files
- Processes audio with FFmpeg for speed adjustment
- Manages processing jobs and status

### Google Drive Service (`pkg/gdrive/gdrive.go`)
- Provides Google Drive API integration
- Handles file uploads, downloads, and permissions
- Manages file searches and metadata

### Podcast RSS Processor (`pkg/podcast/rss.go`)
- Generates RSS XML feeds from processed audio files
- Extracts episode mappings from existing feeds
- Handles RSS metadata and iTunes-specific tags

### Configuration (`pkg/config/config.go`)
- Manages environment variables and default settings
- Provides centralized configuration access

## Migration from Python

This Go version maintains functional compatibility with the original Python implementation:

- Same Google Drive integration patterns
- Compatible RSS feed format
- Equivalent audio processing workflow
- Similar configuration options

Key improvements in the Go version:
- Better concurrency handling
- More explicit error handling
- Improved type safety
- Better resource management
- Faster execution
