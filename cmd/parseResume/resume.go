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
	var conf float64
	for _, word := range p.words {
		conf += word.Confidence
		// word will have Box, Word, Confidence etc.
		line += " " + word.Word
		conf = conf / 2
	}

	return fmt.Sprintf("%s : %.02f", line, conf)
}

func extractWordsToPages(path string, pages *[]Page, page int) {

	var err error
	var failed bool = true
	// init client
	client := gosseract.NewClient()

	defer client.Close()

	client.SetImage(path)
	words, err := client.GetBoundingBoxesVerbose()

	if err != nil {
		fmt.Println("BB error", err.Error())
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

	switch fileExt {
	case ".jpeg", ".jpg", ".png":
		extractWordsToPages(resumePath, &pages, 0)

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

				extractWordsToPages(path, &pages, i)

				// if err != nil {
				// 	return err
				// }
				// pages = append(pages, Page{i, words, false})
				i++
			}
			return nil
		})

		if err != nil {
			log.Fatal("Error Extracting from PDF pages :: ", err.Error())
		}
	default:
		log.Fatalf("Format %s not supported yet.", fileExt)
	}

	// Now all goroutines have completed; we can safely iterate over `pages`
	fmt.Println("Final Output:")
	for _, p := range pages {
		fmt.Println(p)
	}
}
