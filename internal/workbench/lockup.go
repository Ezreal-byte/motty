package workbench

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"sync"

	"github.com/vivaui/viva/ui"
	"motty/assets"
	"motty/internal/buildinfo"
)

const lockupHeight = 36

// lockupBitmap decodes one embedded lockup picture to the premultiplied BGRA the
// canvas draws. The pictures are fixed and embedded, so a bad one is a build error.
func lockupBitmap(data []byte, id uint64) func() ui.Bitmap {
	return sync.OnceValue(func() ui.Bitmap {
		picture, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			panic("workbench: the embedded logo is not a PNG: " + err.Error())
		}
		pix, width, height, err := scaleToBGRA(picture, 1<<14)
		if err != nil {
			panic("workbench: the embedded logo is unusable: " + err.Error())
		}
		return ui.Bitmap{Pix: pix, Width: width, Height: height, Stride: width * 4, ID: id}
	})
}

// lockupMark crops the square mark from a lockup picture. A development build
// uses it beside its build-defined title; a release build keeps the full wordmark.
func lockupMark(data []byte, id uint64) func() ui.Bitmap {
	return sync.OnceValue(func() ui.Bitmap {
		picture, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			panic("workbench: the embedded logo is not a PNG: " + err.Error())
		}
		side := picture.Bounds().Dy()
		mark := image.NewRGBA(image.Rect(0, 0, side, side))
		draw.Draw(mark, mark.Bounds(), picture, picture.Bounds().Min, draw.Src)
		pix, width, height, err := scaleToBGRA(mark, side)
		if err != nil {
			panic("workbench: the embedded logo is unusable: " + err.Error())
		}
		return ui.Bitmap{Pix: pix, Width: width, Height: height, Stride: width * 4, ID: id}
	})
}

var (
	lockupDark      = lockupBitmap(assets.LockupDark, 1<<61|1)
	lockupLight     = lockupBitmap(assets.LockupLight, 1<<61|2)
	lockupMarkDark  = lockupMark(assets.LockupDark, 1<<61|3)
	lockupMarkLight = lockupMark(assets.LockupLight, 1<<61|4)
)

// paintLockup draws the logo with the name beside it into the slot the sidebar
// header kept for it.
func (state *State) paintLockup(canvas *ui.Canvas, bounds ui.Rect) {
	if buildinfo.Development {
		mark := lockupMarkLight()
		if state.pal.dark {
			mark = lockupMarkDark()
		}
		canvas.DrawBitmap(mark, ui.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Height, Height: bounds.Height})
		canvas.Label(buildinfo.ProductName, ui.Rect{
			X: bounds.X + bounds.Height + 8, Y: bounds.Y + 5,
			Width: bounds.Width - bounds.Height - 8, Height: bounds.Height - 8,
		}, ui.TextStyle{Size: 18, Weight: ui.WeightSemibold, Color: state.pal.text})
		return
	}
	bitmap := lockupLight()
	if state.pal.dark {
		bitmap = lockupDark()
	}
	canvas.DrawBitmap(bitmap, bounds)
}
