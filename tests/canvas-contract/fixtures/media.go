package fixtures

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
)

const ImportMediaOrigin = "https://files.tapnow.media"

func WAV() []byte {
	const sampleCount = 80
	content := make([]byte, 44+sampleCount*2)
	copy(content, "RIFF")
	binary.LittleEndian.PutUint32(content[4:], uint32(len(content)-8))
	copy(content[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(content[16:], 16)
	binary.LittleEndian.PutUint16(content[20:], 1)
	binary.LittleEndian.PutUint16(content[22:], 1)
	binary.LittleEndian.PutUint32(content[24:], 8000)
	binary.LittleEndian.PutUint32(content[28:], 16000)
	binary.LittleEndian.PutUint16(content[32:], 2)
	binary.LittleEndian.PutUint16(content[34:], 16)
	copy(content[36:], "data")
	binary.LittleEndian.PutUint32(content[40:], sampleCount*2)
	for sample := 0; sample < sampleCount; sample++ {
		binary.LittleEndian.PutUint16(content[44+sample*2:], uint16(sample*100))
	}
	return content
}

func MP4() ([]byte, error) {
	return base64.StdEncoding.DecodeString("AAAAHGZ0eXBpc29tAAACAGlzb21pc28ybXA0MQAAAAhmcmVlAAAAGm1kYXQAAAGzABAHAAABthBgsYKZt+8AAAMLbW9vdgAAAGxtdmhkAAAAAAAAAAAAAAAAAAAD6AAAACgAAQAAAQAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAlp0cmFrAAAAXHRraGQAAAADAAAAAAAAAAAAAAABAAAAAAAAACgAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAABAAAAAQAAAAAAAkZWR0cwAAABxlbHN0AAAAAAAAAAEAAAAoAAAAAAABAAAAAAHSbWRpYQAAACBtZGhkAAAAAAAAAAAAAAAAAAAyAAAAAgBVxAAAAAAALWhkbHIAAAAAAAAAAHZpZGUAAAAAAAAAAAAAAABWaWRlb0hhbmRsZXIAAAABfW1pbmYAAAAUdm1oZAAAAAEAAAAAAAAAAAAAACRkaW5mAAAAHGRyZWYAAAAAAAAAAQAAAAx1cmwgAAAAAQAAAT1zdGJsAAAA2XN0c2QAAAAAAAAAAQAAAMltcDR2AAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAABAAEABIAAAASAAAAAAAAAABCkxhdmMgbXBlZzQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGP//AAAAT2VzZHMAAAAAA4CAgD4AAQAEgICAMCARAAAAAAMNQAAADhAFgICAHgAAAbABAAABtYkTAAABAAAAASAAxI2IAM0AhAIUYwaAgIABAgAAABBwYXNwAAAAAQAAAAEAAAAUYnRydAAAAAAAAw1AAAAOEAAAABhzdHRzAAAAAAAAAAEAAAABAAACAAAAABxzdHNjAAAAAAAAAAEAAAABAAAAAQAAAAEAAAAUc3RzegAAAAAAAAASAAAAAQAAABRzdGNvAAAAAAAAAAEAAAAsAAAAPXVkdGEAAAA1bWV0YQAAAAAAAAAhaGRscgAAAAAAAAAAbWRpcmFwcGwAAAAAAAAAAAAAAAAIaWxzdA==")
}

func ImportMedia(path string) ([]byte, string, error) {
	switch path {
	case "/contract/image.png":
		content, err := PNG()
		return content, "image/png", err
	case "/contract/audio.wav":
		return WAV(), "audio/wave", nil
	case "/contract/video.mp4":
		content, err := MP4()
		return content, "video/mp4", err
	default:
		return nil, "", errors.New("unknown controlled import media path")
	}
}
