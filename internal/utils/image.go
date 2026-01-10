package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"

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
	default:
		return nil, server_error.New("IMAGE_UNSUPPORTED", "unsupported image format, only JPEG and PNG are supported")
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
		return nil, "", server_error.New("IMAGE_EMPTY", "image data is empty")
	}

	contentType := detectContentType(data)
	if contentType == "" {
		return nil, "", server_error.New("IMAGE_INVALID", "invalid image format")
	}

	return data, contentType, nil
}

func detectContentType(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	return ""
}
