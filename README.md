# Pin Uploader – AI Pinterest RSS

CLI + server that encrypts image uploads, generates Pinterest-ready titles/descriptions with an OpenAI-compatible model (tested with Gemma-3-27b), stores everything in SQLite, and serves an RSS 2.0 feed for Pinterest ingestion.

## What it does
- Client AES-256-GCM encrypts `{filename, base64 image}` payloads and POSTs them to `/upload` (no TLS required while keys stay private).
- Server decrypts, saves images to SQLite, and asks the LLM to build a JSON `{title, description}` using `internal/server/prompt_template.md` (4 KB preview to the model to keep prompts small). A deterministic fallback is used if the LLM fails.
- RSS 2.0 feed is exposed at `/rss`; images are downloadable at `/rss/image/{id}` and linked via enclosures.
- Prompt is configurable (`pin_description_language`), while tags are appended in English as part of the description.

## Prerequisites
- Go 1.24.4+.
- OpenAI-compatible API key + base URL.
- Base64-encoded 32-byte symmetric key shared by client and server.

Generate a key (or run `go run ./cmd/generate-strong-key`):
```bash
head -c 32 /dev/urandom | base64
```

## Configure
Config samples live in `configs/`.

`configs/server.yaml`
```yaml
port: 8080                                 # HTTP listen address
public_base_url: "http://localhost:8080"   # used for RSS links/enclosures
encryption_key: "BASE64_32_BYTE_KEY"       # must be 32 bytes when base64-decoded
database_path: "~/.local/share/pin-uploader/db.sqlite"
llm_api_key: "YOUR_LLM_KEY"
llm_base_url: "https://api.your-llm-provider.com/v1"
llm_model: "gemma-3-27b"
llm_temperature: 0.6
prompt_params:
  pin_description_language: "English" # change to generate descriptions in another language
  pin_title_language: "English"       # change to generate title in another language
# default pin link can contain, for example, a link to your telegram channel
default_pin_link:
```

`~/.config/pin-uploader/config.yaml` (created by `make configure`)
```yaml
server_address: "http://localhost:8080"
encryption_key: "BASE64_32_BYTE_KEY"       # must match the server key
```

## Run the server
```bash
go run ./cmd/server -config configs/server.yaml
```
- Endpoints: `POST /rss/upload`, `GET /rss`, `GET /rss/image/{id}`.
- SQLite schema is created automatically at `database_path` (supports `~` expansion).

## Upload from the client
Prepare the client config once:
```bash
make configure   # writes ~/.config/pin-uploader/config.yaml
```

Upload one or many images (globs allowed; unmatched globs are used as literal paths):
```bash
go run ./cmd/client -link https://www.instagram.com/p/CfJKeradf43fGeORA-N *.jpeg
```
Example:
```
✅ images/cat.jpeg -> ID 1 | GUID f2e... | Title: Cozy Cat Corner
❌ missing.jpeg: open missing.jpeg: no such file or directory
```

## RSS feed for Pinterest
After uploads:
```bash
curl http://localhost:8080/rss
```
Items include the generated title/description, GUID, pubDate, and an enclosure pointing to `/image/{id}` for Pinterest to ingest.

## Notes
- Encryption: AES-256-GCM; nonce is prepended to ciphertext (`nonce || ciphertext`). Use the same base64 key on both sides.
- LLM: OpenAI-compatible via `langchaingo` (`llm_base_url`, `llm_api_key`, `llm_model`, `llm_temperature`). Prompt lives in `internal/server/prompt_template.md`; update `pin_description_language` to change the description language while keeping tags in English.
- Preview size: only the first 4 KB of the image is sent to the LLM to keep prompts small; full image is stored and served via RSS/enclosures.
- Fallback: if the LLM call or JSON parsing fails, the server returns a simple deterministic title/description and logs the issue.
