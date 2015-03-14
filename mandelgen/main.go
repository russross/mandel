package main

import (
	"encoding/json"
	"flag"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"github.com/russross/mandel"
)

func main() {
	// use multiple CPUs if available
	runtime.GOMAXPROCS(runtime.NumCPU())

	// parse options
	p := new(mandel.Parameters)
	var filename, palettefile string

	flag.Float64Var(&p.CenterX, "x", -0.75, "Center point of the image, real part")
	flag.Float64Var(&p.CenterY, "y", 0.0, "Center point of the image, imaginary part")
	flag.Float64Var(&p.Magnification, "m", 0.4, "Magnification level")
	flag.IntVar(&p.MaxIterations, "i", 1000, "Maximum iterations per point")
	flag.IntVar(&p.SizeX, "px", 1024, "Horizontal size of the image in pixels")
	flag.IntVar(&p.SizeY, "py", 768, "Vertical size of the image in pixels")
	flag.IntVar(&p.AntiAlias, "a", 2, "Anti-aliasing level for smoother image (1 is off)")
	flag.BoolVar(&p.Continuous, "c", false, "Enable continuous color gradient")

	flag.StringVar(&filename, "o", "mandelbrot.png", "Output file name")
	flag.StringVar(&palettefile, "palette", "", "Palette JSON file (leave blank for default)")
	flag.Parse()

	if p.AntiAlias < 1 {
		log.Fatalf("Anti-aliasing level must be 1 or higher")
	}
	p.Palette = loadPalette(palettefile)

	if err := p.Init(); err != nil {
		log.Fatal(err)
	}
	canvas := p.Generate()

	// save the image
	fp, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Error creating file %s: %v", filename, err)
	}
	defer fp.Close()
	if err = png.Encode(fp, canvas); err != nil {
		log.Fatalf("Error encoding image: %v", err)
	}
	log.Printf("finished")
}

func loadPalette(filename string) []color.NRGBA {
	var palette []color.NRGBA
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
	return palette
}
