package runner

import (
	"bytes"
	"image/png"
	"testing"
)

func TestImageFixtureIsValidAndDeterministic(test *testing.T) {
	content := ImageFixture(test)
	picture, err := png.Decode(bytes.NewReader(content))
	if err != nil || picture.Bounds().Dx() != 3 || picture.Bounds().Dy() != 2 {
		test.Fatal("image fixture must be a valid 3x2 PNG")
	}
	if !bytes.Equal(content, ImageFixture(test)) {
		test.Fatal("image fixture encoding must be deterministic")
	}
}
