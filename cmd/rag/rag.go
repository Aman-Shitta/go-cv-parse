package rag

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"

	resume "example.com/resPars/cmd/parseResume"
)

const generativeModelName = "gemini-1.5-flash"
const embeddingModelName = "text-embedding-004"

type RAG struct {
	ctx      context.Context
	wvClient *weaviate.Client
	genModel *genai.GenerativeModel
	embModel *genai.EmbeddingModel
}

func (rag *RAG) EmbedDocuments(pages []resume.Page) {

	batch := rag.embModel.NewBatch()
	for _, page := range pages {
		batch.AddContent(genai.Text(page.String()))
	}

	log.Printf("invoking embedding model with document")
	rsp, err := rag.embModel.BatchEmbedContents(rag.ctx, batch)
	if err != nil {
		return
	}

	objects := make([]*models.Object, len(pages))
	for i, page := range pages {
		objects[i] = &models.Object{
			Class: "Document",
			Properties: map[string]any{
				"text": page.String(),
			},
			Vector: rsp.Embeddings[i].Values,
		}
	}

	// Store documents with embeddings in the Weaviate DB.
	log.Printf("storing %v objects in weaviate", len(objects))
	_, err = rag.wvClient.Batch().ObjectsBatcher().WithObjects(objects...).Do(rag.ctx)
	if err != nil {
		return
	}
}

func (rag *RAG) generateRagResponse(query string, docs []string) (string, error) {
	// Merge user query + relevant docs into a single RAG prompt
	ragQuery := fmt.Sprintf(ragTemplateStr, query, strings.Join(docs, "\n"))

	// Call generative model
	resp, err := rag.genModel.GenerateContent(rag.ctx, genai.Text(ragQuery))
	if err != nil {
		return "", fmt.Errorf("calling generative model: %v", err)
	}

	// We expect exactly one candidate for a single completions request
	if len(resp.Candidates) != 1 {
		return "", fmt.Errorf("got %d candidates, expected 1", len(resp.Candidates))
	}

	// Gather all parts of the candidate’s content
	var respTexts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if pt, ok := part.(genai.Text); ok {
			respTexts = append(respTexts, string(pt))
		} else {
			return "", fmt.Errorf("bad type of part: %v", part)
		}
	}
	return strings.Join(respTexts, "\n"), nil
}

const ragTemplateStr = `
I will ask you a question and will provide some additional context information.
Extract key details such as skills, experience, education, and qualifications.

Respond to specific questions based on the document.
Provide precise and contextual answers using only the information available.
If a question cannot be answered from the document, reply with:
"The provided document does not contain enough information to answer that question. Please reach out or explore at [document GITHUB], [document Linkedin] profiles"

Restrictions:
 - Context-Only Responses: Use only the provided document—do not generate or assume missing details.
 - Concise and Clear: Keep responses clear and to the point.

Question:
%s

Context:
%s
`

func decodeGetResults(result *models.GraphQLResponse) ([]string, error) {
	data, ok := result.Data["Get"]
	if !ok {
		return nil, fmt.Errorf("get key not found in result")
	}
	doc, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("get key unexpected type")
	}
	slc, ok := doc["Document"].([]any)
	if !ok {
		return nil, fmt.Errorf("document is not a list of results")
	}

	var out []string
	for _, s := range slc {
		smap, ok := s.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid element in list of documents")
		}
		textVal, ok := smap["text"].(string)
		if !ok {
			return nil, fmt.Errorf("expected string in list of documents")
		}
		out = append(out, textVal)
	}
	return out, nil
}
