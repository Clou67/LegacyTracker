// The "game theme": the status (XP) screen background and the game's bitmap
// fonts, read at run time from the user's own ROM. Nothing from the game is
// stored in this program; without a ROM the tracker uses its plain look.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// ---------------------------------------------------------------- NDS files

type ndsROM struct {
	data  []byte
	files map[string][2]uint32
}

func openROM(path string) (*ndsROM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 0x200 || string(data[0x0C:0x10]) != "ASSE" {
		return nil, errors.New("that isn't Spyro: Shadow Legacy (USA) - game code ASSE")
	}
	r := &ndsROM{data: data, files: map[string][2]uint32{}}
	fnt := binary.LittleEndian.Uint32(data[0x40:])
	fat := binary.LittleEndian.Uint32(data[0x48:])
	var walk func(did uint32, path string)
	walk = func(did uint32, path string) {
		e := fnt + (did&0xfff)*8
		p := fnt + binary.LittleEndian.Uint32(data[e:])
		fid := uint32(binary.LittleEndian.Uint16(data[e+4:]))
		for data[p] != 0 {
			b := data[p]
			p++
			n := string(data[p : p+uint32(b&0x7f)])
			p += uint32(b & 0x7f)
			if b&0x80 != 0 {
				walk(uint32(binary.LittleEndian.Uint16(data[p:])), path+n+"/")
				p += 2
			} else {
				s := binary.LittleEndian.Uint32(data[fat+fid*8:])
				en := binary.LittleEndian.Uint32(data[fat+fid*8+4:])
				r.files[path+n] = [2]uint32{s, en}
				fid++
			}
		}
	}
	walk(0xf000, "")
	return r, nil
}

func (r *ndsROM) file(name string) ([]byte, error) {
	se, ok := r.files[name]
	if !ok || se[1] > uint32(len(r.data)) || se[0] > se[1] {
		return nil, fmt.Errorf("%s not found in the ROM", name)
	}
	return r.data[se[0]:se[1]], nil
}

// ---------------------------------------------------------------- unpacking

// The game's own compressor (0x02000df4); port of N_Unpack_Data in
// projShadow pc/src/game_asm.c.
type bitsIn struct {
	src  []byte
	p    int
	word uint32
	left int
}

func (b *bitsIn) bit() uint32 {
	if b.left == 0 {
		b.word = binary.LittleEndian.Uint32(b.src[b.p:])
		b.p += 4
		b.left = 32
	}
	v := b.word >> 31
	b.word <<= 1
	b.left--
	return v
}
func (b *bitsIn) read(n uint32) uint32 {
	var v uint32
	for ; n > 0; n-- {
		v = v<<1 | b.bit()
	}
	return v
}
func (b *bitsIn) gamma() uint32 {
	k := uint32(0)
	for k < 7 && b.bit() == 1 {
		k++
	}
	return 1<<k | b.read(k)
}

func unpackData(src []byte, max int) (out []byte, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("bad compressed data")
		}
	}()
	head := binary.LittleEndian.Uint32(src)
	tsize, esc := head&0xff, head>>8&0xff
	l, h := head>>16&0xff, head>>24
	table := make([]byte, 0x20)
	copy(table, src[4:4+min(int(tsize), 0x20)])
	b := &bitsIn{src: src, p: 4 + int((tsize+3)&^3)}
	for len(out) < max {
		var v uint32
		if h != 0 {
			v = b.read(h)
		}
		if v != esc {
			out = append(out, byte(v<<(8-h)|b.read(8-h)))
			continue
		}
		g := b.gamma()
		var dist, n uint32
		switch {
		case g >= 2:
			d := b.gamma()
			if d == 0xff {
				return out, nil
			}
			dist = d - 1
			if l != 0 {
				dist = dist<<l | b.read(l)
			}
			n = g + 1
		case b.bit() == 0:
			dist, n = 0, 2
		case b.bit() == 0:
			old := esc
			esc = 0
			if h != 0 {
				esc = b.read(h)
			}
			out = append(out, byte(old<<(8-h)|b.read(8-h)))
			continue
		default:
			cnt, hi := b.gamma(), uint32(0)
			if cnt >= 0x80 {
				cnt = (cnt<<1 | b.bit()) & 0xff
				hi = b.gamma() - 1
			}
			c := b.gamma()
			var cb byte
			if c < 0x20 {
				cb = table[c-1]
			} else {
				cb = byte((c<<3 | b.read(3)) & 0xff)
			}
			for k := cnt + 1 + hi<<8; k > 0; k-- {
				out = append(out, cb)
			}
			continue
		}
		dist = (dist<<8 | b.read(8)) + 1
		for ; n > 0; n-- {
			out = append(out, out[len(out)-int(dist)])
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- images

// image is a plain 32-bit 0x00RRGGBB pixel buffer.
type image struct {
	w, h int
	pix  []uint32
}

func bgr555(c uint16) uint32 {
	r, g, b := uint32(c&31), uint32(c>>5&31), uint32(c>>10&31)
	return (r<<3|r>>2)<<16 | (g<<3|g>>2)<<8 | (b<<3 | b>>2)
}

// decodeBG turns a BG blob (BG_LoadBlob in projShadow BG2.c) into an image.
func decodeBG(data []byte) (*image, error) {
	f := binary.LittleEndian.Uint16(data)
	p := 2
	var pal [256]uint32
	switch {
	case f&1 != 0:
		for i := 0; i < 256; i++ {
			pal[i] = bgr555(binary.LittleEndian.Uint16(data[p+2*i:]))
		}
		p += 0x200
	case f&2 != 0:
		for i := 0; i < 16; i++ {
			pal[i] = bgr555(binary.LittleEndian.Uint16(data[p+2*i:]))
		}
		p += 0x20
	}
	if f&4 != 0 {
		p += 2 + int(binary.LittleEndian.Uint16(data[p:]))
	}
	if f&8 == 0 || f&16 == 0 {
		return nil, errors.New("background has no map or tiles")
	}
	mw, mh := int(binary.LittleEndian.Uint16(data[p:])), int(binary.LittleEndian.Uint16(data[p+2:]))
	n := mw * mh
	half := f&0x100 != 0
	mapAt := p + 4
	sk := n
	if sk&1 != 0 {
		sk++
	}
	if half {
		sk /= 2
	}
	p = mapAt + sk*2
	size := int(binary.LittleEndian.Uint16(data[p:]))
	src := data[p+2:]
	bpp8 := f&0x80 != 0
	tileBytes := 32
	if bpp8 {
		tileBytes = 64
	}
	var tiles []byte
	var err error
	switch f >> 5 & 3 {
	case 0:
		tiles = src[:size]
	case 3:
		tiles, err = unpackData(src, 1<<20)
	default:
		return nil, errors.New("unsupported background compression")
	}
	if err != nil {
		return nil, err
	}
	img := &image{w: mw * 8, h: mh * 8, pix: make([]uint32, mw*mh*64)}
	for ty := 0; ty < mh; ty++ {
		for tx := 0; tx < mw; tx++ {
			var e uint16
			if half {
				e = uint16(data[mapAt+ty*mw+tx])
			} else {
				e = binary.LittleEndian.Uint16(data[mapAt+2*(ty*mw+tx):])
			}
			t, hf, vf, pb := int(e&0x3ff), e&0x400 != 0, e&0x800 != 0, int(e>>12)
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					sx, sy := x, y
					if hf {
						sx = 7 - x
					}
					if vf {
						sy = 7 - y
					}
					var ci int
					if bpp8 {
						o := t*tileBytes + sy*8 + sx
						if o < len(tiles) {
							ci = int(tiles[o])
						}
					} else {
						o := t*tileBytes + sy*4 + sx/2
						if o < len(tiles) {
							ci = int(tiles[o]>>(uint(sx&1)*4)) & 15
							if ci != 0 {
								ci += pb * 16
							}
						}
					}
					img.pix[(ty*8+y)*img.w+tx*8+x] = pal[ci&255]
				}
			}
		}
	}
	return img, nil
}

// ---------------------------------------------------------------- fonts

// pixelFont is one of the game's bitmap fonts (Font_Load in projShadow Font.c):
// u16 first/last character, u8 height, then big-endian offsets of the glyph
// offset table, the widths and the 4-bit bitmaps (column by column).
type pixelFont struct {
	first, last int
	height      int
	offs        []byte
	widths      []byte
	bits        []byte
	maxV        int
}

func parseFont(h []byte) (*pixelFont, error) {
	if len(h) < 0x10 {
		return nil, errors.New("bad font")
	}
	f := &pixelFont{first: int(binary.LittleEndian.Uint16(h)), last: int(binary.LittleEndian.Uint16(h[2:])), height: int(h[4])}
	o := func(i int) int { return int(h[i])<<8 | int(h[i+1]) }
	f.offs, f.widths, f.bits = h[o(6):], h[o(0xa):], h[o(0xe):]
	if f.last < f.first || f.height == 0 || f.height > 32 {
		return nil, errors.New("bad font")
	}
	for _, b := range f.bits {
		if v := int(b & 15); v > f.maxV {
			f.maxV = v
		}
		if v := int(b >> 4); v > f.maxV {
			f.maxV = v
		}
	}
	if f.maxV == 0 {
		f.maxV = 1
	}
	return f, nil
}

func (f *pixelFont) glyph(c rune) int {
	g := int(c) - f.first
	if g < 0 || g > f.last-f.first {
		return 1
	}
	return g
}

func (f *pixelFont) width(c rune) int {
	g := f.glyph(c)
	if g >= len(f.widths) {
		return 0
	}
	return int(f.widths[g])
}

func (f *pixelFont) textWidth(s string) int {
	w := 0
	for _, c := range s {
		w += f.width(c)
	}
	return w
}

// draw writes s at (x, y) (top of the glyphs) into img in colour ink, each
// font pixel as a scale x scale block. Pixel values 1..maxV shade the ink.
func (f *pixelFont) draw(img *image, x, y int, s string, ink uint32, scale int) {
	for _, c := range s {
		g := f.glyph(c)
		if g*2+1 >= len(f.offs) || g >= len(f.widths) {
			continue
		}
		bo := int(binary.LittleEndian.Uint16(f.offs[g*2:]))
		w := int(f.widths[g])
		sh := 0
		for col := 0; col < w; col++ {
			for row := 0; row < f.height; row++ {
				if sh == 8 {
					bo++
					sh = 0
				}
				if bo >= len(f.bits) {
					break
				}
				v := int(f.bits[bo]>>uint(sh)) & 15
				sh += 4
				if v == 0 {
					continue
				}
				// higher values are the bright face, lower ones the outline
				col32 := shade(ink, v, f.maxV)
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						img.set(x+(col)*scale+dx, y+row*scale+dy, col32)
					}
				}
			}
		}
		x += w * scale
	}
}

func shade(ink uint32, v, maxV int) uint32 {
	if maxV <= 1 {
		return ink
	}
	k := 0.35 + 0.65*float64(v)/float64(maxV)
	r := float64(ink>>16&255) * k
	g := float64(ink>>8&255) * k
	b := float64(ink&255) * k
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

func (img *image) set(x, y int, c uint32) {
	if x >= 0 && y >= 0 && x < img.w && y < img.h {
		img.pix[y*img.w+x] = c
	}
}

// ---------------------------------------------------------------- theme

type gameTheme struct {
	bg         *image
	body, bold *pixelFont
	small      *pixelFont
}

// loadTheme reads the XP screen background and the fonts from the ROM.
func loadTheme(romPath string) (*gameTheme, error) {
	r, err := openROM(romPath)
	if err != nil {
		return nil, err
	}
	t := &gameTheme{}
	bg, err := r.file("B_Stats_BKG.bin")
	if err != nil {
		return nil, err
	}
	if t.bg, err = decodeBG(bg); err != nil {
		return nil, fmt.Errorf("background: %w", err)
	}
	for _, f := range []struct {
		name string
		dst  **pixelFont
	}{{"hachipochi12p.bin", &t.body}, {"GoblinBold15_White.bin", &t.bold}, {"drift8psa.bin", &t.small}} {
		d, err := r.file(f.name)
		if err != nil {
			return nil, err
		}
		if *f.dst, err = parseFont(d); err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return t, nil
}
