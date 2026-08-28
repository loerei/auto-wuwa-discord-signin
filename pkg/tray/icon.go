package tray

import (
	"bytes"
	_ "embed"
)

//go:embed assets/kurobot_green.png
var iconGreen []byte

//go:embed assets/kurobot_blue.png
var iconBlue []byte

//go:embed assets/kurobot_red.png
var iconRed []byte

//go:embed assets/kurobot_gray.png
var iconGray []byte

// GenerateIconPNG returns high-resolution 256x256 transparent Kurobot mascot PNG for the given status.
// Status colors:
// Green: #10B981 (success)
// Blue:  #3B82F6 (running)
// Red:   #EF4444 (error0 - error4)
// Gray:  #6B7280 (stopped / idle)
func GenerateIconPNG(status string) []byte {
	switch status {
	case "success":
		return iconGreen
	case "running":
		return iconBlue
	case "error0", "error1", "error2", "error3", "error4":
		return iconRed
	default:
		return iconGray
	}
}

// GenerateIconICO wraps high-resolution PNG data into Windows ICO container for system tray rendering.
func GenerateIconICO(status string) []byte {
	pngBytes := GenerateIconPNG(status)
	icoBuf := new(bytes.Buffer)

	// ICONDIR Header (6 bytes)
	icoBuf.Write([]byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00})

	// ICONDIRENTRY (16 bytes)
	// Width & Height: 0 specifies 256x256 in ICO format specification
	size := len(pngBytes)
	icoBuf.WriteByte(0) // Width (256)
	icoBuf.WriteByte(0) // Height (256)
	icoBuf.WriteByte(0) // Color count
	icoBuf.WriteByte(0) // Reserved
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
