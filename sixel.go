package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

var sixelPalette = []color.RGBA{
	{0, 0, 0, 255}, {255, 255, 255, 255}, {220, 50, 47, 255}, {38, 139, 210, 255},
	{133, 153, 0, 255}, {211, 54, 130, 255}, {42, 161, 152, 255}, {203, 75, 22, 255},
	{88, 110, 117, 255}, {238, 232, 213, 255}, {255, 100, 90, 255}, {100, 180, 255, 255},
	{180, 210, 60, 255}, {255, 120, 200, 255}, {80, 220, 190, 255}, {255, 170, 70, 255},
}

const coverSize = 96

func sixelImage(data []byte) string {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	img = resizeSquare(img, coverSize)
	b := strings.Builder{}
	b.WriteString("\x1bPq")
	b.WriteString(fmt.Sprintf("\"1;1;%d;%d", img.Bounds().Dx(), img.Bounds().Dy()))
	for i, c := range sixelPalette {
		b.WriteString(fmt.Sprintf("#%d;2;%d;%d;%d", i, int(c.R)*100/255, int(c.G)*100/255, int(c.B)*100/255))
	}
	for y := 0; y < img.Bounds().Dy(); y += 6 {
		if y > 0 {
			b.WriteByte('-')
		}
		for p := range sixelPalette {
			b.WriteString(fmt.Sprintf("#%d", p))
			last := byte(0)
			run := 0
			flush := func() {
				if run == 0 {
					return
				}
				if run == 1 {
					b.WriteByte(last)
				} else {
					b.WriteString(fmt.Sprintf("!%d%c", run, last))
				}
				run = 0
			}
			for x := 0; x < img.Bounds().Dx(); x++ {
				bits := byte(0)
				for bit := 0; bit < 6 && y+bit < img.Bounds().Dy(); bit++ {
					if nearest(img.At(x, y+bit)) == p {
						bits |= 1 << bit
					}
				}
				ch := byte(63 + bits)
				if run > 0 && ch != last {
					flush()
				}
				last, run = ch, run+1
			}
			flush()
			if p != len(sixelPalette)-1 {
				b.WriteByte('$')
			}
		}
	}
	b.WriteString("\x1b\\")
	return b.String()
}

func nearest(c color.Color) int {
	r, g, b, _ := c.RGBA()
	best, dist := 0, uint32(^uint32(0))
	for i, p := range sixelPalette {
		dr, dg, db := int(r>>8)-int(p.R), int(g>>8)-int(p.G), int(b>>8)-int(p.B)
		d := uint32(dr*dr + dg*dg + db*db)
		if d < dist {
			best, dist = i, d
		}
	}
	return best
}

func resizeSquare(src image.Image, size int) image.Image {
	b := src.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x0) + (float64(x)+0.5)*float64(side)/float64(size) - 0.5
			fy := float64(y0) + (float64(y)+0.5)*float64(side)/float64(size) - 0.5
			x1, y1 := int(math.Floor(fx)), int(math.Floor(fy))
			x1 = max(x0, min(x0+side-1, x1))
			y1 = max(y0, min(y0+side-1, y1))
			x2, y2 := min(x0+side-1, x1+1), min(y0+side-1, y1+1)
			tx, ty := fx-float64(x1), fy-float64(y1)
			if tx < 0 {
				tx = 0
			}
			if ty < 0 {
				ty = 0
			}
			r11, g11, b11, a11 := src.At(x1, y1).RGBA()
			r21, g21, b21, a21 := src.At(x2, y1).RGBA()
			r12, g12, b12, a12 := src.At(x1, y2).RGBA()
			r22, g22, b22, a22 := src.At(x2, y2).RGBA()
			blend := func(c11, c21, c12, c22 uint32) uint8 {
				top := float64(c11)*(1-tx) + float64(c21)*tx
				bottom := float64(c12)*(1-tx) + float64(c22)*tx
				return uint8((top*(1-ty) + bottom*ty) / 257)
			}
			dst.SetRGBA(x, y, color.RGBA{blend(r11, r21, r12, r22), blend(g11, g21, g12, g22), blend(b11, b21, b12, b22), blend(a11, a21, a12, a22)})
		}
	}
	return dst
}
