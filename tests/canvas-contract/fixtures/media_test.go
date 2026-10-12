package fixtures

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"net/http"
	"testing"
)

func TestImportMediaFixtures(test *testing.T) {
	for _, path := range []string{"/contract/image.png", "/contract/audio.wav", "/contract/video.mp4"} {
		content, mime, err := ImportMedia(path)
		if err != nil || http.DetectContentType(content) != mime {
			test.Fatalf("fixture %s does not contain its declared MIME: %v", path, err)
		}
		repeated, _, err := ImportMedia(path)
		if err != nil || !bytes.Equal(content, repeated) {
			test.Fatal("media fixtures must be reproducible without external tools")
		}
	}
	picture, _ := PNG()
	image, err := png.DecodeConfig(bytes.NewReader(picture))
	if err != nil || image.Width != 3 || image.Height != 2 {
		test.Fatal("image dimensions")
	}
	audio := WAV()
	if string(audio[:4]) != "RIFF" || string(audio[8:16]) != "WAVEfmt " || binary.LittleEndian.Uint32(audio[4:8]) != uint32(len(audio)-8) || binary.LittleEndian.Uint32(audio[24:28]) != 8000 || binary.LittleEndian.Uint32(audio[40:44]) != 160 {
		test.Fatal("PCM WAV duration/size/header")
	}
	video, _ := MP4()
	if len(video) != 841 || !bytes.Contains(video, []byte("mdat")) || !bytes.Contains(video, []byte("mp4v")) || !bytes.Contains(video, []byte("moov")) {
		test.Fatal("MP4 must contain the fixed MPEG-4 sample and movie metadata")
	}
	if _, _, err := ImportMedia("/not-a-fixture"); err == nil {
		test.Fatal("unknown media path accepted")
	}
}
