package soryu

import (
	"fmt"
	"image/color"
	"math/rand"
)

func RandomChannel() Channel {
	r := rand.Float32()
	if r < 0.33 {
		return Green
	} else if r < 0.66 {
		return Red
	}
	return Blue
}

func c(a uint32) uint8 {
	return uint8((float64(a) / MAXC) * 255)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid llength, must be 7 or 4")
	}
	return
}

func shiftColor(in color.Color, left int) (out color.RGBA) {
	var shiftedColor color.RGBA
	r, g, b, a := in.RGBA()

	shiftedColor = color.RGBA{
		R: uint8(b),
		G: uint8(r),
		B: uint8(g),
		A: uint8(a),
	}

	if left == 1 {
		shiftedColor = color.RGBA{
			R: uint8(g),
			G: uint8(b),
			B: uint8(r),
			A: uint8(a),
		}
	}

	return shiftedColor
}
