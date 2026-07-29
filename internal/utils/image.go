package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"

	"immo-lux/internal/server_error"
)

const (
	MaxImageWidth  = 1920
	MaxImageHeight = 1920
	JpegQuality    = 85
)

type ProcessedImage struct {
	Data        []byte
	ContentType string
	Width       int
	Height      int
	FileSize    int
}

func ProcessImage(data []byte, contentType string) (*ProcessedImage, error) {
	var img image.Image
	var err error

	reader := bytes.NewReader(data)

	switch contentType {
	case "image/jpeg", "image/jpg":
		img, err = jpeg.Decode(reader)
		if err != nil {
			return nil, server_error.Wrap("IMAGE_DECODE", "failed to decode JPEG image", err)
		}
	case "image/png":
		img, err = png.Decode(reader)
		if err != nil {
			return nil, server_error.Wrap("IMAGE_DECODE", "failed to decode PNG image", err)
		}
	case "image/webp":
		img, err = webp.Decode(reader)
		if err != nil {
			return nil, server_error.Wrap("IMAGE_DECODE", "failed to decode WebP image", err)
		}
	case "image/avif", "image/heic", "image/heif":
		return nil, server_error.Invalid("IMAGE_FORMAT_NOT_SUPPORTED",
			"AVIF/HEIC/HEIF formats require native libraries. Please convert to JPEG, PNG, WebP, or TIFF before uploading")
	case "image/tiff":
		img, err = tiff.Decode(reader)
		if err != nil {
			return nil, server_error.Wrap("IMAGE_DECODE", "failed to decode TIFF image", err)
		}
	case "image/bmp":
		img, err = bmp.Decode(reader)
		if err != nil {
			return nil, server_error.Wrap("IMAGE_DECODE", "failed to decode BMP image", err)
		}
	default:
		return nil, server_error.Invalid("IMAGE_UNSUPPORTED",
			fmt.Sprintf("unsupported image format: %s. Supported formats: JPEG, PNG, WebP, TIFF, BMP", contentType))
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	needsResize := width > MaxImageWidth || height > MaxImageHeight

	var finalImg image.Image
	var finalWidth, finalHeight int

	if needsResize {
		finalWidth, finalHeight = calculateNewDimensions(width, height, MaxImageWidth, MaxImageHeight)
		finalImg = resizeImage(img, finalWidth, finalHeight)
	} else {
		finalImg = img
		finalWidth = width
		finalHeight = height
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, finalImg, &jpeg.Options{Quality: JpegQuality})
	if err != nil {
		return nil, server_error.Wrap("IMAGE_ENCODE", "failed to encode image as JPEG", err)
	}

	processedData := buf.Bytes()

	return &ProcessedImage{
		Data:        processedData,
		ContentType: "image/jpeg",
		Width:       finalWidth,
		Height:      finalHeight,
		FileSize:    len(processedData),
	}, nil
}

func calculateNewDimensions(width, height, maxWidth, maxHeight int) (int, int) {
	aspectRatio := float64(width) / float64(height)

	newWidth := width
	newHeight := height

	if width > maxWidth {
		newWidth = maxWidth
		newHeight = int(float64(newWidth) / aspectRatio)
	}

	if newHeight > maxHeight {
		newHeight = maxHeight
		newWidth = int(float64(newHeight) * aspectRatio)
	}

	return newWidth, newHeight
}

func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func ReadImageFromReader(reader io.Reader, maxSize int64) ([]byte, string, error) {
	limitedReader := io.LimitReader(reader, maxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", server_error.Wrap("IMAGE_READ", "failed to read image data", err)
	}

	if len(data) == 0 {
		return nil, "", server_error.Invalid("IMAGE_EMPTY", "image data is empty")
	}

	contentType := detectContentType(data)
	if contentType == "" {
		return nil, "", server_error.Invalid("IMAGE_INVALID", "invalid image format")
	}

	return data, contentType, nil
}

func detectContentType(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 &&
		data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	// BMP: 42 4D (BM)
	if data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}

	// TIFF: 49 49 2A 00 (little-endian) or 4D 4D 00 2A (big-endian)
	if (data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00) ||
		(data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A) {
		return "image/tiff"
	}

	// HEIF/HEIC and AVIF all use ISO Base Media File Format (ftyp at offset 4)
	if len(data) >= 12 &&
		data[4] == 0x66 && data[5] == 0x74 && data[6] == 0x79 && data[7] == 0x70 {

		// HEIC: ftyp heic, heix, hevc, hevx (detected but not fully supported without native libs)
		if data[8] == 0x68 && data[9] == 0x65 && data[10] == 0x69 {
			if data[11] == 0x63 || data[11] == 0x78 {
				return "image/heic"
			}
		}
		if data[8] == 0x68 && data[9] == 0x65 && data[10] == 0x76 {
			if data[11] == 0x63 || data[11] == 0x78 {
				return "image/heic"
			}
		}

		// HEIF: ftyp mif1, msf1 (detected but not fully supported without native libs)
		if (data[8] == 0x6D && data[9] == 0x69 && data[10] == 0x66 && data[11] == 0x31) ||
			(data[8] == 0x6D && data[9] == 0x73 && data[10] == 0x66 && data[11] == 0x31) {
			return "image/heif"
		}

		// AVIF: ftyp avif, avis
		if data[8] == 0x61 && data[9] == 0x76 && data[10] == 0x69 {
			if data[11] == 0x66 || data[11] == 0x73 {
				return "image/avif"
			}
		}
	}

	return ""
}
