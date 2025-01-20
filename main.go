//  ___    _      __     ______
//  |_ _|  / \     \ \   / /___ \
//   | |  / _ \ ____\ \ / /    | | -
//   | | / ___ \_____\ V /  ___| |
//  |___/_/   \_\     \_/  |____/

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	resume "example.com/resPars/cmd/parseResume"
	"example.com/resPars/cmd/rag"
)

func main() {
	var ragClient *rag.RAG
	resumePages := resume.ProcessResume()

	ragClient = rag.RagInit()

	ragClient.EmbedDocuments(resumePages)

	defer ragClient.Gclient.Close()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nWhat do you want to know? (type 'bye' to quit): ")

		// Read user input
		if !scanner.Scan() {
			// EOF or other error
			fmt.Println("\nExiting...")
			break
		}
		userQuery := strings.TrimSpace(scanner.Text())

		// Check for exit condition
		if strings.EqualFold(userQuery, "bye") {
			fmt.Println("Goodbye!")
			break
		}
		if userQuery == "" {
			continue // skip empty lines
		}

		// 5. Process the query through RAG for an answer
		answer, err := ragClient.GenerateRagResponse(userQuery)
		if err != nil {
			fmt.Printf("Error getting answer: %v\n", err)
			continue
		}

		// 6. Print the answer
		fmt.Printf("Answer: %s\n", answer)
	}

}
