package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"os"
	"runtime"
)

var (
	iterations    int
	antialias     int
	continuous    bool
	palette       []color.NRGBA
	subpixOffsets []float64
)

type pixel struct {
	x, y  int
	color color.Color
}

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
		palettefile      string
	)

	flag.IntVar(&sizeX, "px", 1024, "Horizontal size of the image in pixels")
	flag.IntVar(&sizeY, "py", 768, "Vertical size of the image in pixels")
	flag.IntVar(&iterations, "i", 1000, "Maximum iterations per point")
	flag.Float64Var(&centerX, "x", -0.75, "Center point of the image, real part")
	flag.Float64Var(&centerY, "y", 0.0, "Center point of the image, imaginary part")
	flag.Float64Var(&magnification, "m", 0.4, "Magnification level")
	flag.IntVar(&antialias, "a", 2, "Anti-aliasing level for smoother image (1 is off)")
	flag.BoolVar(&continuous, "c", false, "Enable continuous color gradient")
	flag.StringVar(&filename, "file", "mandelbrot.png", "Output file name")
	flag.StringVar(&palettefile, "palette", "", "Palette JSON file (leave blank for default)")
	flag.Parse()

	if antialias < 1 {
		log.Fatalf("Anti-aliasing level must be 1 or higher")
	}
	subpixOffsets = make([]float64, antialias)
	for i := 0; i < antialias; i++ {
		subpixOffsets[i] = (0.5+float64(i))/float64(antialias) - 0.5
	}

	loadPalette(palettefile)

	// spin up row workers
	ch := make(chan int)
	done := make(chan bool)
	pixelch := make(chan pixel, sizeX)
	for i := 0; i < fanout; i++ {
		go func() {
			for row := range ch {
				for col := 0; col < sizeX; col++ {
					color := calcPixel(col, row, sizeX, sizeY, centerX, centerY, magnification)
					pixelch <- pixel{col, row, color}
				}
			}
			done <- true
		}()
	}

	// allocate the image
	canvas := image.NewNRGBA(image.Rect(0, 0, sizeX, sizeY))

	// set all pixels using a single worker
	go func() {
		for p := range pixelch {
			canvas.Set(p.x, p.y, p.color)
		}
		done <- true
	}()

	// feed the rows to the workers
	for row := 0; row < sizeY; row++ {
		fmt.Printf("\r%.2f%%", float64(100*row)/float64(sizeY))
		ch <- row
	}
	close(ch)

	// wait for workers to finish
	for i := 0; i < fanout; i++ {
		<-done
	}
	close(pixelch)
	<-done
	fmt.Printf("\rfinished\n")

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

func calcPixel(col, row, sizeX, sizeY int, centerX, centerY, magnification float64) color.Color {
	minsize := sizeX
	if sizeY < sizeX {
		minsize = sizeY
	}

	// loop over subpixels
	r, g, b := 0, 0, 0
	for _, yoffset := range subpixOffsets {
		for _, xoffset := range subpixOffsets {
			x := centerX + (float64(col-sizeX/2)+xoffset)/(magnification*float64(minsize-1))
			y := centerY - (float64(row-sizeY/2)-yoffset)/(magnification*float64(minsize-1))
			rs, gs, bs := getColor(mandel(x, y))
			r, g, b = r+rs, g+gs, b+bs
		}
	}

	aa := antialias * antialias
	return color.NRGBA{uint8(r / aa), uint8(g / aa), uint8(b / aa), 255}
}

func getColor(iters float64) (r, g, b int) {
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

func loadPalette(filename string) {
	var colors [][]uint8
	if filename == "" {
		colors = defaultColors
	} else {
		raw, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatalf("Error reading palette file %s: %v", filename, err)
		}
		if err = json.Unmarshal(raw, &colors); err != nil {
			log.Fatalf("Error parsing palette JSON data: %v", err)
		}
		if len(colors) < 1 {
			log.Fatalf("Palette must have at least color")
		}
	}
	for _, c := range colors {
		if len(c) != 4 {
			log.Fatalf("Error in palette file: each color must have exactly 4 elements: red, green, blue, and alpha: found %v", c)
		}
		palette = append(palette, color.NRGBA{c[0], c[1], c[2], c[3]})
	}
}
