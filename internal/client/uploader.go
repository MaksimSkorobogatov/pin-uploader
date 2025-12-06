package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"go.uber.org/zap"
	"golang.org/x/image/draw"

	appcrypto "github.com/MaksimSkorobogatov/pin-uploader/internal/crypto"
)

// UploadResult represents a single upload outcome.
type UploadResult struct {
	Path  string
	Error error
	Meta  *UploadResponse
}

// UploadResponse mirrors the server response.
type UploadResponse struct {
	ID          int64    `json:"id"`
	GUID        string   `json:"guid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// UploadPayload is encrypted and sent to server.
type UploadPayload struct {
	Filename   string  `json:"filename"`
	DataBase64 string  `json:"data"`
	PinLink    *string `json:"pin_link"`
}

const (
	resizeThresholdPx = 2200
	targetLongestPx   = 2100
	jpegQuality       = 94
)

// Uploader handles encrypted uploads.
type Uploader struct {
	serverURL string
	key       []byte
	client    *http.Client
	logger    *zap.Logger
}

// NewUploader creates a new uploader instance.
func NewUploader(serverURL string, key []byte, logger *zap.Logger) *Uploader {
	return &Uploader{
		serverURL: trimTrailingSlash(serverURL),
		key:       key,
		logger:    logger,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ExpandGlobs expands glob patterns and deduplicates paths.
func ExpandGlobs(patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var paths []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", p, err)
		}
		if len(matches) == 0 {
			matches = []string{p} // treat as literal path
		}
		for _, m := range matches {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			paths = append(paths, m)
		}
	}
	return paths, nil
}

// UploadFile uploads a single file.
func (u *Uploader) UploadFile(ctx context.Context, path string, pinLink *string) UploadResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("read file: %w", err)}
	}

	resized, err := resizeImageIfNeeded(data)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("resize image: %w", err)}
	}
	data = resized.Data
	if resized.Resized {
		u.logger.Info("resized image before upload",
			zap.String("path", path),
			zap.Int("longest_before", resized.Before),
			zap.Int("longest_after", resized.After))
	}

	payload := UploadPayload{
		Filename:   filepath.Base(path),
		DataBase64: base64.StdEncoding.EncodeToString(data),
		PinLink:    pinLink,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("encode payload: %w", err)}
	}

	cipher, err := appcrypto.Encrypt(u.key, raw)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("encrypt: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.serverURL+"/rss/upload", bytes.NewReader(cipher))
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.client.Do(req)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("send request: %w", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return UploadResult{Path: path, Error: fmt.Errorf("server returned %s: %s", resp.Status, string(body))}
	}

	var meta UploadResponse
	if err := json.Unmarshal(body, &meta); err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("decode response: %w", err)}
	}

	return UploadResult{Path: path, Meta: &meta}
}

func trimTrailingSlash(url string) string {
	return strings.TrimRight(url, "/")
}

type resizeResult struct {
	Data    []byte
	Resized bool
	Before  int
	After   int
}

func resizeImageIfNeeded(data []byte) (resizeResult, error) {
	orientation := extractOrientation(data)

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return resizeResult{}, fmt.Errorf("decode: %w", err)
	}
	img = applyOrientation(img, orientation)

	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()
	longest := maxInt(width, height)

	result := resizeResult{
		Data:   data,
		Before: longest,
		After:  longest,
	}

	if longest <= resizeThresholdPx {
		return result, nil
	}

	scale := float64(targetLongestPx) / float64(longest)
	newW := int(math.Round(float64(width) * scale))
	newH := int(math.Round(float64(height) * scale))
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality})
	case "png":
		err = png.Encode(&buf, dst)
	case "gif":
		err = gif.Encode(&buf, dst, nil)
	default:
		return resizeResult{}, fmt.Errorf("unsupported image format for resize: %s", format)
	}
	if err != nil {
		return resizeResult{}, fmt.Errorf("encode %s: %w", format, err)
	}

	result.Data = buf.Bytes()
	result.Resized = true
	result.After = maxInt(newW, newH)
	return result, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func extractOrientation(data []byte) int {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	val, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return val
}

func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipHorizontal(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipVertical(img)
	case 5:
		return rotate90(flipHorizontal(img))
	case 6:
		return rotate90(img)
	case 7:
		return rotate270(flipHorizontal(img))
	case 8:
		return rotate270(img)
	default:
		return img
	}
}

func flipHorizontal(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func flipVertical(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, h-1-y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate90(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate180(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate270(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
