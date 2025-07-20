# Open Search Engine

A distributed web search engine built with Go, featuring a crawler, indexer, and query engine deployed on Kubernetes.
Note: I have disabled the cloud deployment due to monetary constraints

## Overview

This project implements a scalable search engine with three main components:

1. **Crawler**: Multi-threaded web crawler with politeness controls and robots.txt compliance
2. **Indexer**: Text processing and inverted index construction
3. **Query Engine**: Search API with TF-IDF ranking and result snippets

## Architecture

```
┌─────────────┐     ┌──────────┐     ┌──────────────┐
│   Crawler   │────▶│  Indexer │────▶│ Query Engine │
└─────────────┘     └──────────┘     └──────────────┘
      │                   │                  │
      ▼                   ▼                  ▼
┌─────────────┐     ┌──────────┐     ┌──────────────┐
│Crawl Output │────▶│  Index   │◀────│    Search    │
│    JSON     │     │  Files   │     │     API      │
└─────────────┘     └──────────┘     └──────────────┘
```

### Components

#### Crawler
- Multi-threaded web crawling
- URL frontier management
- Robots.txt compliance
- Rate limiting and politeness delays
- HTML content fetching and storage
- Outputs to JSON files

#### Indexer
- HTML cleaning and text extraction
- Stop word removal
- Porter stemming
- Inverted index construction
- Term frequency tracking
- Position-based indexing

#### Query Engine
- TF-IDF based ranking
- Position-based scoring
- Title boosting
- Search suggestions
- REST API
- Snippet generation

## Prerequisites

- Go 1.19 or later
- Docker Desktop
- Google Cloud SDK
- kubectl
- A Google Cloud Platform account

## Local Development

1. **Clone the repository**
```bash
git clone <repository-url>
cd open-search
```

2. **Install dependencies**
```bash
go mod tidy
```

3. **Run components locally**

Crawler:
```bash
go run main.go -mode=crawler -seeds=seeds.txt
```

Indexer:
```bash
go run indexer.go
```

Query Engine:
```bash
cd query_engine/cmd/search
go run main.go
```

## Deployment to Google Cloud

1. **Install and Set Up Google Cloud SDK**
```bash
# Initialize gcloud and login
gcloud init
```

2. **Create a New Project**
```bash
# Create project
gcloud projects create open-search-engine-project

# Set active project
gcloud config set project open-search-engine-project

# Enable APIs
gcloud services enable container.googleapis.com containerregistry.googleapis.com
```

3. **Set Up Container Registry**
```bash
# Configure Docker
gcloud auth configure-docker
```

4. **Create GKE Cluster**
```bash
# Create cluster
gcloud container clusters create open-search-cluster \
    --num-nodes=3 \
    --machine-type=e2-standard-2 \
    --region=us-central1

# Get credentials
gcloud container clusters get-credentials open-search-cluster \
    --region=us-central1
```

5. **Build and Push Docker Images**
```bash
# Set project ID
$PROJECT_ID="open-search-engine-project"

# Build and push images
docker build -t gcr.io/$PROJECT_ID/crawler:latest -f services/crawler/Dockerfile .
docker push gcr.io/$PROJECT_ID/crawler:latest

docker build -t gcr.io/$PROJECT_ID/indexer:latest -f services/indexer/Dockerfile .
docker push gcr.io/$PROJECT_ID/indexer:latest

docker build -t gcr.io/$PROJECT_ID/query-engine:latest -f services/query-engine/Dockerfile .
docker push gcr.io/$PROJECT_ID/query-engine:latest
```

6. **Deploy to Kubernetes**
```bash
# Apply configurations
kubectl apply -f k8s/

# Verify deployment
kubectl get pods
kubectl get services
```

## Configuration

The system can be configured through environment variables or command line flags:

### Crawler
- `CRAWLER_THREADS`: Number of crawler threads (default: 10)
- `POLITENESS_DELAY`: Delay between requests (default: 1s)
- `MAX_PAGES`: Maximum pages to crawl (default: 1000)

### Indexer
- `INDEX_DIR`: Directory for index files (default: "./index_output")
- `BATCH_SIZE`: Documents per batch (default: 1000)

### Query Engine
- `PORT`: API port (default: 8080)
- `MAX_RESULTS`: Maximum search results (default: 10)

## API Endpoints

### Search
```
GET /search?q=query&max=10
```

Response:
```json
{
  "results": [
    {
      "url": "https://example.com",
      "title": "Example Page",
      "score": 0.75,
      "snippet": "Matching text excerpt..."
    }
  ],
  "suggestions": ["related", "terms"]
}
```

### Stats
```
GET /stats
```

Response:
```json
{
  "total_documents": 1000,
  "total_terms": 50000,
  "index_size": "100MB"
}
```

## Monitoring

Monitor the deployment using:
```bash
# View logs
kubectl logs -f <pod-name>

# Check status
kubectl get deployments
kubectl get pods
kubectl get services
```

## Cleanup

To delete the deployment and cluster:
```bash
# Delete K8s resources
kubectl delete -f k8s/

# Delete cluster
gcloud container clusters delete open-search-cluster \
    --region=us-central1
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License
