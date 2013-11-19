package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"runtime"
)

var pixelSize, subpixelSize float64
var iterations int
var antialias bool
var continuous bool

func main() {
	// use multiple CPUs if available
	fanout := runtime.NumCPU()
	runtime.GOMAXPROCS(fanout)

	// parse options
	var (
		centerX, centerY float64
		magnification    float64
		sizeX, sizeY     int
		filename         string
	)

	flag.IntVar(&sizeX, "px", 1024, "Horizontal size of the image in pixels")
	flag.IntVar(&sizeY, "py", 768, "Vertical size of the image in pixels")
	flag.IntVar(&iterations, "n", 1000, "Maximum iterations per point")
	flag.Float64Var(&centerX, "x", -0.75, "Center point of the image, real part")
	flag.Float64Var(&centerY, "y", 0.0, "Center point of the image, imaginary part")
	flag.Float64Var(&magnification, "mag", 0.4, "Magnification level")
	flag.BoolVar(&antialias, "a", true, "Enable anti-aliasing for smoother image")
	flag.BoolVar(&continuous, "c", false, "Enable continuous color gradient")
	flag.StringVar(&filename, "file", "mandelbrot.png", "Output file name")
	flag.Parse()

	// compute image parameters
	minsize := sizeX
	if sizeY < sizeX {
		minsize = sizeY
	}
	pixelSize = 1.0 / magnification / float64(minsize-1)
	subpixelSize = pixelSize / 4.0

	// allocate the image
	canvas := image.NewNRGBA(image.Rect(0, 0, sizeX, sizeY))

	// spin up row workers
	ch := make(chan int)
	done := make(chan bool)
	for i := 0; i < fanout; i++ {
		go func() {
			for row := range ch {
				xstart := centerX + float64(-sizeX/2)*pixelSize
				ystart := centerY - float64(row-sizeY/2)*pixelSize
				calcRow(canvas, xstart, ystart, row)
			}
			done <- true
		}()
	}

	// feed the rows to the workers
	for row := 0; row < sizeY; row++ {
		ch <- row
	}
	close(ch)

	// wait for workers to finish
	for i := 0; i < fanout; i++ {
		<-done
	}

	// save the image
	fp, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Error creating file %s: %v", filename, err)
	}
	defer fp.Close()
	if err = png.Encode(fp, canvas); err != nil {
		log.Fatalf("Error encoding image: %v", err)
	}
}

func calcRow(canvas *image.NRGBA, x, y float64, row int) {
	sizeX := canvas.Bounds().Max.X
	for col := 0; col < sizeX; col++ {
		pixel := calcPixel(x, y)
		canvas.Set(col, row, pixel)
		x += pixelSize
	}
}

func calcPixel(x, y float64) color.Color {
	if !antialias {
		r, g, b := getColorAtPoint(x, y)
		return color.NRGBA{uint8(r), uint8(g), uint8(b), 255}
	}

	r1, g1, b1 := getColorAtPoint(x-subpixelSize, y+subpixelSize)
	r2, g2, b2 := getColorAtPoint(x+subpixelSize, y+subpixelSize)
	r3, g3, b3 := getColorAtPoint(x-subpixelSize, y-subpixelSize)
	r4, g4, b4 := getColorAtPoint(x+subpixelSize, y-subpixelSize)
	return color.NRGBA{uint8((r1 + r2 + r3 + r4) / 4), uint8((g1 + g2 + g3 + g4) / 4), uint8((b1 + b2 + b3 + b4) / 4), 255}
}

func getColorAtPoint(x, y float64) (r, g, b int) {
	iters := mandel(x, y)
	if iters == 0.0 {
		return
	}
	if !continuous {
		c := palette[int(iters)%len(palette)]
		return int(c.R), int(c.G), int(c.B)
	}

	aa := int(math.Floor(iters))
	bb := int(math.Floor(iters)) + 1
	if aa < 1 {
		aa, bb = 1, 2
	}
	weight := iters - math.Floor(iters)
	c1 := palette[(aa-1)%len(palette)]
	c2 := palette[(bb-1)%len(palette)]
	r = int(float64(c1.R)*(1.0-weight) + float64(c2.R)*weight)
	g = int(float64(c1.G)*(1.0-weight) + float64(c2.G)*weight)
	b = int(float64(c1.B)*(1.0-weight) + float64(c2.B)*weight)
	return
}

func mandel(x, y float64) float64 {
	bailout := float64(4.0)
	if continuous {
		bailout = 2 << 16
	}
	a, b := x, y
	for iters := 1; iters <= iterations; iters++ {
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
