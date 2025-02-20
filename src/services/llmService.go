package services

import (
	"context"
	"log"
)

func AskLlm(question string) string {
	ctx := context.Background()

	// Warm up Ollama, in case the model isn't loaded yet
	log.Println("Warming up Ollama...")
	_ = askLLM(ctx, nil, "Hello!")

	// First we ask an LLM a fairly specific question that it likely won't know
	// the answer to.
	log.Println("Question: " + question)
	log.Println("Asking LLM...")
	reply := askLLM(ctx, nil, question)
	log.Printf("Initial reply from the LLM: \"" + reply + "\"\n")

	// CHROMEM STUFF
	docRes := QueryVectorDb(ctx, question)

	// Now we can ask the LLM again, augmenting the question with the knowledge we retrieved.
	// In this example we just use both retrieved documents as context.
	contexts := []string{docRes[0].Content, docRes[1].Content}
	log.Println("Asking LLM with augmented question...")
	reply = askLLM(ctx, contexts, question)
	log.Printf("Reply after augmenting the question with knowledge: \"" + reply + "\"\n")

	return reply
}
