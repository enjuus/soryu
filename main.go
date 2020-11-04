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

	"github.com/disintegration/imaging"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/urfave/cli"
)

var (
	inputFile           string
	outputFile          string
	effects             string
	streakAmount        int
	streakWidth         int
	streakDirection     bool
	noiseColor          string
	shiftChannel        bool
	colorBoost          string
	splitWidth          int
	splitLength         int
	verticalSplitWidth  int
	verticalSplitLength int
	seed                int64
	makegif             bool
	gifDelay            int
	gifFrames           int
)

const MAXC = 1<<16 - 1

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
			if left == true {
				streakEnd = inBounds.Min.X
			} else {
				streakEnd = inBounds.Max.X
			}
		} else {
			if left == true {
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
			if left == true {
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

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := i.Out.At(x, y).RGBA()
			shiftedColor := color.RGBA{
				R: uint8(b),
				G: uint8(r),
				B: uint8(g),
				A: uint8(a),
			}

			if left == true {
				shiftedColor = color.RGBA{
					R: uint8(g),
					G: uint8(b),
					B: uint8(r),
					A: uint8(a),
				}
			}
			i.Out.Set(x, y, shiftedColor)
		}
	}
}

func (i *Img) Ghost() {
	b := bytes.NewBuffer([]byte{})
	var opt jpeg.Options
	opt.Quality = rand.Intn(10)

	jpeg.Encode(b, i.Out, &opt)

	img, _ := jpeg.Decode(b)
	bounds := i.Bounds

	m := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	c := color.RGBA{255, 255, 255, uint8(rand.Intn(255))}
	draw.Draw(m, m.Bounds(), &image.Uniform{c}, image.ZP, draw.Src)
	draw.DrawMask(i.Out, bounds, img, image.ZP, m, image.ZP, draw.Over)
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
	draw.Draw(m, m.Bounds(), &image.Uniform{c}, image.ZP, draw.Src)

	for j := 1; j < ghosts; j++ {
		draw.DrawMask(i.Out, bounds, i.Out, image.Pt(x*j, y*j), m, image.ZP, draw.Over)
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
					if tx > bounds.Max.X {
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

func c(a uint32) uint8 {
	return uint8((float64(a) / MAXC) * 255)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
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
		}
	}
	newFile := fmt.Sprintf("%s", fileName)
	f, err := os.Create(newFile)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Writing file to ", newFile)
	i.Write(f)
	return i
}

func Run() {
	if makegif == false {
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
	newFile := fmt.Sprintf("%s", outputFile)
	opfile, err := os.Create(newFile)
	if err != nil {
		log.Fatalf("Error creating the destination file %s : %s", outputFile, err)
	}

	delays := make([]int, len(frames))
	for j, _ := range delays {
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
			Value:   "Streak,Burst,ShiftChannel,Ghost,GhostStretch,ColorBoost,Split,VerticalSplit,Noise",
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
		verticalSplitWidth = c.Int("vertical-split-width")
		verticalSplitLength = c.Int("vertical-split-length")
		makegif = c.Bool("gif")
		gifDelay = c.Int("gif-delay")
		gifFrames = c.Int("gif-frames")
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
