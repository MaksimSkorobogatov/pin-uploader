I need you to develop a complete client-server application in Go with the following specifications:

## Project Overview
Create an RSS feed generator service for Pinterest that enriches images with AI-generated metadata, plus a CLI client for uploading images.

## Architecture

### Server Component
- **Purpose**: RSS 2.0 feed generator for Pinterest pins
- **Core Features**:
  - Accept image uploads from clients
  - Generate titles, descriptions, and tags for images using Gemma-3-27b LLM via Blackbox API
  - Store RSS feed items in SQLite database
  - Serve RSS 2.0 feed at an HTTP endpoint (e.g., `/rss`)
  - Implement end-to-end encryption for client-server communication using a static shared key

### Client Component
- **Purpose**: Minimalist CLI tool for uploading images
- **Core Features**:
  - Accept single or multiple image file paths (support glob patterns like `*.jpg`, `images/*.png`)
  - Encrypt and send images to the server
  - Display upload status and results

## Technical Requirements

### Language & Libraries
- **Language**: Go
- **Required Libraries**:
  - `langchaingo` - for LLM API integration with Blackbox
  - `go.uber.org/zap` - for structured logging
  - Choose an appropriate SQLite library (e.g., `modernc.org/sqlite` or `github.com/mattn/go-sqlite3`)
  - Standard library for HTTP, crypto, file operations

### Security
- Implement end-to-end encryption between client and server using a static symmetric key
- The encryption key must be configurable in both client and server config files
- Do NOT use SSL/TLS - implement custom encryption layer

### Configuration
- **Server config** (YAML or JSON):
  - Server port
  - Encryption key
  - Blackbox API credentials
  - SQLite database path
  - LLM model parameters

- **Client config** (YAML or JSON):
  - Server address
  - Encryption key

### Database Schema
Design SQLite schema for storing:
- Image metadata (filename, upload timestamp)
- Generated title, description, tags
- RSS feed item data (guid, pubDate, etc.)

### RSS Feed Format
- Valid RSS 2.0 format
- Each item should include:
  - Title (AI-generated)
  - Description (AI-generated)
  - Tags as categories
  - Image enclosure
  - Publication date
  - Unique GUID

## Implementation Guidelines

1. **Project Structure**:
    ```
    project/
    ├── cmd/
    │   ├── server/
    │   │   └── main.go
    │   └── client/
    │       └── main.go
    ├── internal/
    │   ├── server/
    │   │   ├── handler.go
    │   │   ├── rss.go
    │   │   ├── llm.go
    │   │   └── storage.go
    │   ├── client/
    │   │   └── uploader.go
    │   ├── crypto/
    │   │   └── encryption.go
    │   └── config/
    │       └── config.go
    ├── configs/
    │   ├── server.yaml
    │   └── client.yaml
    ├── go.mod
    └── README.md
    ```

2. **Error Handling**: Implement comprehensive error handling with zap logging

3. **Graceful Shutdown**: Server should handle SIGINT/SIGTERM gracefully

4. **CLI Interface**: Find a suitable library for client CLI arguments (it should can accept list of files as argument), or implement custom file list hadling using `os.Args`

## Deliverables

1. Complete, working Go code for both client and server
2. Configuration file templates
3. README.md with:
   - Setup instructions
   - How to generate/configure encryption key
   - Usage examples for client
   - How to access RSS feed
4. After implementation, test the system end-to-end:
   - Start the server
   - Upload images using the client with glob patterns
   - Verify RSS feed generation
   - Confirm encryption is working

## Testing Requirements
- No unit tests needed
- Perform manual integration testing after implementation
- Demonstrate successful image upload and RSS feed generation

## Additional Notes
- Use appropriate image formats (JPEG, PNG, etc.)
- Handle large images appropriately
- Implement reasonable timeouts for LLM API calls
- RSS feed should be accessible via simple HTTP GET request
- Consider pagination or limits for RSS feed items

Please implement this complete solution, ensuring all components work together seamlessly.
