# soryu

glitch an image in the terminal

## Installation

`go get github.com/enjuus/soryu`

## Usage

```
NAME:
   soryu - CLI too glitch an image

NAME:
   soryu - CLI too glitch an image

USAGE:
   soryu [options]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --color-boost value, --cb value              the color to boost [red, green, blue] (default: "red")
   --gif, -g                                    generate an animated gif from multiple glitched versions of the given image (default: false)
   --input value, -i value                      the input file path
   --noise-color value, -n value                the hexcolor of the applied noise (default: "#c0ffee")
   --order value, -o value                      define which effect are to be applied and the order of them (default: "Streak,Burst,ShiftChannel,Ghost,GhostStretch,ColorBoost,Split,VerticalSplit,Noise")
   --output value, --out value                  the path where the file is written (default: "./glitched.png")
   --seed value, --se value                     give a seed (default: 1604524605403604700)       
   --shift-channel-direction, --scd             shift colorchannel direction, if true it is shifted left (default: false)
   --split-length value, --spl value            the length of the splits (default: 50)
   --split-width value, --spw value             the width of the splits (default: 3)
   --streak-amount value, --sa value            the amount of streaks to add to the image (default: 10000)
   --streak-direction, --sd                     the direction of the streak, true for left (default: false)
   --streak-width value, --sw value             the width of the streaks (default: 3)
   --vertical-split-length value, --vspl value  the length of the vertical splits (default: 50)  
   --vertical-split-width value, --vspw value   the width of the vertical splits (default: 3)    
   --help, -h                                   show help (default: false)
```


## Examples

Original

![original image](https://raw.githubusercontent.com/enjuus/soryu/main/examples/example.png)


With `Burst`, `Stretch`, `Streak` and `Split`

![modified 1](https://raw.githubusercontent.com/enjuus/soryu/main/examples/burst-stretch-streak-split.png)

With `Shift` and `Streak`

![modified 2](https://raw.githubusercontent.com/enjuus/soryu/main/examples/shift-right-streak.png)

`--gif` generates 10 different images and combines them into an animated gif

![gif](https://raw.githubusercontent.com/enjuus/soryu/main/examples/gif-example.gif)