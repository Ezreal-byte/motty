package term

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	stdimage "image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"sync/atomic"
)

const (
	maxImageSequence   = 24 << 20
	maxSixelSequence   = 8 << 20
	maxImageWidth      = 4096
	maxImageHeight     = 4096
	maxImagePixels     = 4 << 20
	maxImageBytes      = maxImagePixels * 4
	maxImageMemory     = 64 << 20
	maxImages          = 64
	maxImagePlacements = 256
)

// TerminalImage is one immutable decoded image. Pix is premultiplied BGRA,
// top row first, and remains valid until the image is deleted or evicted.
type TerminalImage struct {
	Pix           []byte
	Width, Height int
	Stride        int
	ID            uint64
}

// ImagePlacement anchors one decoded image to a retained terminal line. A
// renderer finds LineID among visible Row IDs, then places the image at Column.
// Width/HeightCells take priority over Width/HeightPixels when positive.
type ImagePlacement struct {
	ID           uint64
	Image        *TerminalImage
	LineID       uint64
	Column       int
	WidthCells   int
	HeightCells  int
	WidthPixels  int
	HeightPixels int
	SourceX      int
	SourceY      int
	SourceWidth  int
	SourceHeight int
	ProtocolID   uint32
	Z            int32
}

type storedImage struct {
	image      TerminalImage
	protocolID uint32
	kitty      bool
}

type imageStore struct {
	images     []*storedImage
	placements []ImagePlacement
	bytes      int
	nextPlace  uint64
}

var nextTerminalImageID atomic.Uint64

// Images returns retained placements without allocating. The slice is invalid
// after the next Write, Reset, or Resize.
func (t *Terminal) Images() []ImagePlacement {
	if t.images == nil {
		return nil
	}
	return t.images.placements
}

// ImageRow returns the placement row of a retained line relative to the
// current view. The row may be negative or below the viewport when an image
// anchored off-screen still overlaps it.
func (t *Terminal) ImageRow(lineID uint64) (int, bool) {
	if lineID == 0 {
		return 0, false
	}
	if t.onAlt {
		for row, line := range t.alt.lines {
			if line.id == lineID {
				return row, true
			}
		}
		return 0, false
	}
	top := t.histCount - t.view
	for index := 0; index < t.histCount; index++ {
		if t.histAt(index).id == lineID {
			return index - top, true
		}
	}
	for row, line := range t.main.lines {
		if line.id == lineID {
			return t.histCount + row - top, true
		}
	}
	return 0, false
}

func (t *Terminal) acceptImage(pixels []byte, width, height int, protocolID uint32) *storedImage {
	if !validDecodedImage(pixels, width, height) {
		return nil
	}
	if t.images == nil {
		t.images = &imageStore{}
	}
	store := t.images
	if protocolID != 0 {
		store.deleteProtocol(protocolID)
	}
	for len(store.images) >= maxImages || store.bytes+len(pixels) > maxImageMemory {
		if len(store.images) == 0 {
			return nil
		}
		store.deleteAt(0)
	}
	image := &storedImage{
		image:      TerminalImage{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: nextTerminalImageID.Add(1)},
		protocolID: protocolID,
	}
	store.images = append(store.images, image)
	store.bytes += len(pixels)
	return image
}

func validDecodedImage(pixels []byte, width, height int) bool {
	if width < 1 || height < 1 || width > maxImageWidth || height > maxImageHeight || width > maxImagePixels/height {
		return false
	}
	bytes := width * height * 4
	return bytes <= maxImageBytes && len(pixels) == bytes
}

func (t *Terminal) placeImage(image *storedImage, widthCells, heightCells, widthPixels, heightPixels int, z int32) {
	t.placeImageRegion(image, 0, 0, image.image.Width, image.image.Height, widthCells, heightCells, widthPixels, heightPixels, 0, z)
}

func (t *Terminal) placeImageRegion(image *storedImage, sourceX, sourceY, sourceWidth, sourceHeight, widthCells, heightCells, widthPixels, heightPixels int, protocolPlacement uint32, z int32) {
	if image == nil || t.scr == nil || t.cy < 0 || t.cy >= len(t.scr.lines) {
		return
	}
	store := t.images
	if len(store.placements) >= maxImagePlacements {
		copy(store.placements, store.placements[1:])
		store.placements = store.placements[:len(store.placements)-1]
	}
	store.nextPlace++
	sourceX = clampInt(sourceX, 0, image.image.Width-1)
	sourceY = clampInt(sourceY, 0, image.image.Height-1)
	sourceWidth = clampInt(sourceWidth, 1, image.image.Width-sourceX)
	sourceHeight = clampInt(sourceHeight, 1, image.image.Height-sourceY)
	store.placements = append(store.placements, ImagePlacement{
		ID: store.nextPlace, Image: &image.image, LineID: t.scr.lines[t.cy].id, Column: t.cx,
		WidthCells: clampInt(widthCells, 0, maxCols), HeightCells: clampInt(heightCells, 0, maxRows),
		WidthPixels: clampInt(widthPixels, 0, maxImageWidth), HeightPixels: clampInt(heightPixels, 0, maxImageHeight),
		SourceX: sourceX, SourceY: sourceY, SourceWidth: sourceWidth, SourceHeight: sourceHeight,
		ProtocolID: protocolPlacement, Z: z,
	})
}

func (store *imageStore) deleteProtocol(id uint32) {
	if store == nil || id == 0 {
		return
	}
	for index := len(store.images) - 1; index >= 0; index-- {
		if store.images[index].protocolID == id {
			store.deleteAt(index)
		}
	}
}

func (store *imageStore) deleteAt(index int) {
	if index < 0 || index >= len(store.images) {
		return
	}
	image := store.images[index]
	store.bytes -= len(image.image.Pix)
	kept := store.placements[:0]
	for _, placement := range store.placements {
		if placement.Image != &image.image {
			kept = append(kept, placement)
		}
	}
	store.retainPlacements(kept)
	copy(store.images[index:], store.images[index+1:])
	store.images[len(store.images)-1] = nil
	store.images = store.images[:len(store.images)-1]
}

func (store *imageStore) deleteAll() {
	if store == nil {
		return
	}
	clear(store.images)
	clear(store.placements)
	store.images = store.images[:0]
	store.placements = store.placements[:0]
	store.bytes = 0
}

func (store *imageStore) retainPlacements(kept []ImagePlacement) {
	clear(store.placements[len(kept):])
	store.placements = kept
}

func (store *imageStore) dropLine(lineID uint64) {
	if store == nil || lineID == 0 {
		return
	}
	kept := store.placements[:0]
	for _, placement := range store.placements {
		if placement.LineID != lineID {
			kept = append(kept, placement)
		}
	}
	store.retainPlacements(kept)
}

func (store *imageStore) erase(lineID uint64, from, to int) {
	if store == nil || lineID == 0 || from >= to {
		return
	}
	kept := store.placements[:0]
	for _, placement := range store.placements {
		width := max(placement.WidthCells, 1)
		if placement.LineID != lineID || placement.Column+width <= from || placement.Column >= to {
			kept = append(kept, placement)
		}
	}
	store.retainPlacements(kept)
}

func (store *imageStore) insert(lineID uint64, column, count, columns int) {
	if store == nil {
		return
	}
	kept := store.placements[:0]
	for _, placement := range store.placements {
		if placement.LineID == lineID && placement.Column >= column {
			placement.Column += count
			if placement.Column >= columns {
				continue
			}
		}
		kept = append(kept, placement)
	}
	store.retainPlacements(kept)
}

func (store *imageStore) deleteCells(lineID uint64, column, count int) {
	if store == nil {
		return
	}
	kept := store.placements[:0]
	for _, placement := range store.placements {
		if placement.LineID != lineID {
			kept = append(kept, placement)
			continue
		}
		if placement.Column >= column+count {
			placement.Column -= count
			kept = append(kept, placement)
		} else if placement.Column < column {
			kept = append(kept, placement)
		}
	}
	store.retainPlacements(kept)
}

func decodeBase64Image(encoded []byte) ([]byte, error) {
	if len(encoded) == 0 || len(encoded) > maxImageSequence {
		return nil, fmt.Errorf("image payload exceeds encoded limit")
	}
	length := base64.StdEncoding.DecodedLen(len(encoded))
	if length > maxImageBytes {
		return nil, fmt.Errorf("image payload exceeds decoded limit")
	}
	decoded := make([]byte, length)
	n, err := base64.StdEncoding.Strict().Decode(decoded, encoded)
	if err != nil {
		length = base64.RawStdEncoding.DecodedLen(len(encoded))
		if length > maxImageBytes {
			return nil, fmt.Errorf("image payload exceeds decoded limit")
		}
		decoded = make([]byte, length)
		n, err = base64.RawStdEncoding.Strict().Decode(decoded, encoded)
	}
	if err != nil {
		return nil, err
	}
	return decoded[:n], nil
}

func inflateImage(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(decoded) > maxImageBytes {
		return nil, fmt.Errorf("image decompression exceeds limit")
	}
	return decoded, nil
}

func decodeImage(data []byte) ([]byte, int, int, error) {
	if len(data) == 0 || len(data) > maxImageBytes {
		return nil, 0, 0, fmt.Errorf("encoded image exceeds limit")
	}
	config, _, err := stdimage.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	if config.Width < 1 || config.Height < 1 || config.Width > maxImageWidth || config.Height > maxImageHeight || config.Width > maxImagePixels/config.Height {
		return nil, 0, 0, fmt.Errorf("decoded image dimensions exceed limit")
	}
	source, _, err := stdimage.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width != config.Width || height != config.Height || width > maxImagePixels/height {
		return nil, 0, 0, fmt.Errorf("decoded image changed dimensions")
	}
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			red, green, blue, alpha := source.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			offset := (y*width + x) * 4
			pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = byte(blue>>8), byte(green>>8), byte(red>>8), byte(alpha>>8)
		}
	}
	return pixels, width, height, nil
}
