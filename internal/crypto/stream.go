package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	streamMagic = "PUS1" // Pin Uploader Stream v1

	DefaultChunkSize = 32 * 1024

	// MaxChunkPlaintextSize bounds memory usage and allows early reject of bogus lengths.
	// Both client and server must stay within this limit.
	MaxChunkPlaintextSize = 64 * 1024
)

var (
	ErrInvalidStream = errors.New("invalid encrypted stream")
)

// Encrypt encrypts plaintext into a chunked AES-256-GCM stream.
//
// Format:
//
//	magic(4) || baseNonce(12) || [ u32 chunkLen || gcm(chunk) ]* || u32(0)
func Encrypt(key, plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := EncryptStreamWithChunkSize(key, &buf, bytes.NewReader(plaintext), DefaultChunkSize); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decrypt decrypts the chunked AES-256-GCM stream produced by Encrypt/EncryptStream*.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	r, err := NewDecryptReader(key, bytes.NewReader(ciphertext))
	if err != nil {
		return nil, err
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// EncryptStream encrypts src and writes ciphertext to dst using DefaultChunkSize.
func EncryptStream(key []byte, dst io.Writer, src io.Reader) error {
	return EncryptStreamWithChunkSize(key, dst, src, DefaultChunkSize)
}

// EncryptStreamWithChunkSize encrypts src and writes ciphertext to dst using chunkSize.
// chunkSize is capped to MaxChunkPlaintextSize.
func EncryptStreamWithChunkSize(key []byte, dst io.Writer, src io.Reader, chunkSize int) error {
	if chunkSize <= 0 {
		return fmt.Errorf("invalid chunk size: %d", chunkSize)
	}
	if chunkSize > MaxChunkPlaintextSize {
		chunkSize = MaxChunkPlaintextSize
	}

	gcm, err := newGCM(key)
	if err != nil {
		return err
	}
	baseNonce, err := newNonce(gcm)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(dst, streamMagic); err != nil {
		return fmt.Errorf("write magic: %w", err)
	}
	if _, err := dst.Write(baseNonce); err != nil {
		return fmt.Errorf("write nonce: %w", err)
	}

	buf := make([]byte, chunkSize)
	var chunkIndex uint64
	var lenPrefix [4]byte

	for {
		n, readErr := io.ReadFull(src, buf)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			if !errors.Is(readErr, io.ErrUnexpectedEOF) {
				return fmt.Errorf("read plaintext: %w", readErr)
			}
			// n contains remaining bytes, process as last chunk.
		}

		if n > MaxChunkPlaintextSize {
			return fmt.Errorf("chunk too large: %d", n)
		}
		binary.BigEndian.PutUint32(lenPrefix[:], uint32(n))
		if _, err := dst.Write(lenPrefix[:]); err != nil {
			return fmt.Errorf("write chunk len: %w", err)
		}

		nonce, err := deriveNonce(baseNonce, chunkIndex)
		if err != nil {
			return err
		}
		chunkIndex++

		ct := gcm.Seal(nil, nonce, buf[:n], nil)
		if _, err := dst.Write(ct); err != nil {
			return fmt.Errorf("write chunk: %w", err)
		}

		if errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
	}

	binary.BigEndian.PutUint32(lenPrefix[:], 0)
	if _, err := dst.Write(lenPrefix[:]); err != nil {
		return fmt.Errorf("write eof marker: %w", err)
	}

	return nil
}

// NewDecryptReader returns an io.Reader that decrypts from src on the fly.
func NewDecryptReader(key []byte, src io.Reader) (io.Reader, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	baseNonce := make([]byte, gcm.NonceSize())

	var hdr [4]byte
	if _, err := io.ReadFull(src, hdr[:]); err != nil {
		return nil, fmt.Errorf("%w: read magic: %w", ErrInvalidStream, err)
	}
	if string(hdr[:]) != streamMagic {
		return nil, fmt.Errorf("%w: bad magic", ErrInvalidStream)
	}

	if _, err := io.ReadFull(src, baseNonce); err != nil {
		return nil, fmt.Errorf("%w: read nonce: %w", ErrInvalidStream, err)
	}

	return &decryptReader{
		src:       src,
		gcm:       gcm,
		baseNonce: baseNonce,
	}, nil
}

type decryptReader struct {
	src io.Reader

	gcm       cipher.AEAD
	baseNonce []byte

	chunkIndex uint64

	buf []byte
	pos int
	eof bool
}

func (d *decryptReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		if d.pos < len(d.buf) {
			n := copy(p, d.buf[d.pos:])
			d.pos += n
			if d.pos == len(d.buf) {
				d.buf = nil
				d.pos = 0
			}
			return n, nil
		}
		if d.eof {
			return 0, io.EOF
		}

		var lenPrefix [4]byte
		if _, err := io.ReadFull(d.src, lenPrefix[:]); err != nil {
			return 0, fmt.Errorf("%w: read chunk len: %w", ErrInvalidStream, err)
		}
		chunkLen := int(binary.BigEndian.Uint32(lenPrefix[:]))
		if chunkLen == 0 {
			d.eof = true
			continue
		}
		if chunkLen < 0 || chunkLen > MaxChunkPlaintextSize {
			return 0, fmt.Errorf("%w: invalid chunk size %d", ErrInvalidStream, chunkLen)
		}

		ct := make([]byte, chunkLen+d.gcm.Overhead())
		if _, err := io.ReadFull(d.src, ct); err != nil {
			return 0, fmt.Errorf("%w: read chunk: %w", ErrInvalidStream, err)
		}

		nonce, err := deriveNonce(d.baseNonce, d.chunkIndex)
		if err != nil {
			return 0, err
		}
		d.chunkIndex++

		pt, err := d.gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			return 0, fmt.Errorf("%w: decrypt: %w", ErrInvalidStream, err)
		}

		d.buf = pt
		d.pos = 0
	}
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	if gcm.NonceSize() != 12 {
		return nil, fmt.Errorf("unexpected gcm nonce size: %d", gcm.NonceSize())
	}
	return gcm, nil
}

func deriveNonce(baseNonce []byte, chunkIndex uint64) ([]byte, error) {
	if len(baseNonce) != 12 {
		return nil, fmt.Errorf("%w: bad nonce size", ErrInvalidStream)
	}

	nonce := make([]byte, 12)
	copy(nonce, baseNonce)

	baseCounter := binary.BigEndian.Uint64(nonce[4:12])
	if baseCounter > ^uint64(0)-chunkIndex {
		return nil, fmt.Errorf("%w: nonce counter overflow", ErrInvalidStream)
	}
	binary.BigEndian.PutUint64(nonce[4:12], baseCounter+chunkIndex)
	return nonce, nil
}

func newNonce(gcm cipher.AEAD) ([]byte, error) {
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return nonce, nil
}
