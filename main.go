package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/StephaneBunel/bresenham"
	"github.com/anthonynsimon/bild/blend"
	"github.com/anthonynsimon/bild/noise"
	"github.com/disintegration/imaging"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/urfave/cli/v2"
	xdraw "golang.org/x/image/draw"
)

var (
	inputFile            string
	outputFile           string
	effects              string
	streakAmount         int
	streakWidth          int
	streakDirection      bool
	noiseColor           string
	shiftChannel         bool
	colorBoost           string
	splitWidth           int
	splitLength          int
	seed                 int64
	makegif              bool
	gifDelay             int
	gifFrames            int
	overlayImage         string
	overlayEveryNthFrame int
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
	imgtype string
}

type Images struct {
	In     image.Image
	Gifify []Img
}

func NewImage(file string) (*Img, error) {
	nf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	imgtype := http.DetectContentType(nf)
	buff := bytes.NewBuffer(nf)
	var img image.Image

	switch imgtype {
	case "image/jpeg":
		img, err = jpeg.Decode(buff)
		if err != nil {
			return nil, err
		}
	case "image/png":
		img, err = png.Decode(buff)
		if err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	imgbounds := img.Bounds()

	image := &Img{
		In:      img,
		Bounds:  imgbounds,
		Out:     image.NewRGBA(image.Rect(0, 0, imgbounds.Dx(), imgbounds.Dy())),
		imgtype: "png",
	}

	return image, nil
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
	if i.imgtype == "png" {
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

func (i *Img) OverlayImage() {
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

func CreateGlitchedImage(fileName string, reseed bool, imgNumber int) *Img {
	i, err := NewImage(inputFile)
	if err != nil {
		log.Fatal(err)
	}
	i.Copy()
	commands := strings.Split(effects, ",")
	for _, effect := range commands {
		fmt.Println("Applying ", effect)
		switch effect {
		case "Streak":
			if imgNumber%2 == 0 {
				streakAmount += (rand.Intn(100) / 5) + 5
			}
			i.Streak(streakAmount, streakWidth, streakDirection)
		case "Burst":
			if imgNumber%2 == 0 {
				continue
			}
			i.Burst()
		case "ShiftChannel":
			i.ShiftChannel(shiftChannel)
		case "Ghost":
			i.Ghost()
		case "GhostStretch":
			i.GhostStretch()
		case "ColorBoost":
			i.ColorBoost(colorBoost)
		case "Split":
			if imgNumber%5 == 0 {
				continue
			}
			newWidth := splitWidth
			if imgNumber == 1 || imgNumber == 3 {
				newWidth = splitWidth + rand.Intn(10)
			}
			i.Split(newWidth, splitLength, false)
		case "VerticalSplit":
			if imgNumber%5 == 0 {
				continue
			}
			newWidth := splitWidth
			if imgNumber == 1 || imgNumber == 3 {
				newWidth = splitWidth + rand.Intn(10)
			}
			i.VerticalSplit(newWidth, splitLength, false)
		case "Noise":
			i.Noise(noiseColor)
		case "GaussianNoise":
			i.GaussianNoise()
		case "Scanlines":
			i.Scanlines()
		case "BigLines":
			if imgNumber%5 == 0 {
				continue
			}
			i.BigLines()
		case "CopyChannelBigLines":
			i.CopyChannelBigLines()
		case "RandomCorruptions":
			if makegif {
				if imgNumber%6 == 0 {
					i.RandomCorruptions(false)
				}
			} else {
				i.RandomCorruptions(false)
			}
		case "OverlayImage":
			if makegif {
				if imgNumber%overlayEveryNthFrame == 0 {
					i.OverlayImage()
				}
			} else {
				i.OverlayImage()
			}
		}
	}
	newFile := fileName
	f, err := os.Create(newFile)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Writing file to ", newFile)
	i.Write(f)
	return i
}

func Run() {
	if !makegif {
		rand.Seed(seed)
		CreateGlitchedImage(outputFile, false, 1)
		os.Exit(0)
	}
	for i := 0; i < gifFrames; i++ {
		rand.Seed(time.Now().UTC().UnixNano())
		tmpFileName := fmt.Sprintf("./temp%d.png", i) //TODO Write to temp folder for each OS
		CreateGlitchedImage(tmpFileName, true, i)
	}
	srcFiles, err := filepath.Glob("temp*.png")
	if err != nil {
		log.Fatalf("error in globbing source file pattern %s", err)
	}

	if len(srcFiles) == 0 {
		log.Fatalf("No source images found via pattern")
	}

	sort.Strings(srcFiles)

	var frames []*image.Paletted

	for _, filename := range srcFiles {
		img, err := imaging.Open(filename)
		if err != nil {
			fmt.Println("Couldn't load file")
			os.Exit(1)
		}
		buf := bytes.Buffer{}
		if err := gif.Encode(&buf, img, nil); err != nil {
			log.Printf("Skilling file %s due to errorr in gif encoding: %s", filename, err)
		}

		tmpimg, err := gif.Decode(&buf)
		if err != nil {
			log.Printf("skipping file %s due to weird error reading the temp gif: %s", filename, err)
		}

		frames = append(frames, tmpimg.(*image.Paletted))
	}
	log.Printf("Parsed all images... creating gif")
	newFile := outputFile
	opfile, err := os.Create(newFile)
	if err != nil {
		log.Fatalf("Error creating the destination file %s : %s", outputFile, err)
	}

	delays := make([]int, len(frames))
	for j := range delays {
		delays[j] = gifDelay
	}

	if err := gif.EncodeAll(opfile, &gif.GIF{Image: frames, Delay: delays, LoopCount: 0}); err != nil {
		log.Printf("error encoding output into animated gif: %s", err)
	}
	opfile.Close()
	files, err := filepath.Glob("temp*.png")
	if err != nil {
		log.Fatalln("couldn't delete temp files", err)
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			log.Fatalln("couldnt remove file", f, err)
		}
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "soryu"
	app.Usage = "CLI too glitch an image"
	app.UsageText = "soryu [options]"
	app.Flags = []cli.Flag{
		&cli.Int64Flag{
			Name:    "seed",
			Aliases: []string{"se"},
			Usage:   "give a seed",
			Value:   time.Now().UTC().UnixNano(),
		},
		&cli.StringFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Usage:   "the input file path",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"out"},
			Usage:   "the path where the file is written",
			Value:   "./glitched.png",
		},
		&cli.StringFlag{
			Name:    "order",
			Aliases: []string{"o"},
			Usage:   "define which effect are to be applied and the order of them",
			Value:   "Streak,Burst,ShiftChannel,Ghost,GhostStretch,ColorBoost,Split,VerticalSplit,Noise,GaussianNoise,Scanlines",
		},
		// Streak - amount int, width int, direction bool true = left
		&cli.IntFlag{
			Name:    "streak-amount",
			Aliases: []string{"sa"},
			Usage:   "the amount of streaks to add to the image",
			Value:   10000,
		},
		&cli.IntFlag{
			Name:    "streak-width",
			Aliases: []string{"sw"},
			Usage:   "the width of the streaks",
			Value:   3,
		},
		&cli.BoolFlag{
			Name:    "streak-direction",
			Aliases: []string{"sd"},
			Usage:   "the direction of the streak, true for left",
			Value:   false,
		},
		// Noise - #FFFFFF
		&cli.StringFlag{
			Name:    "noise-color",
			Aliases: []string{"n"},
			Usage:   "the hexcolor of the applied noise",
			Value:   "#c0ffee",
		},
		// ShiftChannel - direction bool, true = left
		&cli.BoolFlag{
			Name:    "shift-channel-direction",
			Aliases: []string{"scd"},
			Usage:   "shift colorchannel direction, if true it is shifted left",
			Value:   false,
		},
		// Colorboost - red, green, blue string
		&cli.StringFlag{
			Name:    "color-boost",
			Aliases: []string{"cb"},
			Usage:   "the color to boost [red, green, blue]",
			Value:   "red",
		},
		// Split - width, length int, true
		&cli.IntFlag{
			Name:    "split-width",
			Aliases: []string{"spw"},
			Usage:   "the width of the splits",
			Value:   3,
		},
		&cli.IntFlag{
			Name:    "split-length",
			Aliases: []string{"spl"},
			Usage:   "the length of the splits",
			Value:   50,
		},
		// VerticalSplit - width, length int, true
		&cli.IntFlag{
			Name:    "vertical-split-width",
			Aliases: []string{"vspw"},
			Usage:   "the width of the vertical splits",
			Value:   3,
		},
		&cli.IntFlag{
			Name:    "vertical-split-length",
			Aliases: []string{"vspl"},
			Usage:   "the length of the vertical splits",
			Value:   50,
		},
		&cli.BoolFlag{
			Name:    "gif",
			Aliases: []string{"g"},
			Usage:   "generate an animated gif from multiple glitched versions of the given image",
			Value:   false,
		},
		&cli.IntFlag{
			Name:    "gif-delay",
			Aliases: []string{"gd"},
			Usage:   "the amount of delay between frames",
			Value:   20,
		},
		&cli.IntFlag{
			Name:    "gif-frames",
			Aliases: []string{"gf"},
			Usage:   "the amount of frames to be genrated for the gif",
			Value:   10,
		},
		&cli.StringFlag{
			Name:    "overlay-image",
			Aliases: []string{"oi"},
			Usage:   "overlay a png image over the file",
			Value:   "",
		},
		&cli.IntFlag{
			Name:    "overlay-every-nth-frame",
			Aliases: []string{"oenf"},
			Usage:   "overlay every nth frame in a gif",
			Value:   3,
		},
	}

	app.Action = func(c *cli.Context) error {
		seed = c.Int64("seed")
		inputFile = c.String("input")
		outputFile = c.String("output")
		streakAmount = c.Int("streak-amount")
		streakWidth = c.Int("streak-width")
		streakDirection = c.Bool("streak-direction")
		effects = c.String("order")
		noiseColor = c.String("noise-color")
		shiftChannel = c.Bool("shift-channel-direction")
		colorBoost = c.String("color-boost")
		splitWidth = c.Int("split-width")
		splitLength = c.Int("split-length")
		makegif = c.Bool("gif")
		gifDelay = c.Int("gif-delay")
		gifFrames = c.Int("gif-frames")
		overlayImage = c.String("overlay-image")
		overlayEveryNthFrame = c.Int("overlay-every-nth-frame")
		if inputFile == "" {
			log.Fatal("Please enter a file")
		}

		Run()
		return nil
	}
	sort.Sort(cli.FlagsByName(app.Flags))
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
