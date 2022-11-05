package soryu

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/rand"
	"os"

	"github.com/StephaneBunel/bresenham"
	"github.com/anthonynsimon/bild/blend"
	"github.com/anthonynsimon/bild/noise"
	"github.com/lucasb-eyer/go-colorful"
	xdraw "golang.org/x/image/draw"
)

type Channel int

const (
	// Red is the red channel
	Red Channel = iota
	// Green is the green channel
	Green
	// Blue is the blue channel
	Blue
	// Alpha is the alpha channel
	Alpha

	MAXC = 1<<16 - 1
)

type Img struct {
	In      image.Image
	Out     draw.Image
	Bounds  image.Rectangle
	Imgtype string
}

type Images struct {
	In     image.Image
	Gifify []Img
}

func (i *Img) SetNewBounds(seed int64) {
	rand.Seed(seed)
}

func (i *Img) Seed(seed int64) {
	rand.Seed(seed)
}

func (i *Img) Copy() {
	bounds := i.Bounds
	draw.Draw(i.Out, bounds, i.In, bounds.Min, draw.Src)
}

func (i *Img) Write(out io.Writer) error {
	if i.Imgtype == "png" {
		return png.Encode(out, i.Out)
	}

	var opt jpeg.Options
	opt.Quality = 80

	return jpeg.Encode(out, i.Out, &opt)
}

func (i *Img) Streak(streaks, length int, left bool) {
	bounds := i.Bounds
	inBounds := i.In.Bounds()

	for streaks > 0 {
		x := bounds.Min.X + rand.Intn(bounds.Max.X-bounds.Min.X)
		y := bounds.Min.Y + rand.Intn(bounds.Max.Y-bounds.Min.Y)
		k := i.Out.At(x, y)

		var streakEnd int
		if length < 0 {
			if left {
				streakEnd = inBounds.Min.X
			} else {
				streakEnd = inBounds.Max.X
			}
		} else {
			if left {
				streakEnd = minInt(x-length, inBounds.Min.X)
			} else {
				streakEnd = minInt(x+length, inBounds.Max.X)
			}
		}

		for x >= streakEnd {
			r1, g1, b1, a1 := k.RGBA()
			r2, g2, b2, a2 := i.Out.At(x, y).RGBA()

			k = color.RGBA{
				c(r1/4*3 + r2/4),
				c(g1/4*3 + g2/4),
				c(b1/4*3 + b2/4),
				c(a1/4*3 + a2/4),
			}

			i.Out.Set(x, y, k)
			if left {
				x--
			} else {
				x++
			}
		}
		streaks--
	}
}

func (i *Img) Burst() {
	b := i.Bounds
	offset := rand.Intn(b.Dy()/10) + 25
	alpha := uint32(rand.Intn(MAXC))

	var out color.RGBA64

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sr, sg, sb, sa := i.Out.At(x, y).RGBA()

			dr, _, _, _ := i.Out.At(x+offset, y+offset).RGBA()
			_, dg, _, _ := i.Out.At(x-offset, y+offset).RGBA()
			_, _, db, _ := i.Out.At(x+offset, y-offset).RGBA()
			_, _, _, da := i.Out.At(x-offset, y-offset).RGBA()

			a := MAXC - (sa * alpha / MAXC)

			out.R = uint16((dr*a + sr*alpha) / MAXC)
			out.G = uint16((dg*a + sg*alpha) / MAXC)
			out.B = uint16((db*a + sb*alpha) / MAXC)
			out.A = uint16((da*a + sa*alpha) / MAXC)

			i.Out.Set(x, y, &out)
		}
	}
}

func (i *Img) GaussianNoise() {
	result := noise.Generate(i.Bounds.Max.X, i.Bounds.Max.Y, &noise.Options{Monochrome: true, NoiseFn: noise.Gaussian})
	i.Out = blend.Opacity(i.Out, result, 0.35)
}

func (i *Img) Noise(hex string) {
	color, err := ParseHexColor(hex)
	if err != nil {
		log.Fatal(err)
	}
	r, g, b, a := float64(color.R), float64(color.G), float64(color.B), float64(0.1)
	bounds := i.Bounds
	var out colorful.Color
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			baseRaw := i.Out.At(x, y)
			baseC, _ := colorful.MakeColor(baseRaw)

			randomBlue := colorful.LinearRgb(
				rand.Float64()*r,
				rand.Float64()*g,
				rand.Float64()*b,
			)

			out = baseC.BlendLab(randomBlue, rand.Float64()*a)
			i.Out.Set(x, y, &out)
		}
	}
}

func (i *Img) ShiftChannel(left bool) {
	bounds := i.Bounds
	leftInt := 0
	if left {
		leftInt = 1
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			shiftedColor := shiftColor(i.Out.At(x, y), leftInt)
			i.Out.Set(x, y, shiftedColor)
		}
	}
}

func (i *Img) Ghost() {
	b := bytes.NewBuffer([]byte{})
	var opt jpeg.Options
	opt.Quality = rand.Intn(50)

	jpeg.Encode(b, i.Out, &opt)

	img, _ := jpeg.Decode(b)
	bounds := i.Bounds

	m := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	c := color.RGBA{255, 255, 255, uint8(rand.Intn(255))}
	draw.Draw(m, m.Bounds(), &image.Uniform{c}, image.Point{0, 0}, draw.Src)
	draw.DrawMask(i.Out, bounds, img, image.Point{5, 5}, m, image.Point{5, 5}, draw.Over)
}

func (i *Img) GhostTint() {
	b := bytes.NewBuffer([]byte{})
	var opt jpeg.Options
	opt.Quality = rand.Intn(50)

	jpeg.Encode(b, i.Out, &opt)

	img, _ := jpeg.Decode(b)
	bounds := i.Bounds

	m := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	c := color.RGBA{255, 255, 255, uint8(rand.Intn(255))}
	draw.Draw(m, m.Bounds(), &image.Uniform{c}, image.Point{0, 0}, draw.Src)
	draw.DrawMask(i.Out, bounds, img, image.Point{5, 5}, m, image.Point{5, 5}, draw.Over)
}

func (i *Img) RandomCorruptions(uniform bool) {
	iterations := int(float64(i.In.Bounds().Max.Y) * float64(i.In.Bounds().Max.X) * 0.03)

	for it := 0; it <= iterations; it++ {
		height := rand.Intn(int(float64(i.In.Bounds().Max.Y) * 0.01))
		width := rand.Intn(int(float64(i.In.Bounds().Max.X) * 0.01))
		x := rand.Intn(i.In.Bounds().Max.X)
		y := rand.Intn(i.In.Bounds().Max.Y)
		destX := x + width
		destY := y + height

		r := image.Rect(x, y, destX, destY)
		p := image.Pt(x, y)
		randomColor := color.RGBA{0, 200, 0, 100}
		if uniform {
			randomColor = shiftColor(i.In.At(x, y), 1)
		}
		draw.Draw(i.Out, r, &image.Uniform{randomColor}, p, draw.Src)
	}
}

func (i *Img) CopyChannelBigLines() {
	bounds := i.Bounds
	cursor := bounds.Min.Y
	split := false
	height := i.Bounds.Max.Y * rand.Intn(25) / 100
	width := i.Bounds.Max.X * rand.Intn(10) / 100
	y := 0
	jitter := rand.Intn(100)

	for cursor < bounds.Max.Y {
		rC := RandomChannel()
		if split {
			jitter := rand.Intn(200)
			jitterWidth := rand.Intn(30)
			next := cursor + height + jitter
			if next >= bounds.Max.Y {
				return
			}
			for cursor <= next {
				for x := bounds.Min.X; x <= bounds.Max.X; x++ {
					tx := x + width + jitterWidth
					if tx >= bounds.Max.X {
						tx = tx - bounds.Max.X
					}
					i.CopyChannel(x, cursor, tx, cursor, rC)
				}
				cursor++
			}
			cursor = next
		} else {
			cursor = cursor + height + jitter
		}
		split = !split
		y++
	}
}

func (i *Img) CopyChannel(inX, inY, outX, outY int, copyChannel Channel) {
	// Note type assertion to get a color.RGBA
	r, g, b, a := i.In.At(inX, inY).RGBA()
	dr, dg, db, da := i.Out.At(outX, outY).RGBA()

	switch copyChannel {
	case Red:
		dr = r
	case Green:
		dg = g
	case Blue:
		db = b
	case Alpha:
		da = a
	}

	shiftedColor := color.RGBA{
		R: uint8(dr),
		G: uint8(dg),
		B: uint8(db),
		A: uint8(da),
	}

	i.Out.Set(outX, outY, shiftedColor)
}

// TODO: fix
func (i *Img) GhostStretch() {
	bounds := i.Bounds

	ghosts := rand.Intn(bounds.Dy()/10) + 1
	x := rand.Intn(bounds.Dx()/ghosts) - (bounds.Dx() / ghosts * 2)
	y := rand.Intn(bounds.Dy()/ghosts) - (bounds.Dy() / ghosts * 2)
	alpha := uint8(rand.Intn(255 / 2))

	m := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	c := color.RGBA{0, 0, 0, alpha}
	draw.Draw(m, m.Bounds(), &image.Uniform{c}, image.Point{0, 0}, draw.Src)

	for j := 1; j < ghosts; j++ {
		draw.DrawMask(i.Out, bounds, i.Out, image.Pt(x*j, y*j), m, image.Point{0, 0}, draw.Over)
	}
}

func (i *Img) ColorBoost(boostColor string) {
	bounds := i.Bounds

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := i.Out.At(x, y).RGBA()
			na := MAXC - (a * r / MAXC)
			switch boostColor {
			case "red":
				r = uint32((r*na + r*a) / MAXC)
			case "green":
				g = uint32((g*na + g*a) / MAXC)
			case "blue":
				b = uint32((b*na + b*a) / MAXC)
			}
			s := color.RGBA64{
				R: uint16(r),
				G: uint16(g),
				B: uint16(b),
				A: uint16(a),
			}
			i.Out.Set(x, y, s)
		}
	}
}

func (i *Img) Split(height, width int, split bool) {
	bounds := i.Bounds
	cursor := bounds.Min.Y

	for cursor < bounds.Max.Y {
		if split {
			next := cursor + height
			if next > bounds.Max.Y {
				return
			}
			for cursor < next {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					tx := x + width
					if tx >= bounds.Max.X {
						tx = tx - bounds.Max.X
					}
					c := i.In.At(tx, cursor)
					i.Out.Set(x, cursor, c)
				}
				cursor++
			}
			cursor = next
		} else {
			cursor = cursor + height
		}

		split = !split
	}
}

func (i *Img) VerticalSplit(width, height int, split bool) {
	bounds := i.Bounds
	cursor := bounds.Min.X

	for cursor < bounds.Max.X {
		if split {
			next := cursor + width
			if next > bounds.Max.X {
				return
			}
			for cursor < next {
				for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
					ty := y + height
					if ty > bounds.Max.Y {
						ty = ty - bounds.Max.Y
					}
					c := i.In.At(cursor, ty)
					i.Out.Set(cursor, y, c)
				}
				cursor++
			}
			cursor = next
		} else {
			cursor = cursor + width
		}
		split = !split
	}
}

func (i *Img) Scanlines() {
	var color = color.RGBA{0, 0, 0, 50}
	for y := 0; y < i.Bounds.Dy(); y++ {
		if y%3 == 0 {
			bresenham.DrawLine(i.Out, 0, y, i.Bounds.Dx(), y, color)
		}
	}
}

func (i *Img) BigLines() {
	bounds := i.Bounds
	cursor := bounds.Min.Y
	split := false
	height := i.Bounds.Max.Y * rand.Intn(25) / 100
	width := i.Bounds.Max.X * rand.Intn(10) / 100
	y := 0

	for cursor < bounds.Max.Y {
		if split {
			jitter := rand.Intn(200)
			next := cursor + height + jitter
			if next >= bounds.Max.Y {
				return
			}
			for cursor <= next {
				for x := bounds.Min.X; x <= bounds.Max.X; x++ {
					jitterWidth := rand.Intn(30)
					tx := x + width + jitterWidth
					if tx >= bounds.Max.X {
						tx = tx - bounds.Max.X
					}
					clr := i.In.At(x, cursor)
					if y%3 == 0 {
						clr = shiftColor(i.In.At(x, cursor), 1)
					}
					i.Out.Set(tx, cursor, clr)
				}
				cursor++
			}
			cursor = next
		} else {
			cursor = cursor + height
		}
		split = !split
		y++
	}
}

func (i *Img) OverlayImage(overlayImage string) {
	overlay, err := os.Open(overlayImage)
	if err != nil {
		fmt.Println("Could not open image!", err)
		os.Exit(1)
	}
	defer overlay.Close()

	img, err := png.Decode(overlay)
	if err != nil {
		fmt.Println("Could not decode .png file", err)
	}

	dst := image.NewRGBA(image.Rect(0, 0, i.In.Bounds().Max.X, i.In.Bounds().Max.Y))

	xdraw.ApproxBiLinear.Scale(i.Out, dst.Rect, img, img.Bounds(), draw.Over, nil)
}
