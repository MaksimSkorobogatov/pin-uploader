package client

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResizeImageIfNeeded_OrientationAndScale(t *testing.T) {
	cases := []struct {
		name           string
		file           string
		expectPortrait bool
	}{
		{name: "portrait", file: "test1.jpeg", expectPortrait: true},
		{name: "landscape", file: "test2.jpeg", expectPortrait: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, file, _, _ := runtime.Caller(0)
			imgPath := filepath.Join(filepath.Dir(file), tc.file)

			data, err := os.ReadFile(imgPath)
			if err != nil {
				t.Fatalf("read test image: %v", err)
			}

			res, err := resizeImageIfNeeded(data)
			if err != nil {
				t.Fatalf("resizeImageIfNeeded error: %v", err)
			}
			if !res.Resized {
				t.Fatalf("expected resize to occur")
			}
			if res.Before <= resizeThresholdPx {
				t.Fatalf("expected original longest side > %d, got %d", resizeThresholdPx, res.Before)
			}
			if res.After > targetLongestPx {
				t.Fatalf("expected resized longest side <= %d, got %d", targetLongestPx, res.After)
			}

			os.WriteFile(tc.name+"_.jpeg", res.Data, 0644)

			img, format, err := image.Decode(bytes.NewReader(res.Data))
			if err != nil {
				t.Fatalf("decode resized image: %v", err)
			}
			if format != "jpeg" {
				t.Fatalf("expected jpeg format, got %s", format)
			}
			b := img.Bounds()
			width := b.Dx()
			height := b.Dy()
			if tc.expectPortrait && width >= height {
				t.Fatalf("expected portrait orientation after resize, got %dx%d", width, height)
			}
			if !tc.expectPortrait && width <= height {
				t.Fatalf("expected landscape orientation after resize, got %dx%d", width, height)
			}
		})
	}
}
