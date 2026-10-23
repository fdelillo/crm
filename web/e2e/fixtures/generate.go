// T-F701: generate deterministic, fictional fixtures for the browser suite.
// Run from the repository root with `go run ./web/e2e/fixtures/generate.go`.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
)

const (
	jpegMinimumBytes = 4_500_000
	jpegMaximumBytes = 5_500_000
	pngBoundaryMin   = 1_900_000
	pngBoundaryMax   = 2_097_152
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	outputDir := filepath.Dir(mustSourcePath())
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}

	if err := writeJPEG(filepath.Join(outputDir, "orientation-6-gps.jpg")); err != nil {
		return err
	}
	for _, fixture := range []struct {
		name string
		img  image.Image
	}{
		{"transparent-3000.png", transparentPNG()},
		{"small-800x400.png", smallPNG()},
		{"boundary-2000px.png", boundaryPNG()},
	} {
		if err := writePNG(filepath.Join(outputDir, fixture.name), fixture.img); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(outputDir, "vector.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect width="64" height="64" fill="#52796f"/><circle cx="32" cy="32" r="18" fill="#cad2c5"/></svg>`), 0o644); err != nil {
		return fmt.Errorf("write SVG fixture: %w", err)
	}
	if err := writeGIF(filepath.Join(outputDir, "animated.gif")); err != nil {
		return err
	}
	return nil
}

func mustSourcePath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot locate fixture generator")
	}
	return file
}

func writeJPEG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 4000, 3000))
	random := rand.New(rand.NewSource(701))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 4000; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(random.Intn(256)), G: uint8(random.Intn(256)), B: uint8(random.Intn(256)), A: 255})
		}
	}

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 50}); err != nil {
		return fmt.Errorf("encode JPEG fixture: %w", err)
	}
	withEXIF, err := insertEXIF(encoded.Bytes())
	if err != nil {
		return err
	}
	if size := len(withEXIF); size < jpegMinimumBytes || size > jpegMaximumBytes {
		return fmt.Errorf("JPEG fixture is %d bytes; expected %d–%d", size, jpegMinimumBytes, jpegMaximumBytes)
	}
	if err := os.WriteFile(path, withEXIF, 0o644); err != nil {
		return fmt.Errorf("write JPEG fixture: %w", err)
	}
	return nil
}

func insertEXIF(jpegBytes []byte) ([]byte, error) {
	if len(jpegBytes) < 2 || jpegBytes[0] != 0xff || jpegBytes[1] != 0xd8 {
		return nil, errors.New("encoded JPEG has no SOI marker")
	}
	const (
		ifd0Offset   = 8
		gpsIFD       = 38
		gpsRationals = 92
	)
	var tiff bytes.Buffer
	tiff.Write([]byte{'I', 'I'})
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(42))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(ifd0Offset))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(2))
	writeEntry(&tiff, 0x0112, 3, 1, 6)
	writeEntry(&tiff, 0x8825, 4, 1, gpsIFD)
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(4))
	writeInlineBytes(&tiff, 0x0000, 4, []byte{2, 3, 0, 0})
	writeInlineBytes(&tiff, 0x0001, 2, []byte{'N', 0})
	writeEntry(&tiff, 0x0002, 5, 3, gpsRationals)
	writeInlineBytes(&tiff, 0x0003, 2, []byte{'E', 0})
	writeEntry(&tiff, 0x0004, 5, 3, gpsRationals+24)
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(0))
	for _, value := range []uint32{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1} {
		_ = binary.Write(&tiff, binary.LittleEndian, value)
	}

	app1 := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	if len(app1)+2 > 65_535 {
		return nil, errors.New("EXIF APP1 segment is too large")
	}
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(app1)+2))
	segment = append(segment, app1...)
	result := make([]byte, 0, len(jpegBytes)+len(segment))
	result = append(result, jpegBytes[:2]...)
	result = append(result, segment...)
	result = append(result, jpegBytes[2:]...)
	return result, nil
}

func writeEntry(dst *bytes.Buffer, tag, kind uint16, count, value uint32) {
	_ = binary.Write(dst, binary.LittleEndian, tag)
	_ = binary.Write(dst, binary.LittleEndian, kind)
	_ = binary.Write(dst, binary.LittleEndian, count)
	_ = binary.Write(dst, binary.LittleEndian, value)
}

func writeInlineBytes(dst *bytes.Buffer, tag, kind uint16, value []byte) {
	_ = binary.Write(dst, binary.LittleEndian, tag)
	_ = binary.Write(dst, binary.LittleEndian, kind)
	_ = binary.Write(dst, binary.LittleEndian, uint32(len(value)))
	field := make([]byte, 4)
	copy(field, value)
	_, _ = dst.Write(field)
}

func transparentPNG() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 3000, 3000))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 3000; x++ {
			if x < 1000 || (x >= 2000 && y >= 1500) {
				img.SetNRGBA(x, y, color.NRGBA{R: 68, G: 118, B: 108, A: 0})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: 82, G: 121, B: 111, A: 255})
			}
		}
	}
	return img
}

func smallPNG() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 800, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 800; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(40 + x%180), G: uint8(60 + y%150), B: 100, A: 255})
		}
	}
	return img
}

func boundaryPNG() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 800, 800))
	random := rand.New(rand.NewSource(702))
	for y := 0; y < 800; y++ {
		for x := 0; x < 800; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(random.Intn(256)), G: uint8(random.Intn(256)), B: uint8(random.Intn(256)), A: 255})
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if filepath.Base(path) == "boundary-2000px.png" && (encoded.Len() < pngBoundaryMin || encoded.Len() > pngBoundaryMax) {
		return fmt.Errorf("boundary PNG is %d bytes; expected %d–%d", encoded.Len(), pngBoundaryMin, pngBoundaryMax)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeGIF(path string) error {
	// A valid one-pixel GIF avoids third-party encoders for this rejected-format fixture.
	encoded, err := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
	if err != nil {
		return fmt.Errorf("decode GIF fixture: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write GIF fixture: %w", err)
	}
	return nil
}
