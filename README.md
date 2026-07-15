# Radioplatform Media CLI

Manage your Radio Platform station media library from the command line.

## Installation

### From source

```bash
go install radioplatform-media-ci/cmd/media-cli@latest
```

### From GitHub Releases

**Linux (amd64):**

```bash
tar -xzf media-cli_Linux_amd64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

**Linux (arm64):**

```bash
tar -xzf media-cli_Linux_arm64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

**macOS (arm64):**

```bash
tar -xzf media-cli_Darwin_arm64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

**macOS (amd64):**

```bash
tar -xzf media-cli_Darwin_amd64.tar.gz
sudo install -m 0755 media-cli /usr/local/bin/media-cli
```

Windows is not supported.

### CI and releases

Every branch push and pull request runs formatting checks, `go vet`, race-enabled tests, and a snapshot build. The generated Linux and macOS archives are available from the workflow run for 14 days.

Push a semantic version tag to publish a GitHub Release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The release contains Linux and macOS binaries for amd64 and arm64, plus `checksums.txt`.

## Configuration

- The API server URL is hardcoded into the binary and cannot be overridden.
- For upgrade compatibility, CLI credentials remain stored in `~/.config/rpmedia-cli/config.json`.
- If `XDG_CONFIG_HOME` is set, the path remains `$XDG_CONFIG_HOME/rpmedia-cli/config.json`.
- The config directory is created with `0700` permissions.
- The config file is created with `0600` permissions.
- The `RADIO_PLATFORM_CLI_KEY` environment variable temporarily overrides the stored key.
- The CLI never creates API keys. Generate one in **Account Settings → CLI API keys**.

## Getting started

```console
$ media-cli login
Radioplatform Media CLI

Server: https://radio.example.com
CLI API key: **************

✓ Credentials validated

Select the default station:

> Accra Radio
  Kumasi FM
  Test Station

✓ Logged in
✓ Default station set to Accra Radio
```

## Station workflows

List accessible stations:

```bash
media-cli stations list
```

Set the default station:

```bash
media-cli stations use "Accra Radio"
```

Override the station for a single command:

```bash
media-cli media upload song.mp3 --station "Kumasi FM"
```

**Important rules:**

- Commands use the persisted default station when no override is supplied.
- `--station` changes only the current command. It does not update the persisted default.
- The destination station is always displayed before uploads begin.
- The CLI never chooses the first station automatically.

## Folder workflows

List folders:

```bash
media-cli folders list
```

Create a folder:

```bash
media-cli folders create "High Rotation"
```

Create a folder on a specific station:

```bash
media-cli folders create "Jingles" --station 2f71a6cb
```

## Media upload

Upload one file to the default station's media root:

```bash
media-cli media upload song.mp3
```

Upload to another station:

```bash
media-cli media upload song.mp3 --station "Kumasi FM"
```

Upload multiple files:

```bash
media-cli media upload song1.mp3 song2.mp3
```

Upload with a glob pattern:

```bash
media-cli media upload "./tracks/*.mp3" --folder "High Rotation"
```

Upload a directory recursively:

```bash
media-cli media upload ./New-Releases
```

This maps the local directory `New-Releases` to the remote folder `New-Releases` on the selected station.

Upload a directory into a specific folder:

```bash
media-cli media upload ./Music --folder "High Rotation"
```

Upload multiple directories into matching remote folders:

```bash
media-cli media upload ./Music ./Jingles --create-folders --yes
```

Upload all files as jingles:

```bash
media-cli media upload ./Jingles --jingle
```

Non-interactive batch upload:

```bash
media-cli media upload ./Music \
  --station "Accra Radio" \
  --create-folders \
  --yes \
  --json
```

Process JSON results with jq:

```bash
media-cli media upload ./Music --json | jq '.results[] | select(.success == false)'
```

## Media list

```bash
media-cli media list
media-cli media list --station "Accra Radio"
media-cli media list --folder "High Rotation"
media-cli media list --search "afrobeats"
media-cli media list --page 2 --per-page 100
media-cli media list --search "station id" --json
```

Search is performed client-side on the fetched page.

## Directory upload details

- The API has no dedicated directory-upload endpoint.
- The CLI recursively finds local files in the specified directories.
- Embedded metadata is read before upload when available. Supported tags include ID3v1/ID3v2, MP4, FLAC, and OGG metadata.
- Interactive plans show title, artist, album, and track information. Single-file uploads also show year, genre, and tag format.
- Missing or malformed metadata never blocks an upload; the CLI falls back to the filename.
- JSON upload results include a `metadata` object for tagged files.
- Each file is uploaded through a separate multipart API request.
- Top-level directories map to remote folders by their basename.
- Nested local directories are flattened into the mapped top-level remote folder.
- For example, `Music/album-one/track.mp3` and `Music/album-two/track.mp3` both upload to the remote `Music` folder.
- Duplicate destination filenames in the same folder are rejected by default.
- Use `--allow-name-collisions` to override this protection.
- Use `--create-folders` to automatically create missing remote folders.
- Requests are throttled to 60 per minute (shared rate limiter).
- All uploads target one explicitly resolved station.

## Troubleshooting

### Missing API key

```text
No CLI API key is configured.

Run:
  media-cli login
```

### Invalid or revoked key

```text
The CLI API key is invalid, expired, or has been revoked.

Generate a new key in Account Settings → CLI API keys.
```

### Missing default station

```text
No destination station is configured.

Run:
  media-cli stations use <uuid-or-name>

Or provide:
  --station <uuid-or-name>
```

### Ambiguous station name

Narrow the match by using a full UUID, a unique UUID prefix, or a more specific name fragment.

### Missing folder

Create it:

```bash
media-cli folders create "Folder Name" --station "Station Name"
```

Or upload with `--create-folders`.
