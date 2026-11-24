package home

import (
	"bytes"
	"cmp"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"slices"
)

// Colors finds up to three colours that stand for a picture, the most telling first. It gives
// none for formats it cannot read.
func Colors(pic []byte) []string {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(pic))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(pic))
	if err != nil {
		return nil
	}
	type bin struct{ r, g, b, n, w int }
	bins := map[int]*bin{}
	box := img.Bounds()
	step := max(1, max(box.Dx(), box.Dy())/80)
	for y := box.Min.Y; y < box.Max.Y; y += step {
		for x := box.Min.X; x < box.Max.X; x += step {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			if ca < 0x8000 {
				continue
			}
			r, g, b := int(cr>>8), int(cg>>8), int(cb>>8)
			k := r>>5<<10 | g>>5<<5 | b>>5
			e := bins[k]
			if e == nil {
				e = &bin{}
				bins[k] = e
			}
			hi, lo := max(r, g, b), min(r, g, b)
			e.r, e.g, e.b, e.n = e.r+r, e.g+g, e.b+b, e.n+1
			e.w += 12 + (hi - lo) // colour counts for more than grey
			if hi > 40 && lo < 225 {
				e.w += 30
			}
		}
	}
	all := make([]*bin, 0, len(bins))
	for _, e := range bins {
		e.r, e.g, e.b = e.r/e.n, e.g/e.n, e.b/e.n
		all = append(all, e)
	}
	slices.SortFunc(all, func(a, b *bin) int {
		if c := cmp.Compare(b.w, a.w); c != 0 {
			return c
		}
		return cmp.Compare(a.r<<16|a.g<<8|a.b, b.r<<16|b.g<<8|b.b)
	})
	var out []string
	var taken []*bin
	for _, e := range all {
		near := slices.ContainsFunc(taken, func(o *bin) bool {
			dr, dg, db := e.r-o.r, e.g-o.g, e.b-o.b
			return dr*dr+dg*dg+db*db < 70*70
		})
		if !near {
			taken = append(taken, e)
			out = append(out, fmt.Sprintf("#%02x%02x%02x", e.r, e.g, e.b))
		}
		if len(out) == 3 {
			break
		}
	}
	return out
}
