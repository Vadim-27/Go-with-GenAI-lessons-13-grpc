package main

import (
	"image"
	"image/color"
)

// fit зменшує зображення так, щоб воно вписалося в maxW×maxH із збереженням
// пропорцій. Збільшувати не будемо: менше за ліміт — повертаємо як є.
// Білінійна інтерполяція лише зі стандартної бібліотеки — жодних зайвих
// залежностей, адже кожен пакет збільшує бінарник і холодний старт (слайд 41).
func fit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxW && h <= maxH {
		return src
	}

	scale := float64(maxW) / float64(w)
	if s := float64(maxH) / float64(h); s < scale {
		scale = s
	}
	nw, nh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))

	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := (float64(y)+0.5)/scale - 0.5
		for x := 0; x < nw; x++ {
			sx := (float64(x)+0.5)/scale - 0.5
			dst.SetNRGBA(x, y, sample(src, b.Min.X, b.Min.Y, w, h, sx, sy))
		}
	}
	return dst
}

func sample(src image.Image, ox, oy, w, h int, fx, fy float64) color.NRGBA {
	x0, y0 := floor(fx), floor(fy)
	tx, ty := fx-float64(x0), fy-float64(y0)

	var r, g, bl, a float64
	for dy := 0; dy <= 1; dy++ {
		for dx := 0; dx <= 1; dx++ {
			px := clamp(x0+dx, 0, w-1)
			py := clamp(y0+dy, 0, h-1)
			wt := weight(dx, tx) * weight(dy, ty)
			cr, cg, cb, ca := src.At(ox+px, oy+py).RGBA()
			r += float64(cr) * wt
			g += float64(cg) * wt
			bl += float64(cb) * wt
			a += float64(ca) * wt
		}
	}
	if a == 0 {
		return color.NRGBA{}
	}
	// RGBA() дає premultiplied 16-бітні значення → переводимо в NRGBA 8 біт.
	return color.NRGBA{
		R: uint8(r/a*255 + 0.5),
		G: uint8(g/a*255 + 0.5),
		B: uint8(bl/a*255 + 0.5),
		A: uint8(a/65535*255 + 0.5),
	}
}

func weight(d int, t float64) float64 {
	if d == 0 {
		return 1 - t
	}
	return t
}

func floor(f float64) int {
	i := int(f)
	if float64(i) > f {
		i--
	}
	return i
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
