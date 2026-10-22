// Planar codec decompression for 32 bpp bitmaps (MS-RDPEGDI 2.2.2.5).
//
// 32 bpp compressed bitmaps use the RDP 6.0 Bitmap Compression format
// instead of Interleaved RLE. Pixel data is split into separate A/R/G/B
// planes, each independently compressed. Each control byte encodes a
// run length (low nibble) and raw byte count (high nibble). The first
// scanline contains absolute values; subsequent scanlines use zigzag-
// encoded deltas from the row above.
package rle

import (
	"errors"
	"runtime"
	"sync"
)

// Planar format header flags (MS-RDPEGDI 2.2.2.5.1).
const (
	planarCLLMask = 0x07 // bits 0-2: compression level (lossy)
	planarRLE     = 0x10 // bit 4: per-plane RLE
	planarNA      = 0x20 // bit 5: no alpha plane
)

var (
	errPlanarShort   = errors.New("planar: data too short")
	errPlanarOverrun = errors.New("planar: scanline overrun")
)

// DecompressPlanar decompresses RDP 6.0 planar-coded bitmap data into
// bottom-up RGBA (4 bytes per pixel) output. R/G/B planes are decoded
// directly into the output buffer at stride-4 intervals, eliminating
// the intermediate planes buffer. dst is a caller-provided reusable
// buffer (may be nil; will be allocated/grown as needed).
func DecompressPlanar(dst []byte, width, height int, src []byte) ([]byte, error) {
	if len(src) < 1 {
		return nil, errPlanarShort
	}

	hdr := src[0]
	cll := int(hdr & planarCLLMask)
	rle := hdr&planarRLE != 0
	noAlpha := hdr&planarNA != 0
	off := 1

	planeSize := width * height
	dstLen := planeSize * 4
	if cap(dst) < dstLen {
		dst = make([]byte, dstLen)
	} else {
		dst = dst[:dstLen]
	}

	if rle {
		// Plane order: Alpha (if present), Red, Green, Blue.
		if !noAlpha {
			// Skip alpha plane — we force 0xFF below.
			consumed, err := skipPlaneRLE(src[off:], width, height)
			if err != nil {
				return nil, err
			}
			off += consumed
		}
		// Find each plane's bounds first, so the three independent RLE decodes
		// can run together. The scan is cheap compared to writing the pixels.
		starts := [4]int{}
		starts[0] = off
		for plane := 0; plane < 3; plane++ {
			consumed, err := skipPlaneRLE(src[starts[plane]:], width, height)
			if err != nil {
				return nil, err
			}
			starts[plane+1] = starts[plane] + consumed
		}
		var wg sync.WaitGroup
		var planeErrs [3]error
		for plane, planeOff := range [3]int{0, 1, 2} {
			wg.Add(1)
			go func(plane, planeOff int) {
				defer wg.Done()
				_, planeErrs[plane] = decodePlaneRLEStrided(dst[planeOff:], src[starts[plane]:starts[plane+1]], width, height, 4)
			}(plane, planeOff)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Desktop bitmaps are always opaque; servers commonly send alpha=0
			// which would make pixels transparent.
			for i := 3; i < dstLen; i += 4 {
				dst[i] = 0xFF
			}
		}()
		wg.Wait()
		for _, err := range planeErrs {
			if err != nil {
				return nil, err
			}
		}
		off = starts[3]
	} else {
		// Uncompressed planes.
		if !noAlpha {
			// Skip alpha plane raw bytes.
			if off+planeSize > len(src) {
				return nil, errPlanarShort
			}
			off += planeSize
		}
		// Interleave the three contiguous planes in one pass. Writing RGBA
		// four bytes at a time is much friendlier to the cache than three
		// separate strided scatter passes.
		if off+3*planeSize > len(src) {
			return nil, errPlanarShort
		}
		red := src[off : off+planeSize]
		green := src[off+planeSize : off+2*planeSize]
		blue := src[off+2*planeSize : off+3*planeSize]
		interleavePlanes(dst, red, green, blue)
		off += 3 * planeSize
	}

	// Apply CLL (compression level lossy): left-shift R, G, B channels.
	if cll > 0 {
		for i := 0; i < dstLen; i += 4 {
			dst[i] <<= cll
			dst[i+1] <<= cll
			dst[i+2] <<= cll
		}
	}

	return dst, nil
}

// decodePlaneRLEStrided decodes a single RLE-compressed plane directly into
// an RGBA output buffer. Decoded bytes are written at dst[0], dst[stride],
// dst[2*stride], etc. This fuses plane decoding with interleaving, avoiding
// an intermediate buffer and a separate scatter pass.
// Returns the number of source bytes consumed.
func decodePlaneRLEStrided(dst []byte, src []byte, width, height, stride int) (int, error) {
	srcOff := 0
	dstOff := 0
	rowStride := width * stride

	// First scanline: absolute values (no delta, no prevRow lookup).
	if height > 0 {
		var pixel byte
		for x := 0; x < width; {
			if srcOff >= len(src) {
				return 0, errPlanarShort
			}
			ctrl := src[srcOff]
			srcOff++

			nRun := int(ctrl & 0x0F)
			cRaw := int(ctrl >> 4)
			if nRun == 1 {
				nRun = cRaw + 16
				cRaw = 0
			} else if nRun == 2 {
				nRun = cRaw + 32
				cRaw = 0
			}

			total := nRun + cRaw
			if total == 0 {
				continue // Motty fix: a zero-length run indexed dst[-stride] (found by fuzzing)
			}
			if x+total > width {
				return 0, errPlanarOverrun
			}
			if srcOff+cRaw > len(src) {
				return 0, errPlanarShort
			}

			// BCE: prove dst write range is in-bounds.
			_ = dst[dstOff+(total-1)*stride]

			for range cRaw {
				pixel = src[srcOff]
				srcOff++
				dst[dstOff] = pixel
				dstOff += stride
			}
			for range nRun {
				dst[dstOff] = pixel
				dstOff += stride
			}
			x += total
		}
	}

	// Subsequent scanlines: zigzag-encoded deltas from the row above.
	for y := 1; y < height; y++ {
		var delta byte

		for x := 0; x < width; {
			if srcOff >= len(src) {
				return 0, errPlanarShort
			}
			ctrl := src[srcOff]
			srcOff++

			nRun := int(ctrl & 0x0F)
			cRaw := int(ctrl >> 4)
			if nRun == 1 {
				nRun = cRaw + 16
				cRaw = 0
			} else if nRun == 2 {
				nRun = cRaw + 32
				cRaw = 0
			}

			total := nRun + cRaw
			if total == 0 {
				continue // Motty fix (fuzzing): dst[-stride]
			}
			if x+total > width {
				return 0, errPlanarOverrun
			}
			if srcOff+cRaw > len(src) {
				return 0, errPlanarShort
			}

			// BCE: prove dst and prevRow ranges are in-bounds.
			_ = dst[dstOff+(total-1)*stride]
			_ = dst[dstOff+(total-1)*stride-rowStride]

			for range cRaw {
				dv := src[srcOff]
				srcOff++
				delta = dv >> 1
				if dv&1 != 0 {
					delta = ^delta
				}
				dst[dstOff] = dst[dstOff-rowStride] + delta
				dstOff += stride
			}
			for range nRun {
				dst[dstOff] = dst[dstOff-rowStride] + delta
				dstOff += stride
			}
			x += total
		}
	}

	return srcOff, nil
}

// skipPlaneRLE parses a single RLE-compressed plane without writing output,
// returning the number of source bytes consumed. Used to skip the alpha plane.
func skipPlaneRLE(src []byte, width, height int) (int, error) {
	srcOff := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; {
			if srcOff >= len(src) {
				return 0, errPlanarShort
			}
			ctrl := src[srcOff]
			srcOff++

			nRun := int(ctrl & 0x0F)
			cRaw := int(ctrl >> 4)
			if nRun == 1 {
				nRun = cRaw + 16
				cRaw = 0
			} else if nRun == 2 {
				nRun = cRaw + 32
				cRaw = 0
			}

			total := nRun + cRaw
			if x+total > width {
				return 0, errPlanarOverrun
			}
			if srcOff+cRaw > len(src) {
				return 0, errPlanarShort
			}
			srcOff += cRaw
			x += total
		}
	}
	return srcOff, nil
}

// decodePlaneRLE decodes a single RLE-compressed plane using the RDP 6.0
// planar codec format. Each control byte packs a run length (low nibble)
// and raw byte count (high nibble). The first scanline has absolute values;
// subsequent scanlines use zigzag-encoded deltas from the row above.
// Returns the number of source bytes consumed.
func decodePlaneRLE(dst []byte, src []byte, width, height int) (int, error) {
	return decodePlaneRLEStrided(dst, src, width, height, 1)
}

// planarParallelPixels is where interleaving moves to a few goroutines. A
// 1080p frame is 2M pixels and dominates the RDP decode; smaller bitmaps and
// tile updates stay on the calling goroutine.
const planarParallelPixels = 1 << 20

// interleavePlanes writes R/G/B planes into RGBA output. The planes are
// independent, so a large frame is split into at most four chunks; the write
// is the expensive part and parallel chunks use otherwise idle cores.
func interleavePlanes(dst, red, green, blue []byte) {
	workers := runtime.GOMAXPROCS(0)
	if len(red) < planarParallelPixels || workers < 2 {
		interleaveSequential(dst, red, green, blue)
		return
	}
	if workers > 8 {
		workers = 8
	}
	chunk := (len(red) + workers - 1) / workers
	var wg sync.WaitGroup
	for start := 0; start < len(red); start += chunk {
		end := min(start+chunk, len(red))
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			interleaveSequential(dst[start*4:end*4], red[start:end], green[start:end], blue[start:end])
		}(start, end)
	}
	wg.Wait()
}

func interleaveSequential(dst, red, green, blue []byte) {
	for i, r := range red {
		j := i * 4
		dst[j] = r
		dst[j+1] = green[i]
		dst[j+2] = blue[i]
		dst[j+3] = 0xFF
	}
}
