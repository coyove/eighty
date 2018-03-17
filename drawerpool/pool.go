package drawerpool

import (
	"image"
	"io/ioutil"
	"log"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

var fontf *truetype.Font

const (
	FontSize   = 16
	DPI        = 72
	LineHeight = FontSize * DPI * 6 / 5 / 72
)

func init() {
	if fontBytes, err := ioutil.ReadFile("test/unifont-10.0.07.ttf"); err != nil {
		log.Fatalln(err)
	} else if f, err := truetype.Parse(fontBytes); err != nil {
		log.Fatalln(err)
	} else {
		fontf = f
	}
}

type pair struct {
	pool *Pool
	*font.Drawer
}

type Pool struct {
	c chan *pair
}

func NewPool(size int, imgW, imgH int, initstub func(d *font.Drawer)) *Pool {
	p := &Pool{
		c: make(chan *pair, size),
	}

	for i := 0; i < size; i++ {
		pp := &pair{
			Drawer: &font.Drawer{
				Dst: image.NewPaletted(image.Rect(0, 0, imgW, imgH), nil),
				Face: truetype.NewFace(fontf, &truetype.Options{
					Size:    FontSize,
					DPI:     DPI,
					Hinting: font.HintingNone,
				}),
			},
			pool: p,
		}
		if initstub != nil {
			initstub(pp.Drawer)
		}
		p.c <- pp
	}

	return p
}

func (p *Pool) Get() *pair {
	return <-p.c
}

func (p *pair) Free() {
	p.pool.c <- p
}
