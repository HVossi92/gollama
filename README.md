# Gollama

A simple chatbot using Go on the backend, HTMX on the frontend and libsql as a vector database as well as Ollama to run the LLMs.
This was mostly an exercise to learn learn the used technologies.

## Required Ollama models:

- "llama3.1:8b-instruct-q8_0" for answering questions. It is a fairly small model, to make sure it can only answer questions by actually using RAG.
- "nomic-embed-text:latest" for creating embeddings from text. This is a very fast model, specially for creating embeddings from text.

## Run

- Simply run via `air`

## Add RAG data to vector database

- Open plain text via the 'Vector database upload area' in the frontend. Every upload will be chunked and saved to the DB.

## Chat Workflow

- Upload data to the vector db, to make the system smarter by using RAG.
- Send a request from the frontend chat interface to the backend.
- The backend will forward that request to an embedding model, to get the question as a vector.
- Then that vector question will be used to query the vector database, to find similar chunks of text.
- Now the plain text question, together with the plain text chunks of text, will be sent to the LLM.
- The LLM will generate a response based on that data, and either answer the question of say it doesn't know.

## Image Workflow (just a small test case)

- Choose an image file.
- Upload, it will be saved to the backend.
- Start annotation.
- Draw a rectangle around an area of interest.
- Submit annotation
- The picture will be formatted to base64, and the formatted image together with the rectangle coordinates will be sent to the LLM to be analyzed.
