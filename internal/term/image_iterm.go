package term

import (
	"bytes"
	"strconv"
	"strings"
)

type itermDimension struct {
	value int
	cells bool
}

func (t *Terminal) itermImage(payload []byte) {
	if !bytes.HasPrefix(payload, []byte("File=")) {
		return
	}
	separator := bytes.IndexByte(payload, ':')
	if separator < 0 {
		return
	}
	metadata, encoded := payload[len("File="):separator], payload[separator+1:]
	inline := false
	preserveAspect := true
	declaredSize := -1
	var width, height itermDimension
	for len(metadata) > 0 {
		field := metadata
		if semicolon := bytes.IndexByte(metadata, ';'); semicolon >= 0 {
			field, metadata = metadata[:semicolon], metadata[semicolon+1:]
		} else {
			metadata = nil
		}
		key, value, ok := bytes.Cut(field, []byte{'='})
		if !ok {
			continue
		}
		switch string(key) {
		case "inline":
			inline = bytes.Equal(value, []byte{'1'})
		case "preserveAspectRatio":
			preserveAspect = !bytes.Equal(value, []byte{'0'})
		case "size":
			if parsed, err := strconv.Atoi(string(value)); err == nil && parsed >= 0 && parsed <= maxImageBytes {
				declaredSize = parsed
			} else {
				return
			}
		case "width":
			width = parseItermDimension(value)
		case "height":
			height = parseItermDimension(value)
		}
	}
	if !inline {
		return
	}
	compressed, err := decodeBase64Image(encoded)
	if err != nil || declaredSize >= 0 && declaredSize != len(compressed) {
		return
	}
	pixels, pixelWidth, pixelHeight, err := decodeImage(compressed)
	if err != nil {
		return
	}
	image := t.acceptImage(pixels, pixelWidth, pixelHeight, 0)
	if image == nil {
		return
	}
	widthCells, heightCells, widthPixels, heightPixels := 0, 0, 0, 0
	if width.cells {
		widthCells = width.value
	} else {
		widthPixels = width.value
	}
	if height.cells {
		heightCells = height.value
	} else {
		heightPixels = height.value
	}
	if preserveAspect {
		switch {
		case widthCells > 0 && heightCells == 0 && heightPixels == 0:
			heightCells = 0
		case heightCells > 0 && widthCells == 0 && widthPixels == 0:
			widthCells = 0
		case widthPixels > 0 && heightPixels == 0 && heightCells == 0:
			heightPixels = max(1, widthPixels*pixelHeight/pixelWidth)
		case heightPixels > 0 && widthPixels == 0 && widthCells == 0:
			widthPixels = max(1, heightPixels*pixelWidth/pixelHeight)
		}
	}
	t.placeImage(image, widthCells, heightCells, widthPixels, heightPixels, 0)
}

func parseItermDimension(value []byte) itermDimension {
	text := string(value)
	if text == "" || text == "auto" || strings.HasSuffix(text, "%") {
		return itermDimension{}
	}
	cells := true
	if strings.HasSuffix(text, "px") {
		cells = false
		text = strings.TrimSuffix(text, "px")
	}
	number, err := strconv.Atoi(text)
	if err != nil || number < 1 {
		return itermDimension{}
	}
	if cells {
		number = min(number, maxCols)
	} else {
		number = min(number, maxImageWidth)
	}
	return itermDimension{value: number, cells: cells}
}
