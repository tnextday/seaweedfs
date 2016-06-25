package images

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
)

const defaultJpegQuality = 85

func Resized(ext string, data []byte, width, height int) (resized []byte, w int, h int, e error) {
	if width == 0 && height == 0 {
		return data, 0, 0, nil
	}
	srcImage, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		bounds := srcImage.Bounds()
		var dstImage *image.NRGBA
		if bounds.Dx() > width && width != 0 || bounds.Dy() > height && height != 0 {
			if width == height && bounds.Dx() != bounds.Dy() {
				dstImage = imaging.Thumbnail(srcImage, width, height, imaging.Lanczos)
				w, h = width, height
			} else {
				dstImage = imaging.Resize(srcImage, width, height, imaging.Lanczos)
			}
		} else {
			return data, bounds.Dx(), bounds.Dy(), nil
		}
		var buf bytes.Buffer
		switch ext {
		case ".png":
			err = png.Encode(&buf, dstImage)
		case ".jpg", ".jpeg":
			err = jpeg.Encode(&buf, dstImage, &jpeg.Options{Quality: defaultJpegQuality})
		case ".gif":
			err = gif.Encode(&buf, dstImage, nil)
		}
		if err == nil {
			return buf.Bytes(), dstImage.Bounds().Dx(), dstImage.Bounds().Dy(), nil
		}
	}
	return data, 0, 0, err
}
