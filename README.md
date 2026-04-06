# jamfschool2snipeIT

`jamfschool2snipeIT` syncs devices from [JAMF School](https://www.jamf.com/products/jamf-school/) into [Snipe-IT](https://snipeitapp.com/) using Go on both sides: [`jamfschool-go-sdk`](https://github.com/Jamf-Concepts/jamfschool-go-sdk) for JAMF School and [`go-snipeit`](https://github.com/michellepellon/go-snipeit) for Snipe-IT.

The project is intentionally modeled after [`axm2snipe`](https://github.com/CampusTech/axm2snipe): a small Go CLI with YAML config, dry-run support, auto-created Snipe-IT models, and serial-number-based create/update behavior.

## Features

- Sync all JAMF School devices into Snipe-IT assets
- Match existing Snipe-IT assets by exact serial number
- Auto-create Snipe-IT models from JAMF School model name and identifier
- Dry-run mode that blocks writes to Snipe-IT
- Update-only mode to avoid creating new assets or models
- Create or update a reusable Snipe-IT custom fieldset mapping with `setup`
- Sync JAMF School locations into Snipe-IT locations with optional auto-create
- Optional beta user-assignment heuristics backed by JAMF School and Snipe-IT user directories
- Managed notes blocks that preserve manual notes while keeping sync metadata fresh
- Release packaging via `goreleaser`

## Commands

```bash
jamfschool2snipeIT test
jamfschool2snipeIT setup -v
jamfschool2snipeIT sync --dry-run -v
jamfschool2snipeIT sync --serial C02X1234
jamfschool2snipeIT sync --device-type computer --device-type tablet
```

## Configuration

Copy the example config:

```bash
cp settings.example.yaml settings.yaml
```

Environment variables can override the core secrets:

- `JAMFSCHOOL_URL`
- `JAMFSCHOOL_NETWORK_ID`
- `JAMFSCHOOL_API_KEY`
- `SNIPEIT_URL`
- `SNIPEIT_API_KEY`

## Security

### Credentials

The `settings.yaml` file may contain API keys in plaintext. Never commit it to version control; it is ignored by `.gitignore` for this reason.

For production and CI use, prefer environment variables over the YAML file:

| Variable | Overrides |
| --- | --- |
| `JAMFSCHOOL_URL` | `jamf_school.url` |
| `JAMFSCHOOL_NETWORK_ID` | `jamf_school.network_id` |
| `JAMFSCHOOL_API_KEY` | `jamf_school.api_key` |
| `SNIPEIT_URL` | `snipe_it.url` |
| `SNIPEIT_API_KEY` | `snipe_it.api_key` |

In CI pipelines, inject these as masked secret environment variables rather than checking in a `settings.yaml`.

### HTTPS Enforcement

Both `jamf_school.url` and `snipe_it.url` must use `https://`. The tool refuses to start if either is configured with plain HTTP, preventing accidental credential exposure.

### Beta User Assignment

The `beta_user_assignment` feature is heuristic and intentionally requires two opt-in flags: `beta: true` and `enabled: true`. Every match and non-match decision is logged at WARN level so you can audit what the tool did after a sync.

## Field Mapping Sources

These source keys are supported under `sync.field_mapping`:

- `serial_number`
- `asset_tag`
- `name`
- `udid`
- `notes`
- `last_checkin`
- `model_name`
- `model_identifier`
- `model_type`
- `os_name`
- `os_version`
- `device_enroll_type`
- `battery_level`
- `total_capacity`
- `location_id`
- `location_name`
- `managed`
- `supervised`
- `matched_user`

## User Assignment

JAMF School device records in this SDK do not expose a direct assigned-user field, so user assignment is intentionally treated as a beta feature. When enabled, the tool extracts identifiers from configured device fields, defaults to `notes`, confirms them against the JAMF School user directory if `require_jamf_user` is enabled, and then matches them against Snipe-IT users by email, username, or both.

To enable it, both of these must be set:

```yaml
sync:
  beta_user_assignment:
    beta: true
    enabled: true
```

If `enabled: true` is set without `beta: true`, the tool will stop with a validation error instead of quietly turning on heuristic assignment. The default beta source is now just `notes` to reduce accidental matches from generic device names.

## Release

```bash
make test
make build
make snapshot
```
