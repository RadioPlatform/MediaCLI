# Radioplatform Media CLI

A command-line tool for managing your RadioPlatform station media library — upload, organise, and browse audio files across stations, folders, and directories.

## Features

- **Upload files** — single files, glob patterns, or entire directories
- **Station management** — list, select, and switch between stations
- **Folder management** — create and list remote folders
- **Batch operations** — non-interactive uploads with JSON output
- **Metadata parsing** — reads ID3v1/ID3v2, MP4, FLAC, and OGG tags
- **Chunked uploads** — retryable 5 MiB chunks with server-confirmed byte progress
- **Interactive login** — guided setup with station selection

## Installation

### macOS

**Apple Silicon (arm64):**

```bash
tar -xzf media-cli_Darwin_arm64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

**Intel (amd64):**

```bash
tar -xzf media-cli_Darwin_amd64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

### Linux

**amd64:**

```bash
tar -xzf media-cli_Linux_amd64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

**arm64:**

```bash
tar -xzf media-cli_Linux_arm64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

### Go install

```bash
go install radioplatform-media-ci/cmd/media-cli@latest
```

> **Note:** Windows is not currently supported.

## Quick start

```console
$ media-cli login
Radioplatform Media CLI

Server: https://radio.example.com
CLI API key: **************

✓ Credentials validated

Select the default station:
  Accra Radio
  Kumasi FM
> Test Station

✓ Logged in
✓ Default station set to Accra Radio
```

Generate a CLI API key in **Account Settings → CLI API keys** before logging in.

Provide the server URL when prompted during interactive login. For non-interactive
use, set both `RADIO_PLATFORM_CLI_KEY` and `RADIO_PLATFORM_CLI_URL`.

## Usage

### Stations

| Command | Description |
|---------|-------------|
| `media-cli stations list` | List accessible stations |
| `media-cli stations use "Accra Radio"` | Set the default station |
| `media-cli media upload song.mp3 --station "Kumasi FM"` | Override station for one command |

The CLI uses the persisted default station when `--station` is not supplied.

### Folders

```bash
media-cli folders list
media-cli folders create "High Rotation"
media-cli folders create "Jingles" --station 2f71a6cb
```

### Upload media

Files are sent through the chunk-upload API in retryable 5 MiB parts. Interactive terminals display one server-confirmed byte progress bar per active file; JSON mode emits only the final result document.

```bash
# Single file
media-cli media upload song.mp3

# Multiple files
media-cli media upload song1.mp3 song2.mp3

# Glob pattern
media-cli media upload "./tracks/*.mp3" --folder "High Rotation"

# Directory (maps to remote folder with same name)
media-cli media upload ./New-Releases

# Directory into a specific folder
media-cli media upload ./Music --folder "High Rotation"

# Multiple directories with auto-creation
media-cli media upload ./Music ./Jingles --create-folders --yes

# Upload as jingles
media-cli media upload ./Jingles --jingle

# Non-interactive batch with JSON output
media-cli media upload ./Music \
  --station "Accra Radio" \
  --create-folders \
  --yes \
  --json

# Process results with jq
media-cli media upload ./Music --json | jq '.results[] | select(.success == false)'
```

### List media

```bash
media-cli media list
media-cli media list --station "Accra Radio"
media-cli media list --folder "High Rotation"
media-cli media list --search "afrobeats"
media-cli media list --page 2 --per-page 100
media-cli media list --search "station id" --json
```

## Configuration

Configuration is stored in `~/.config/rpmedia-cli/config.json` (or `$XDG_CONFIG_HOME/rpmedia-cli/config.json` if `XDG_CONFIG_HOME` is set).

The server URL can be stored in the configuration file as `server_url` or set at
runtime with `RADIO_PLATFORM_CLI_URL`. The environment variable takes precedence,
matching the behavior of `RADIO_PLATFORM_CLI_KEY`.

```json
{
  "server_url": "https://radio.example.com",
  "api_key": "your-cli-api-key"
}
```

| Setting | Details |
|---------|---------|
| Config directory permissions | `0700` |
| Config file permissions | `0600` |
| Environment override | `RADIO_PLATFORM_CLI_KEY` temporarily overrides the stored API key |

The CLI never creates API keys — generate one in **Account Settings → CLI API keys**.

## Troubleshooting

| Error | Solution |
|-------|----------|
| No CLI API key is configured | Run `media-cli login` |
| Invalid, expired, or revoked key | Generate a new key in Account Settings → CLI API keys |
| No destination station is configured | Run `media-cli stations use <uuid-or-name>` or provide `--station` |
| Ambiguous station name | Use a full UUID, unique prefix, or more specific name fragment |
| Missing remote folder | Create it with `media-cli folders create "Name" --station "Station"` or use `--create-folders` |

## Development

### Prerequisites

- Go 1.26+

### Commands

```bash
make build    # Build the binary
make test     # Run tests
make vet      # Run go vet
make clean    # Clean build artifacts
make install  # Install via go install
```

## License

MIT — see [LICENSE](./LICENSE) for details.
