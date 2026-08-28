package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// GenerateIconPNG creates a 32x32 icon with a 'W' on top of 'D' stylized glyph and border color.
// Status colors:
// Green: #22C55E (success)
// Blue:  #3B82F6 (running)
// Red:   #EF4444 (error0 - error4)
// Gray:  #6B7280 (stopped / idle)
func GenerateIconPNG(status string) []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	var primaryColor color.RGBA
	switch status {
	case "success":
		primaryColor = color.RGBA{R: 34, G: 197, B: 94, A: 255} // Green
	case "running":
		primaryColor = color.RGBA{R: 59, G: 130, B: 246, A: 255} // Blue
	case "error0", "error1", "error2", "error3", "error4":
		primaryColor = color.RGBA{R: 239, G: 68, B: 68, A: 255} // Red
	default:
		primaryColor = color.RGBA{R: 107, G: 114, B: 128, A: 255} // Gray
	}

	// Draw rounded square background with status color
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			// Circular rounded shape
			dx := float64(x - size/2)
			dy := float64(y - size/2)
			if dx*dx+dy*dy <= float64((size/2-2)*(size/2-2)) {
				img.Set(x, y, primaryColor)
			}
		}
	}

	// Draw white inner 'W' glyph approximation
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	// Vertical / diagonal strokes for "W"
	for y := 8; y <= 22; y++ {
		img.Set(8, y, white)
		img.Set(9, y, white)
		img.Set(22, y, white)
		img.Set(23, y, white)
	}
	for i := 0; i <= 6; i++ {
		img.Set(9+i, 22-i, white)
		img.Set(10+i, 22-i, white)
		img.Set(22-i, 22-i, white)
		img.Set(21-i, 22-i, white)
	}

	buf := new(bytes.Buffer)
	_ = png.Encode(buf, img)
	return buf.Bytes()
}

// GenerateIconICO wraps PNG data into basic Windows ICO container if required.
func GenerateIconICO(status string) []byte {
	pngBytes := GenerateIconPNG(status)
	icoBuf := new(bytes.Buffer)

	// ICONDIR Header (6 bytes)
	icoBuf.Write([]byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00})

	// ICONDIRENTRY (16 bytes)
	size := len(pngBytes)
	icoBuf.WriteByte(32) // Width
	icoBuf.WriteByte(32) // Height
	icoBuf.WriteByte(0)  // Color count
	icoBuf.WriteByte(0)  // Reserved
	icoBuf.Write([]byte{0x01, 0x00}) // Color planes
	icoBuf.Write([]byte{0x20, 0x00}) // Bits per pixel (32)
	// Image size (4 bytes)
	icoBuf.WriteByte(byte(size & 0xFF))
	icoBuf.WriteByte(byte((size >> 8) & 0xFF))
	icoBuf.WriteByte(byte((size >> 16) & 0xFF))
	icoBuf.WriteByte(byte((size >> 24) & 0xFF))
	// Image offset (4 bytes: 6 + 16 = 22)
	icoBuf.Write([]byte{22, 0, 0, 0})

	// Write PNG body
	icoBuf.Write(pngBytes)
	return icoBuf.Bytes()
}
