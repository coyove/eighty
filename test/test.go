package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/coyove/eighty/kkformat"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

var (
	dpi      = flag.Float64("dpi", 72, "screen resolution in Dots Per Inch")
	fontfile = flag.String("fontfile", "simsun.ttc", "filename of the ttf font")
	hinting  = flag.String("hinting", "none", "none | full")
	size     = flag.Float64("size", 16, "font size in points")
	spacing  = flag.Float64("spacing", 1.5, "line spacing (e.g. 2 means double spaced)")
	wonb     = flag.Bool("whiteonblack", false, "white text on a black background")
)

const title = "Jabberwocky"

var text = " ``` "

func main() {
	flag.Parse()

	// Read the font data.
	fontBytes, err := ioutil.ReadFile("unifont-10.0.07.ttf")
	if err != nil {
		log.Println(err)
		return
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Println(err)
		return
	}

	const imgW, imgH = 756, 9500
	th := kkformat.BlackTheme
	pp, bg := kkformat.GetTheme(th)
	rgba := image.NewPaletted(image.Rect(0, 0, imgW, imgH), pp)
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)

	d := &font.Drawer{
		Dst: rgba,
		Face: truetype.NewFace(f, &truetype.Options{
			Size:    *size,
			DPI:     *dpi,
			Hinting: font.HintingNone,
		}),
	}

	start := time.Now()
	ibuf, _ := ioutil.ReadFile(`../_raw/lorem.txt`)
	ibuf = append([]byte(" \na(/*/**//*/)\na(\"/*/**//*/\")\n//\"\n"), ibuf...)
	fo := &kkformat.Formatter{Source: ibuf, Img: d, LineHeight: int(*size * *dpi * 6 / 5 / 72), Columns: 80, Theme: th}
	img := fo.Render()
	log.Println(time.Now().Sub(start).Nanoseconds() / 1e6)

	// Save that RGBA image to disk.
	outFile, err := os.Create("out.png")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer outFile.Close()
	b := bufio.NewWriter(outFile)
	err = png.Encode(b, img)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	err = b.Flush()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	fmt.Println("Wrote out.png OK.")
}
