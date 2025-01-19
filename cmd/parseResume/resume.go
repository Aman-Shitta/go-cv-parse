package resume

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	helper "example.com/resPars/internal/utils"
	"github.com/otiai10/gosseract/v2"
)

type Words []gosseract.BoundingBox

type Page struct {
	PageNum int
	words   Words
	failed  bool
}

func (p *Page) String() (line string) {
	for _, word := range p.words {
		// word will have Box, Word, Confidence etc.
		line += " " + strings.Trim(word.Word, "\n") + " "
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

func extractPageData(resumePath string) (pages []Page, tempPath string, err error) {

	client := gosseract.NewClient()

	defer client.Close()

	fileExt := strings.ToLower(filepath.Ext(resumePath))
	fileName := helper.FileNameWithoutExtension(strings.ToLower(filepath.Base(resumePath)))

	var outImgsPath string

	switch fileExt {
	case ".png":
		extractWordsToPages(client, resumePath, &pages, 0)

	case ".pdf":

		outImgsPath, err := os.MkdirTemp("", fileName)

		if err != nil {
			return nil, outImgsPath, fmt.Errorf("error creating temp folder : %s", err.Error())
		}

		fmt.Printf("[+] Processing PDF @ %s [+]\n", outImgsPath)

		_, err = helper.ConvertPdfToImg(resumePath, outImgsPath)

		if err != nil {
			return nil, outImgsPath, fmt.Errorf("error converting file : %s", err.Error())
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
			return nil, outImgsPath, fmt.Errorf("error Extracting from PDF pages : %s", err.Error())
		}
	case ".jpeg", ".jpg":
		return nil, outImgsPath, fmt.Errorf("fomat JPEG is currently not working")
	default:
		return nil, outImgsPath, fmt.Errorf("format %s not supported yet", fileExt)
	}

	return pages, outImgsPath, nil
}

func ProcessResume() {

	if len(os.Args) < 2 {
		log.Fatal("Usage: ./main <resume_path> [IMG or PDF]")
	}

	resumePath := os.Args[1]
	fmt.Printf("Got %s for %s\n", filepath.Ext(resumePath), resumePath)

	pages, tempDir, err := extractPageData(resumePath)

	if tempDir != "" {
		defer os.RemoveAll(tempDir)
	}

	if err != nil {
		log.Fatal("extractPageData : err : ", err.Error())
	}

	// Now all goroutines have completed; we can safely iterate over `pages`
	fmt.Println("Final Output:")
	for _, p := range pages {
		fmt.Println(p)
	}

}
