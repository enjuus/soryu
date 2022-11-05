package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/disintegration/imaging"
	"github.com/enjuus/soryu/soryu"
	"github.com/urfave/cli/v2"
)

var (
	inputFile            string
	outputFile           string
	effects              string
	effectsGui           []string
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
	gui                  bool
	currentImg           *soryu.Img
)

func NewImage(file string) (*soryu.Img, error) {
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

	image := &soryu.Img{
		In:      img,
		Bounds:  imgbounds,
		Out:     image.NewRGBA(image.Rect(0, 0, imgbounds.Dx(), imgbounds.Dy())),
		Imgtype: "png",
	}

	return image, nil
}

func CreateGlitchedImage(fileName string, reseed bool, imgNumber int) *soryu.Img {
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
					i.OverlayImage(overlayImage)
				}
			} else {
				i.OverlayImage(overlayImage)
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

func RunWithGui() {
	a := app.NewWithID("soryu")

	// Image window
	w := a.NewWindow("soryu")
	wFile := a.NewWindow("soryo - file")
	wFile.Resize(fyne.NewSize(600, 600))
	//w.SetContent(image)
	//w.Resize(fyne.NewSize(float32(i.Bounds.Dx()), float32(i.Bounds.Dy())))

	fmt.Println("show and run")

	// Settings window
	wSettings := a.NewWindow("soryu - settings")

	// Form input bindings
	bStreakAmount := binding.NewString()
	bStreakAmount.Set(strconv.Itoa(streakAmount))

	bStreakWidth := binding.NewString()
	bStreakWidth.Set(strconv.Itoa(streakWidth))

	bStreakDirection := binding.NewStringList()
	bStreakDirection.Set([]string{"left", "right"})

	bNoiseColor := binding.NewString()
	bNoiseColor.Set(noiseColor)

	bShiftChannel := binding.NewBool()
	bShiftChannel.Set(shiftChannel)

	bSplitWidth := binding.NewString()
	bSplitWidth.Set(strconv.Itoa(splitWidth))

	bSplitLength := binding.NewString()
	bSplitLength.Set(strconv.Itoa(splitLength))

	bSeed := binding.NewString()
	bSeed.Set(strconv.Itoa(int(seed)))

	// Form

	saveButtonInput := widget.NewButton("Choose path", func() {
		wFileSave := a.NewWindow("soryu - save file")
		wFileSave.Resize(fyne.NewSize(600, 600))
		outputFileInput := dialog.NewFileSave(func(file fyne.URIWriteCloser, err error) {
			filename := file.URI().Path()
			if err != nil {
				fmt.Println("error saving")
			} else {
				f, err := os.Create(filename)
				if err != nil {
					fmt.Println("error saving")
				}
				currentImg.Write(f)
				notif := fyne.NewNotification("Soryo", "Image saved")
				app.New().SendNotification(notif)
				wFileSave.Close()
			}
		}, wFileSave)
		outputFileInput.Show()
		wFileSave.Show()
	})

	effectOptions := []string{"Streak", "Burst", "ShiftChannel", "Ghost", "GhostStretch", "ColorBoost", "Split", "VerticalSplit", "Noise", "GaussianNoise", "Scanlines"}
	guiEffectsInput := widget.NewCheckGroup(effectOptions, func(selected []string) {
		effectsGui = selected
	})

	streakAmountInput := widget.NewEntryWithData(bStreakAmount)
	streakWidthInput := widget.NewEntryWithData(bStreakWidth)
	streakDirectionRadio := widget.NewRadioGroup([]string{"left", "right"}, func(res string) {
		if res == "left" {
			streakDirection = true
		} else {
			streakDirection = false
		}
	})
	colorBoostRadio := widget.NewRadioGroup([]string{"blue", "red", "green"}, func(res string) {
		colorBoost = res
	})
	noiseColorInput := widget.NewEntryWithData(bNoiseColor)
	splitWidthInput := widget.NewEntryWithData(bSplitWidth)
	splitLengthInput := widget.NewEntryWithData(bSplitLength)
	//seedInput := widget.NewEntryWithData(bSeed)

	form := &widget.Form{
		SubmitText: "Generate",
		Items: []*widget.FormItem{
			{Text: "Effects to apply", Widget: guiEffectsInput},
			{Text: "Streak amount", Widget: streakAmountInput},
			{Text: "Streak width", Widget: streakWidthInput},
			{Text: "Streak direction", Widget: streakDirectionRadio},
			{Text: "boost color", Widget: colorBoostRadio},
			{Text: "Noise color", Widget: noiseColorInput},
			{Text: "Split length", Widget: splitLengthInput},
			{Text: "Split width", Widget: splitWidthInput},
			{Text: "Save", Widget: saveButtonInput},
		},
		OnSubmit: func() {
			streakAmountT, _ := bStreakAmount.Get()
			streakAmount, _ = strconv.Atoi(streakAmountT)

			streakWidthT, _ := bStreakWidth.Get()
			streakWidth, _ = strconv.Atoi(streakWidthT)

			bStreakDirection.Set([]string{"left", "right"})

			bNoiseColor.Set(noiseColor)

			bShiftChannel.Set(shiftChannel)

			splitWidthT, _ := bSplitWidth.Get()
			splitWidth, _ = strconv.Atoi(splitWidthT)

			splitLengthT, _ := bSplitLength.Get()
			splitLength, _ = strconv.Atoi(splitLengthT)

			seedT, _ := bSeed.Get()
			seedTi, _ := strconv.Atoi(seedT)
			seed = int64(seedTi)
			fmt.Println(inputFile)
			newI, err := NewImage(inputFile)
			if err != nil {
				log.Fatal(err)
			}
			newI.Copy()
			newI = handleGuiEffects(newI)
			image := canvas.NewImageFromImage(newI.Out)
			currentImg = newI
			image.FillMode = canvas.ImageFillContain
			w.Resize(fyne.NewSize(float32(newI.Bounds.Dx()), float32(newI.Bounds.Dy())))
			w.SetContent(image)
		},
	}
	wSettings.SetContent(form)

	fileNameInput := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
		inputFile = file.URI().Path()
		newI, err := NewImage(inputFile)
		if err != nil {
			fmt.Println("error")
		} else {
			image := canvas.NewImageFromImage(newI.In)
			image.FillMode = canvas.ImageFillContain
			w.Resize(fyne.NewSize(float32(newI.Bounds.Dx()), float32(newI.Bounds.Dy())))
			w.SetContent(image)
			w.Show()
			wSettings.Show()
			wFile.Close()
		}
	}, wFile)

	btn := widget.NewButton("File:", func() {
		fileNameInput.Show()
	})
	wFile.SetContent(btn)
	wFile.Show()
	a.Run()
}

func handleGuiEffects(i *soryu.Img) *soryu.Img {
	for _, effect := range effectsGui {
		fmt.Println("Applying ", effect)
		switch effect {
		case "Streak":
			i.Streak(streakAmount, streakWidth, streakDirection)
		case "ShiftChannel":
			i.ShiftChannel(shiftChannel)
		case "Ghost":
			i.Ghost()
		case "GhostStretch":
			i.GhostStretch()
		case "ColorBoost":
			i.ColorBoost(colorBoost)
		case "Split":
			newWidth := splitWidth
			i.Split(newWidth, splitLength, false)
		case "VerticalSplit":
			newWidth := splitWidth
			i.VerticalSplit(newWidth, splitLength, false)
		case "Noise":
			i.Noise(noiseColor)
		case "GaussianNoise":
			i.GaussianNoise()
		case "Scanlines":
			i.Scanlines()
		case "BigLines":
			i.BigLines()
		case "CopyChannelBigLines":
			i.CopyChannelBigLines()
		case "RandomCorruptions":
			i.RandomCorruptions(false)
		case "Burst":
			i.Burst()
		case "OverlayImage":
			i.OverlayImage(overlayImage)
		}
	}

	return i
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
			Usage:   "the direction of the streak, true for left [broken]",
			Value:   true,
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
		&cli.BoolFlag{
			Name:  "gui",
			Usage: "run soryu with a gui",
			Value: false,
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
		gui = c.Bool("gui")
		if inputFile == "" && !gui {
			log.Fatal("Please enter a file")
		}

		if gui {
			RunWithGui()
		} else {
			Run()
		}
		return nil
	}
	sort.Sort(cli.FlagsByName(app.Flags))
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
