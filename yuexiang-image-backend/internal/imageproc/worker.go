package imageproc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Variant struct {
	Kind      string `json:"kind"`
	ObjectKey string `json:"object_key"`
	MimeType  string `json:"mime_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int64  `json:"bytes"`
}

type Result struct {
	Width          int                `json:"width"`
	Height         int                `json:"height"`
	PerceptualHash string             `json:"perceptual_hash"`
	Variants       []Variant          `json:"variants"`
	Engine         string             `json:"engine"`
	Generated      []GeneratedVariant `json:"-"`
}

type GeneratedVariant struct {
	Variant Variant
	Body    []byte
}

type LibvipsWorker struct {
	Binary string
}

func (w LibvipsWorker) Process(ctx context.Context, r io.Reader) (Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}, err
	}
	cfg, _, _ := image.DecodeConfig(bytesReader(data))
	result := Result{
		Width:          cfg.Width,
		Height:         cfg.Height,
		PerceptualHash: perceptualHash(data),
		Engine:         "libvips-worker",
	}
	if w.Binary == "" {
		w.Binary = "vipsthumbnail"
	}
	if _, err := exec.LookPath(w.Binary); err != nil {
		result.Engine = "libvips-unavailable"
		return result, nil
	}
	generated := w.generateVariants(ctx, data, cfg.Width, cfg.Height)
	result.Generated = generated
	for _, item := range generated {
		result.Variants = append(result.Variants, item.Variant)
	}
	return result, nil
}

func (w LibvipsWorker) generateVariants(ctx context.Context, data []byte, width, height int) []GeneratedVariant {
	tmpDir, err := os.MkdirTemp("", "yuexiang-vips-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	input := filepath.Join(tmpDir, "source")
	if err := os.WriteFile(input, data, 0600); err != nil {
		return nil
	}
	specs := []struct {
		kind     string
		mimeType string
		size     string
		filename string
		width    int
		height   int
	}{
		{kind: "thumbnail", mimeType: "image/webp", size: "640x", filename: "thumbnail.webp", width: min(width, 640), height: 0},
		{kind: "webp", mimeType: "image/webp", size: fmt.Sprintf("%dx", max(width, 1)), filename: "original.webp", width: width, height: height},
		{kind: "avif", mimeType: "image/avif", size: fmt.Sprintf("%dx", max(width, 1)), filename: "original.avif", width: width, height: height},
	}
	var out []GeneratedVariant
	for _, spec := range specs {
		output := filepath.Join(tmpDir, spec.filename)
		cmd := exec.CommandContext(ctx, w.Binary, input, "--size", spec.size, "-o", output)
		if err := cmd.Run(); err != nil {
			continue
		}
		body, err := os.ReadFile(output)
		if err != nil || len(body) == 0 {
			continue
		}
		out = append(out, GeneratedVariant{
			Variant: Variant{
				Kind:     spec.kind,
				MimeType: spec.mimeType,
				Width:    spec.width,
				Height:   spec.height,
				Bytes:    int64(len(body)),
			},
			Body: body,
		})
	}
	return out
}

func fastHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

func perceptualHash(data []byte) string {
	img, _, err := image.Decode(bytesReader(data))
	if err != nil {
		return fastHash(data)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return fastHash(data)
	}
	var values [64]uint64
	var total uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			px := bounds.Min.X + (x*width+width/2)/8
			py := bounds.Min.Y + (y*height+height/2)/8
			r, g, b, _ := img.At(px, py).RGBA()
			gray := (299*uint64(r) + 587*uint64(g) + 114*uint64(b)) / 1000
			values[y*8+x] = gray
			total += gray
		}
	}
	mean := total / 64
	var bits uint64
	for idx, value := range values {
		if value >= mean {
			bits |= 1 << uint(63-idx)
		}
	}
	return fmt.Sprintf("ahash:%016x", bits)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type byteSliceReader struct {
	data []byte
	pos  int
}

func bytesReader(data []byte) io.Reader {
	return &byteSliceReader{data: data}
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
