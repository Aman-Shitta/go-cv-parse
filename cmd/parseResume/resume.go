package resume

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	pdfconv "example.com/resPars/internal/utils"
	"github.com/otiai10/gosseract/v2"
)

type Words []gosseract.BoundingBox

type Page struct {
	PageNum int
	words   Words
	failed  bool
}

func (p Page) String() (line string) {
	for _, word := range p.words {
		// word will have Box, Word, Confidence etc.
		line += " " + strings.Trim(word.Word, "\n")
	}

	return line
}

func extractWordsToPages(client *gosseract.Client, path string, pages *[]Page, page int) {
	var err error
	var failed bool = true

	client.SetImage(path)

	words, err := client.GetBoundingBoxesVerbose()

	if err != nil {
		failed = true
		words = nil
	}

	*pages = append(*pages, Page{
		PageNum: page,
		words:   words,
		failed:  failed,
	})

}

func ProcessResume() {

	var pages []Page

	if len(os.Args) < 2 {
		log.Fatal("Usage: ./main <resume_path> [IMG or PDF]")
	}

	resumePath := os.Args[1]
	fmt.Printf("Got %s for %s\n", filepath.Ext(resumePath), resumePath)

	fileExt := strings.ToLower(filepath.Ext(resumePath))

	client := gosseract.NewClient()

	defer client.Close()

	switch fileExt {
	case ".png":
		extractWordsToPages(client, resumePath, &pages, 0)

	case ".pdf":

		outImgsPath, err := os.MkdirTemp("", "")
		if err != nil {
			log.Fatal("Error creating temp folder :: ", err.Error())
		}

		fmt.Printf("[+] Processing PDF @ %s [+]\n", outImgsPath)

		_, err = pdfconv.ConvertPdfToImg(resumePath, outImgsPath)

		if err != nil {
			log.Fatal("Error converting file", err.Error())
		}
		i := 0
		err = filepath.Walk(outImgsPath, func(
			path string, d fs.FileInfo, err_ error) error {
			if d.Mode().IsRegular() && d.Size() > 0 {
				extractWordsToPages(client, path, &pages, i)
				i++
			}
			return nil
		})

		if err != nil {
			log.Fatal("Error Extracting from PDF pages :: ", err.Error())
		}
	case ".jpeg", ".jpg":
		log.Fatal("JPEG is currently not working.")
	default:
		log.Fatalf("Format %s not supported yet.", fileExt)
	}

	// Now all goroutines have completed; we can safely iterate over `pages`
	fmt.Println("Final Output:")
	for _, p := range pages {
		fmt.Println(p)
	}
}
