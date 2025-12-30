package transport

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type UploadHeader struct {
	Filename string  `json:"filename"`
	PinLink  *string `json:"pin_link,omitempty"`
}

const (
	MaxHeaderBytes = 16 * 1024
	MaxImageBytes  = 5 << 20 // 5 MiB (mirrors previous server cap)
)

func NewUploadPlaintextReader(header UploadHeader, image []byte) (io.Reader, error) {
	hb, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}
	if len(hb) == 0 || len(hb) > MaxHeaderBytes {
		return nil, fmt.Errorf("header size %d out of bounds", len(hb))
	}
	if len(image) == 0 {
		return nil, fmt.Errorf("empty image payload")
	}
	if len(image) > MaxImageBytes {
		return nil, fmt.Errorf("image too large: %d bytes", len(image))
	}

	prefix := make([]byte, 4+len(hb)+8)
	binary.BigEndian.PutUint32(prefix[:4], uint32(len(hb)))
	copy(prefix[4:], hb)
	binary.BigEndian.PutUint64(prefix[4+len(hb):], uint64(len(image)))

	return io.MultiReader(
		bytesReader(prefix),
		bytesReader(image),
	), nil
}

func DecodeUploadPlaintext(r io.Reader) (UploadHeader, []byte, error) {
	var header UploadHeader

	var u32 [4]byte
	if _, err := io.ReadFull(r, u32[:]); err != nil {
		return header, nil, fmt.Errorf("read header len: %w", err)
	}
	headerLen := int(binary.BigEndian.Uint32(u32[:]))
	if headerLen <= 0 || headerLen > MaxHeaderBytes {
		return header, nil, fmt.Errorf("invalid header size: %d", headerLen)
	}

	hb := make([]byte, headerLen)
	if _, err := io.ReadFull(r, hb); err != nil {
		return header, nil, fmt.Errorf("read header: %w", err)
	}
	if err := json.Unmarshal(hb, &header); err != nil {
		return header, nil, fmt.Errorf("decode header: %w", err)
	}
	if header.Filename == "" {
		return header, nil, fmt.Errorf("missing filename")
	}

	var u64 [8]byte
	if _, err := io.ReadFull(r, u64[:]); err != nil {
		return header, nil, fmt.Errorf("read image len: %w", err)
	}
	imageLen := int(binary.BigEndian.Uint64(u64[:]))
	if imageLen <= 0 || imageLen > MaxImageBytes {
		return header, nil, fmt.Errorf("invalid image size: %d", imageLen)
	}

	image := make([]byte, imageLen)
	if _, err := io.ReadFull(r, image); err != nil {
		return header, nil, fmt.Errorf("read image: %w", err)
	}

	var extra [1]byte
	n, err := r.Read(extra[:])
	if n > 0 || (err != nil && err != io.EOF) {
		return header, nil, fmt.Errorf("trailing plaintext data")
	}

	return header, image, nil
}

// bytesReader avoids importing bytes in many places; it is small and inlineable.
func bytesReader(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b []byte
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if len(s.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.b)
	s.b = s.b[n:]
	return n, nil
}
