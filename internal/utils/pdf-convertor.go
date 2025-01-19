package pdfconv

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-fitz"
)

// https://github.com/gen2brain/go-fitz/

func ConvertPdfToImg(path string, outPath string) (int, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return 0, err
	}

	defer doc.Close()

	totalPages := doc.NumPage()
	// Extract pages as images
	for n := 0; n < totalPages; n++ {
		img, err := doc.Image(n)
		if err != nil {
			return 0, err
		}
		imgPath := filepath.Join(outPath, fmt.Sprintf("page_%03d.jpeg", n))

		f, err := os.Create(imgPath)
		if err != nil {
			return 0, err
		}

		err = png.Encode(f, img)
		if err != nil {
			return 0, err
		}

		f.Close()
	}
	return totalPages, nil
}
