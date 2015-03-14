package mandel

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
)

type Parameters struct {
	CenterX       float64       `json:"x"`
	CenterY       float64       `json:"y"`
	Magnification float64       `json:"m"`
	MaxIterations int           `json:"i"`
	SizeX         int           `json:"px"`
	SizeY         int           `json:"py"`
	AntiAlias     int           `json:"a"`
	Continuous    bool          `json:"c"`
	Palette       []color.NRGBA `json:"palette"`
	InsideColor   color.NRGBA   `json:"inside"`
	subpixOffsets []float64
}

func (p *Parameters) Init() error {
	// compute subpixel offsets
	if p.AntiAlias < 1 {
		return fmt.Errorf("anti-aliasing level must be 1 or higher")
	}
	p.subpixOffsets = make([]float64, p.AntiAlias)
	for i := 0; i < p.AntiAlias; i++ {
		p.subpixOffsets[i] = (0.5+float64(i))/float64(p.AntiAlias) - 0.5
	}

	if len(p.Palette) < 1 {
		return fmt.Errorf("palette must not be empty")
	}

	return nil
}

type pixel struct {
	x, y  int
	color color.Color
}

func (p *Parameters) Generate() *image.NRGBA {
	if len(p.subpixOffsets) != p.AntiAlias {
		panic("Generate cannot be called before Init")
	}

	// spin up row workers
	fanout := runtime.GOMAXPROCS(-1)
	rows := make(chan int)
	done := make(chan struct{})
	pixelch := make(chan pixel, p.SizeX)
	for i := 0; i < fanout; i++ {
		go func() {
			for row := range rows {
				for col := 0; col < p.SizeX; col++ {
					color := p.CalcPixel(col, row)
					pixelch <- pixel{col, row, color}
				}
			}
			done <- struct{}{}
		}()
	}

	// allocate the image
	canvas := image.NewNRGBA(image.Rect(0, 0, p.SizeX, p.SizeY))

	// set all pixels using a single worker
	go func() {
		for pix := range pixelch {
			canvas.Set(pix.x, pix.y, pix.color)
		}
		done <- struct{}{}
	}()

	// feed the rows to the workers
	for row := 0; row < p.SizeY; row++ {
		rows <- row
	}
	close(rows)

	// wait for workers to finish
	for i := 0; i < fanout; i++ {
		<-done
	}
	close(pixelch)
	<-done

	return canvas
}

func (p *Parameters) CalcPixel(col, row int) color.Color {
	if len(p.subpixOffsets) != p.AntiAlias {
		panic("CalcPixel cannot be called before Init")
	}

	minsize := p.SizeX
	if p.SizeY < p.SizeX {
		minsize = p.SizeY
	}

	// loop over subpixels
	r, g, b := 0, 0, 0
	for _, yoffset := range p.subpixOffsets {
		for _, xoffset := range p.subpixOffsets {
			x := p.CenterX + (float64(col-p.SizeX/2)+xoffset)/(p.Magnification*float64(minsize-1))
			y := p.CenterY - (float64(row-p.SizeY/2)-yoffset)/(p.Magnification*float64(minsize-1))
			rs, gs, bs := p.getColor(mandel(p.MaxIterations, x, y, p.Continuous))
			r, g, b = r+rs, g+gs, b+bs
		}
	}

	aa := p.AntiAlias * p.AntiAlias
	return color.NRGBA{uint8(r / aa), uint8(g / aa), uint8(b / aa), 255}
}

func (p *Parameters) getColor(iters float64) (r, g, b int) {
	if iters == 0.0 {
		c := p.InsideColor
		return int(c.R), int(c.G), int(c.B)
	}
	if !p.Continuous {
		c := p.Palette[int(iters)%len(p.Palette)]
		return int(c.R), int(c.G), int(c.B)
	}

	aa := int(math.Floor(iters))
	bb := int(math.Floor(iters)) + 1
	if aa < 1 {
		aa, bb = 1, 2
	}
	weight := iters - math.Floor(iters)
	c1 := p.Palette[(aa-1)%len(p.Palette)]
	c2 := p.Palette[(bb-1)%len(p.Palette)]
	r = int(float64(c1.R)*(1.0-weight) + float64(c2.R)*weight)
	g = int(float64(c1.G)*(1.0-weight) + float64(c2.G)*weight)
	b = int(float64(c1.B)*(1.0-weight) + float64(c2.B)*weight)
	return r, g, b
}

func mandel(maxIters int, x, y float64, continuous bool) float64 {
	bailout := float64(4.0)
	if continuous {
		bailout = 2 << 16
	}
	a, b := x, y
	for iters := 1; iters <= maxIters; iters++ {
		a2 := a * a
		b2 := b * b
		if a2+b2 >= bailout {
			if continuous {
				nu := math.Log2(math.Log2(a2+b2) * 0.5)
				return float64(iters+1) - nu
			} else {
				return float64(iters)
			}
		}
		ab := a * b
		a = a2 - b2 + x
		b = ab + ab + y
	}
	return 0.0
}
