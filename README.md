# Gollama

A learning exercise project implementing a simple chatbot with RAG (Retrieval-Augmented Generation) capabilities and basic image analysis features. This project was created specifically to explore and learn the following technologies:

- Go (Backend)
- HTMX (Frontend)
- LibSQL (Vector Database)
- Ollama (Local LLM Runtime)

> **Note**: This is an educational project intended for learning purposes and should not be considered production-ready.

## Features

- Chat interface with RAG capabilities
- Vector database for storing and retrieving relevant context
- Basic image analysis with region selection
- Simple and lightweight frontend using HTMX

## Prerequisites

### Required Ollama Models

This project requires two specific models to be installed via Ollama:

1. `llama3.1:8b-instruct-q8_0` - For question answering (deliberately using a smaller model to demonstrate RAG effectiveness)
2. `nomic-embed-text:latest` - For fast and efficient text embedding generation

Make sure to have [Ollama](https://ollama.ai) installed and these models pulled before running the application.

## Development Setup

1. Install Go and Ollama
2. Clone this repository
3. Install [Air](https://github.com/cosmtrek/air) for hot reloading (optional but recommended for development)
4. Run the development server:
   ```bash
   air
   ```

## Production Build

The application can be built into a single binary that includes all static assets:

```bash
make build
```

The compiled binary will be available in the `build/` directory as `gollama`. The application is self-contained except for:

- The database file (automatically generated on first run)
- Ollama and its models (must be installed separately)

## Usage Guide

### Vector Database Setup

1. Start the application
2. Navigate to the "Vector Database Upload Area" in the web interface
3. Upload plain text files that will serve as the knowledge base
4. The text will be automatically chunked and stored in the vector database

### Chat Interface

1. Ensure you've uploaded some knowledge base text first
2. Enter your question in the chat interface
3. The system will:
   - Convert your question into a vector
   - Find relevant context from the vector database
   - Use the LLM to generate an answer based on the retrieved context

### Image Analysis

1. Select an image file through the interface
2. Upload the image
3. Click "Start Annotation"
4. Draw a rectangle around the area you want to analyze
5. Submit the annotation
6. The system will analyze the selected region using the LLM

## Project Architecture

- `src/` - Main application code
  - `main.go` - Application entry point and server setup
  - `services/` - Core business logic
  - `templates/` - HTML templates
  - `static/` - Frontend assets
  - `utils/` - Helper functions
