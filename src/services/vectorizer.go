package services

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/philippgille/chromem-go"
)

const embeddingApi = "http://192.168.178.105:11434/api"
const embeddingModel = "nomic-embed-text"

var collection *chromem.Collection

func SetUpVectorDb(overwrite bool) {
	ctx := context.Background()
	// Now we use our vector database for retrieval augmented generation (RAG),
	// which means we provide the LLM with relevant knowledge.
	// Set up chromem-go with persistence, so that when the program restarts, the
	// DB's data is still available.
	log.Println("Setting up chromem-go...")
	db, err := chromem.NewPersistentDB("./db", false)
	if err != nil {
		panic(err)
	}
	// Create collection if it wasn't loaded from persistent storage yet.
	// You can pass nil as embedding function to use the default (OpenAI text-embedding-3-small),
	// which is very good and cheap. It would require the OPENAI_API_KEY environment
	// variable to be set.
	// For this example we choose to use a locally running embedding model though.
	// It requires Ollama to serve its API at "http://localhost:11434/api".
	if overwrite {
		collection, err = db.CreateCollection("vectors", nil, chromem.NewEmbeddingFuncOllama(embeddingModel, embeddingApi))
	} else {
		collection, err = db.GetOrCreateCollection("vectors", nil, chromem.NewEmbeddingFuncOllama(embeddingModel, embeddingApi))
	}
	if err != nil {
		panic(err)
	}

	// Add docs to the collection, if the collection was just created (and not
	// loaded from persistent storage).
	var docs []chromem.Document
	if collection.Count() == 0 {
		// Here we use a DBpedia sample, where each line contains the lead section/introduction
		// to some Wikipedia article and its category.
		f, err := os.Open("./rag-data/dbpedia_sample.jsonl")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		d := json.NewDecoder(f)
		log.Println("Reading JSON lines...")
		for i := 1; ; i++ {
			var article struct {
				Text string `json:"text"`
			}
			err := d.Decode(&article)
			if err == io.EOF {
				break // reached end of file
			} else if err != nil {
				panic(err)
			}

			// The embeddings model we use in this example ("nomic-embed-text")
			// fare better with a prefix to differentiate between document and query.
			// We'll have to cut it off later when we retrieve the documents.
			// An alternative is to create the embedding with `chromem.NewDocument()`,
			// and then change back the content before adding it do the collection
			// with `collection.AddDocument()`.
			content := "search_document: " + article.Text

			docs = append(docs, chromem.Document{
				ID:      strconv.Itoa(i),
				Content: content,
			})
		}
		log.Println("Adding documents to chromem-go, including creating their embeddings via Ollama API...")
		err = collection.AddDocuments(ctx, docs, runtime.NumCPU())
		if err != nil {
			panic(err)
		}
	} else {
		log.Println("Not reading JSON lines because collection was loaded from persistent storage.")
	}
}

func QueryVectorDb(ctx context.Context, question string) []chromem.Result {
	// Search for documents that are semantically similar to the original question.
	// We ask for the two most similar documents, but you can use more or less depending
	// on your needs and the supported context size of the LLM you use.
	// You can limit the search by filtering on content or metadata (like the article's
	// category), but we don't do that in this example.
	start := time.Now()
	log.Println("Querying chromem-go...")
	// "nomic-embed-text" specific prefix (not required with OpenAI's or other models)
	query := "search_query: " + question

	docRes, err := collection.Query(ctx, query, 8, nil, nil)
	if err != nil {
		panic(err)
	}
	log.Println("Search (incl query embedding) took", time.Since(start))
	// Here you could filter out any documents whose similarity is below a certain threshold.
	// if docRes[...].Similarity < 0.5 { ...

	// Print the retrieved documents and their similarity to the question.
	for i, res := range docRes {
		// Cut off the prefix we added before adding the document (see comment above).
		// This is specific to the "nomic-embed-text" model.
		content := strings.TrimPrefix(res.Content, "search_document: ")
		log.Printf("Document %d (similarity: %f): \"%s\"\n", i+1, res.Similarity, content)
	}

	return docRes

}
