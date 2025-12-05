## Pin Uploader – AI-enriched Pinterest RSS

A small client/server toolchain to upload images, generate Pinterest-ready metadata with a Blackbox (Gemma-3-27b) LLM via `langchaingo`, and expose them through an RSS 2.0 feed. All client-server traffic is encrypted with a shared AES-256 key (no TLS required).

### Prerequisites
- Go 1.24.4+ (toolchain installed automatically via `go mod tidy`).
- Blackbox API key and base URL (OpenAI-compatible).
- Base64-encoded 32-byte symmetric key shared between client and server.

Generate a key:
```bash
head -c 32 /dev/urandom | base64
```

### Configuration
Templates live under `configs/`.

`configs/server.yaml`
```yaml
port: 8080
public_base_url: "http://localhost:8080"   # used for RSS links/enclosures
encryption_key: "BASE64_32_BYTE_KEY"
database_path: "./pins.db"
blackbox_api_key: "YOUR_BLACKBOX_KEY"
blackbox_base_url: "https://api.blackbox.ai/v1"
llm_model: "gemma-3-27b"
llm_temperature: 0.7
```

`configs/client.yaml`
```yaml
server_address: "http://localhost:8080"
encryption_key: "BASE64_32_BYTE_KEY"       # must match server
```

### Running the server
```bash
go run ./cmd/server --config configs/server.yaml
```
The server listens on `port`, handles `/upload`, `/rss`, and `/image/{id}` endpoints, and persists pins into SQLite.

### Uploading images from the client
```bash
go run ./cmd/client --config configs/client.yaml images/*.png
```
- Glob patterns are supported; unmatched patterns are treated as literal paths.
- The client encrypts a JSON payload (filename + base64 image data) with AES-GCM and posts to `/upload`.
- Responses echo back the generated metadata.

Example output:
```
✅ images/cat.png -> ID 1 | GUID f2e... | Title: Cozy Cat Corner
❌ missing.jpg: open missing.jpg: no such file or directory
```

### Accessing the RSS feed
After uploads, fetch the feed:
```bash
curl http://localhost:8080/rss
```
Each item includes title, description, categories (tags), and an enclosure pointing to `/image/{id}`.

### Notes on encryption & LLM
- Encryption uses AES-256-GCM with a shared key (nonce prepended to ciphertext).
- LLM calls go through `langchaingo`’s OpenAI client; configure `blackbox_base_url`/`blackbox_api_key` for your Blackbox endpoint. If the LLM call fails or returns invalid JSON, the server falls back to deterministic metadata and logs the issue.

### Manual testing checklist
1) Start the server with your config.
2) Run the client with a set of images (glob patterns supported).
3) Confirm uploads succeed and IDs/GUIDs are returned.
4) Fetch `/rss` and verify items appear with enclosures.
5) Open `/image/{id}` in a browser to view stored images and confirm they render.
