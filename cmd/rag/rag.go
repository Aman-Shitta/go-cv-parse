package rag

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
	"google.golang.org/api/option"

	resume "example.com/resPars/cmd/parseResume"
)

const generativeModelName = "gemini-1.5-flash"
const embeddingModelName = "text-embedding-004"

func RagInit() *RAG {
	ctx := context.Background()
	wvClient, err := initWeaviate(ctx)
	if err != nil {
		log.Fatal("some inir err:", err.Error())
		return nil
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal(err)
	}

	rag := &RAG{
		ctx:      ctx,
		wvClient: wvClient,
		genModel: genaiClient.GenerativeModel(generativeModelName),
		embModel: genaiClient.EmbeddingModel(embeddingModelName),
		Gclient:  genaiClient,
	}

	return rag
}

type RAG struct {
	ctx      context.Context
	wvClient *weaviate.Client
	genModel *genai.GenerativeModel
	embModel *genai.EmbeddingModel
	Gclient  *genai.Client
}

func (rag *RAG) EmbedDocuments(pages []resume.Page) {

	batch := rag.embModel.NewBatch()
	for _, page := range pages {
		batch.AddContent(genai.Text("hellp"))
		page.PageNum += 0
	}

	log.Println("invoking embedding model with document", batch, rag)
	rsp, err := rag.embModel.BatchEmbedContents(rag.ctx, batch)
	if err != nil {
		log.Fatal("Embed error bathc :: ", err.Error())
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

func (rag *RAG) GenerateRagResponse(query string) (string, error) {

	fmt.Println("query : ", query, rag.ctx)
	// Embed the query contents.
	rsp, err := rag.embModel.EmbedContent(rag.ctx, genai.Text(query))
	if err != nil {
		return "", err
	}

	// Search weaviate to find the most relevant (closest in vector space)
	// documents to the query.
	gql := rag.wvClient.GraphQL()
	result, err := gql.Get().
		WithNearVector(
			gql.NearVectorArgBuilder().WithVector(rsp.Embedding.Values)).
		WithClassName("Document").
		WithFields(graphql.Field{Name: "text"}).
		WithLimit(3).
		Do(rag.ctx)
	if werr := combinedWeaviateError(result, err); werr != nil {
		return "", werr
	}

	contents, err := decodeGetResults(result)
	if err != nil {
		return "", err
	}

	// Merge user query + relevant docs into a single RAG prompt
	ragQuery := fmt.Sprintf(ragTemplateStr, query, strings.Join(contents, "\n"))

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
[NEVER SKIP THIS CONTEXT]
Respond to specific questions based on the document.
Provide precise and contextual answers using only the information available.
If a question cannot be answered from the document, reply with:
"The provided document does not contain enough information to answer that question. Please reach out or explore at [github link from text], [Linkedin github link from text] profiles"

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
