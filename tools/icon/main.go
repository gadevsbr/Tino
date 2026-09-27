package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

	ico "github.com/Kodeworks/golang-image-ico"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "uso: go run ./tools/icon entrada.png saida.ico")
		os.Exit(2)
	}
	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	source, err := png.Decode(in)
	_ = in.Close()
	if err != nil {
		panic(err)
	}

	const size = 256
	outImage := image.NewNRGBA(image.Rect(0, 0, size, size))
	bounds := source.Bounds()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/size
			sy := bounds.Min.Y + y*bounds.Dy()/size
			outImage.Set(x, y, source.At(sx, sy))
		}
	}

	out, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	if err := ico.Encode(out, outImage); err != nil {
		_ = out.Close()
		panic(err)
	}
	if err := out.Close(); err != nil {
		panic(err)
	}
}
