# Radioplatform Media CLI

Manage your Radio Platform station media library from the command line.

## Installation

### From source

```bash
go install radioplatform-media-ci/cmd/rpmedia-cli@latest
```

### From GitHub Releases

**Linux (amd64):**

```bash
tar -xzf rpmedia-cli_Linux_amd64.tar.gz
sudo install -m 0755 rpmedia-cli /usr/local/bin/rpmedia-cli
```

**Linux (arm64):**

```bash
tar -xzf rpmedia-cli_Linux_arm64.tar.gz
sudo install -m 0755 rpmedia-cli /usr/local/bin/rpmedia-cli
```

**macOS (arm64):**

```bash
tar -xzf rpmedia-cli_Darwin_arm64.tar.gz
sudo install -m 0755 rpmedia-cli /usr/local/bin/rpmedia-cli
```

**macOS (amd64):**

```bash
tar -xzf rpmedia-cli_Darwin_amd64.tar.gz
sudo install -m 0755 rpmedia-cli /usr/local/bin/rpmedia-cli
```

Windows is not supported.

## Configuration

- The API server URL is hardcoded into the binary and cannot be overridden.
- CLI credentials are stored in `~/.config/rpmedia-cli/config.json`.
- If `XDG_CONFIG_HOME` is set, the path becomes `$XDG_CONFIG_HOME/rpmedia-cli/config.json`.
- The config directory is created with `0700` permissions.
- The config file is created with `0600` permissions.
- The `RADIO_PLATFORM_CLI_KEY` environment variable temporarily overrides the stored key.
- The CLI never creates API keys. Generate one in **Account Settings → CLI API keys**.

## Getting started

```console
$ rpmedia-cli login
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
rpmedia-cli stations list
```

Set the default station:

```bash
rpmedia-cli stations use "Accra Radio"
```

Override the station for a single command:

```bash
rpmedia-cli media upload song.mp3 --station "Kumasi FM"
```

**Important rules:**

- Commands use the persisted default station when no override is supplied.
- `--station` changes only the current command. It does not update the persisted default.
- The destination station is always displayed before uploads begin.
- The CLI never chooses the first station automatically.

## Folder workflows

List folders:

```bash
rpmedia-cli folders list
```

Create a folder:

```bash
rpmedia-cli folders create "High Rotation"
```

Create a folder on a specific station:

```bash
rpmedia-cli folders create "Jingles" --station 2f71a6cb
```

## Media upload

Upload one file to the default station's media root:

```bash
rpmedia-cli media upload song.mp3
```

Upload to another station:

```bash
rpmedia-cli media upload song.mp3 --station "Kumasi FM"
```

Upload multiple files:

```bash
rpmedia-cli media upload song1.mp3 song2.mp3
```

Upload with a glob pattern:

```bash
rpmedia-cli media upload "./tracks/*.mp3" --folder "High Rotation"
```

Upload a directory recursively:

```bash
rpmedia-cli media upload ./New-Releases
```

This maps the local directory `New-Releases` to the remote folder `New-Releases` on the selected station.

Upload a directory into a specific folder:

```bash
rpmedia-cli media upload ./Music --folder "High Rotation"
```

Upload multiple directories into matching remote folders:

```bash
rpmedia-cli media upload ./Music ./Jingles --create-folders --yes
```

Upload all files as jingles:

```bash
rpmedia-cli media upload ./Jingles --jingle
```

Non-interactive batch upload:

```bash
rpmedia-cli media upload ./Music \
  --station "Accra Radio" \
  --create-folders \
  --yes \
  --json
```

Process JSON results with jq:

```bash
rpmedia-cli media upload ./Music --json | jq '.results[] | select(.success == false)'
```

## Media list

```bash
rpmedia-cli media list
rpmedia-cli media list --station "Accra Radio"
rpmedia-cli media list --folder "High Rotation"
rpmedia-cli media list --search "afrobeats"
rpmedia-cli media list --page 2 --per-page 100
rpmedia-cli media list --search "station id" --json
```

Search is performed client-side on the fetched page.

## Directory upload details

- The API has no dedicated directory-upload endpoint.
- The CLI recursively finds local files in the specified directories.
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
  rpmedia-cli login
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
  rpmedia-cli stations use <uuid-or-name>

Or provide:
  --station <uuid-or-name>
```

### Ambiguous station name

Narrow the match by using a full UUID, a unique UUID prefix, or a more specific name fragment.

### Missing folder

Create it:

```bash
rpmedia-cli folders create "Folder Name" --station "Station Name"
```

Or upload with `--create-folders`.
