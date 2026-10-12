package fixtures

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func PNG() ([]byte, error) {
	picture := image.NewRGBA(image.Rect(0, 0, 3, 2))
	picture.Set(1, 1, color.RGBA{R: 64, G: 128, B: 192, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
