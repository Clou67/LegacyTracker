package main

import (
	"encoding/binary"
	img "image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// fakeFont builds an original stand-in bitmap font (hollow boxes), in the
// game's font format, so the themed layout can be tested without the ROM.
func fakeFont(height, width int) *pixelFont {
	n := 0x7f - 0x20
	f := &pixelFont{first: 0x20, last: 0x7e, height: height, maxV: 3}
	perCol := (height + 1) / 2
	bits := []byte{}
	offs := make([]byte, n*2)
	widths := make([]byte, n)
	for g := 0; g < n; g++ {
		binary.LittleEndian.PutUint16(offs[g*2:], uint16(len(bits)))
		w := width
		if g == 0 {
			w = width / 2 // space
		}
		widths[g] = byte(w)
		// one stream of 4-bit pixels, column by column
		px := []byte{}
		for col := 0; col < w; col++ {
			for row := 0; row < height; row++ {
				v := byte(0)
				if g != 0 && col < w-1 && row > 1 && (col == 0 || col == w-2 || row == 2 || row == height-1) {
					v = 3
				}
				px = append(px, v)
			}
		}
		for i := 0; i < len(px); i += 2 {
			b := px[i]
			if i+1 < len(px) {
				b |= px[i+1] << 4
			}
			bits = append(bits, b)
		}
		_ = perCol
	}
	f.offs, f.widths, f.bits = offs, widths, bits
	return f
}

func fakeTheme() *gameTheme {
	bg := &image{w: 256, h: 192, pix: make([]uint32, 256*192)}
	for y := 0; y < 192; y++ {
		for x := 0; x < 256; x++ {
			bg.pix[y*256+x] = uint32(40+x/4)<<16 | uint32(30+y/4)<<8 | 90
		}
	}
	return &gameTheme{bg: bg, body: fakeFont(12, 7), bold: fakeFont(14, 9), small: fakeFont(8, 5)}
}

func TestThemedLayout(t *testing.T) {
	path := os.Getenv("SSL_SAVE")
	if path == "" {
		t.Skip("set SSL_SAVE")
	}
	snap, _, err := ReadSave(path, -1)
	if err != nil {
		t.Fatal(err)
	}
	r := Evaluate(snap, nil, "t")
	c := newFBCanvas(1180, 1000, fakeTheme(), 0)
	draw(c, r, "Save file: test.sav (slot 1)", false, false, 1180)
	out := img.NewRGBA(img.Rect(0, 0, c.img.w, c.img.h))
	for i, p := range c.img.pix {
		out.Set(i%c.img.w, i/c.img.w, color.RGBA{uint8(p >> 16), uint8(p >> 8), uint8(p), 255})
	}
	if o := os.Getenv("SSL_OUT"); o != "" {
		f, _ := os.Create(o + "themed_fake.png")
		png.Encode(f, out)
		f.Close()
	}
}
