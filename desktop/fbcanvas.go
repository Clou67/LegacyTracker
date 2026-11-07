// A canvas that draws into a pixel buffer: used for the game theme, where the
// background and the fonts come from the user's ROM.
package main

import "strings"

type fbCanvas struct {
	img   *image
	th    *gameTheme
	dy    int
	alpha float64 // opacity of the panels over the background
}

func newFBCanvas(w, h int, th *gameTheme, dy int) *fbCanvas {
	c := &fbCanvas{img: &image{w: w, h: h, pix: make([]uint32, w*h)}, th: th, dy: dy, alpha: 0.78}
	c.background()
	return c
}

// background covers the buffer with the theme's background, scaled with
// nearest-neighbour to fill it (cropping the overflow), and fixed in place.
func (c *fbCanvas) background() {
	bg, img := c.th.bg, c.img
	sx := float64(img.w) / float64(bg.w)
	sy := float64(img.h) / float64(bg.h)
	s := sx
	if sy > s {
		s = sy
	}
	ox := (float64(img.w) - float64(bg.w)*s) / 2
	oy := (float64(img.h) - float64(bg.h)*s) / 2
	for y := 0; y < img.h; y++ {
		by := int((float64(y) - oy) / s)
		if by < 0 {
			by = 0
		}
		if by >= bg.h {
			by = bg.h - 1
		}
		row := bg.pix[by*bg.w:]
		for x := 0; x < img.w; x++ {
			bx := int((float64(x) - ox) / s)
			if bx < 0 {
				bx = 0
			}
			if bx >= bg.w {
				bx = bg.w - 1
			}
			img.pix[y*img.w+x] = row[bx]
		}
	}
}

func rgbOf(c uintptr) uint32 { // GDI COLORREF (0x00BBGGRR) -> 0x00RRGGBB
	return uint32(c&0xff)<<16 | uint32(c>>8&0xff)<<8 | uint32(c>>16&0xff)
}

func blend(dst, src uint32, a float64) uint32 {
	mix := func(d, s uint32) uint32 { return uint32(float64(d)*(1-a) + float64(s)*a) }
	return mix(dst>>16&255, src>>16&255)<<16 | mix(dst>>8&255, src>>8&255)<<8 | mix(dst&255, src&255)
}

func (c *fbCanvas) rect(x, y, w, h int, col uint32, a float64) {
	y -= c.dy
	for yy := y; yy < y+h; yy++ {
		if yy < 0 || yy >= c.img.h {
			continue
		}
		for xx := x; xx < x+w; xx++ {
			if xx < 0 || xx >= c.img.w {
				continue
			}
			i := yy*c.img.w + xx
			if a >= 1 {
				c.img.pix[i] = col
			} else {
				c.img.pix[i] = blend(c.img.pix[i], col, a)
			}
		}
	}
}

func (c *fbCanvas) fill(x, y, w, h int, col uintptr) {
	switch col {
	case cBg:
		return // the background picture shows instead
	case cPanel:
		c.rect(x, y, w, h, rgbOf(col), c.alpha)
	default:
		c.rect(x, y, w, h, rgbOf(col), 1)
	}
}

func (c *fbCanvas) frame(x, y, w, h int, col uintptr) {
	k := rgbOf(col)
	c.rect(x, y, w, 1, k, 1)
	c.rect(x, y+h-1, w, 1, k, 1)
	c.rect(x, y, 1, h, k, 1)
	c.rect(x+w-1, y, 1, h, k, 1)
}

// font picks the game font and scale for a layout text size
func (c *fbCanvas) font(size int, bold bool) (*pixelFont, int) {
	switch {
	case size >= 18:
		return c.th.bold, 2
	case bold && size >= 13:
		return c.th.bold, 1
	case size <= 9:
		return c.th.small, 1
	}
	return c.th.body, 1
}

func (c *fbCanvas) text(x, y, w int, s string, col uintptr, size int, bold bool, flags uintptr) int {
	f, sc := c.font(size, bold)
	lineH := f.height*sc + 3
	ink := rgbOf(col)
	measure := func(t string) int { return f.textWidth(t) * sc }
	// the box the layout reserved is size+6 tall: centre the glyphs in it
	top := y + (size+6-f.height*sc)/2
	if flags&dtWordBrk != 0 {
		var lines []string
		cur := ""
		for _, word := range strings.Fields(s) {
			try := word
			if cur != "" {
				try = cur + " " + word
			}
			if measure(try) > w && cur != "" {
				lines = append(lines, cur)
				cur = word
			} else {
				cur = try
			}
		}
		lines = append(lines, cur)
		for i, l := range lines {
			f.draw(c.img, x, y+i*lineH-c.dy, l, ink, sc)
		}
		return len(lines) * lineH
	}
	if measure(s) > w {
		for len(s) > 0 && measure(s+"...") > w {
			r := []rune(s)
			s = string(r[:len(r)-1])
		}
		s += "..."
	}
	if flags&dtRight != 0 {
		x += w - measure(s)
	}
	f.draw(c.img, x, top-c.dy, s, ink, sc)
	return size + 6
}
