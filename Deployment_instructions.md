1. **Install and Set Up Google Cloud SDK**

First, you need to install the Google Cloud SDK:
1. Download the Google Cloud SDK installer from: https://cloud.google.com/sdk/docs/install
2. Run the installer
3. Open a new terminal and run:

```bash
# Initialize gcloud and login
gcloud init

# Follow the prompts to:
# 1. Log in to your Google account
# 2. Select or create a project
# 3. Choose a default region (e.g., us-central1)
```

2. **Create a New Project**

```bash
# Create a new project
gcloud projects create open-search-engine-project

# Set it as your active project
gcloud config set project open-search-engine-project

# Enable required APIs
gcloud services enable container.googleapis.com containerregistry.googleapis.com
```

3. **Set Up Container Registry Access**

```bash
# Configure Docker to use Google Cloud authentication
gcloud auth configure-docker
```

4. **Create a GKE (Google Kubernetes Engine) Cluster**

```bash
# Create a 3-node cluster
gcloud container clusters create open-search-cluster \
    --num-nodes=3 \
    --machine-type=e2-standard-2 \
    --region=us-central1 \
    --project=open-search-engine-project

# Get credentials for kubectl
gcloud container clusters get-credentials open-search-cluster \
    --region=us-central1 \
    --project=open-search-engine-project
```

5. **Build and Push Docker Images**

```bash
# Set your project ID as an environment variable
$PROJECT_ID="open-search-engine-project"

# Build and push crawler
docker build -t gcr.io/$PROJECT_ID/crawler:latest -f services/crawler/Dockerfile .
docker push gcr.io/$PROJECT_ID/crawler:latest

# Build and push indexer
docker build -t gcr.io/$PROJECT_ID/indexer:latest -f services/indexer/Dockerfile .
docker push gcr.io/$PROJECT_ID/indexer:latest

# Build and push query-engine
docker build -t gcr.io/$PROJECT_ID/query-engine:latest -f services/query-engine/Dockerfile .
docker push gcr.io/$PROJECT_ID/query-engine:latest
```

6. **Deploy to Kubernetes**

First, verify your connection to the cluster:
```bash
# Should show your GKE cluster
kubectl config current-context

# Should show available nodes
kubectl get nodes
```

Now deploy the services:
```bash
# Create the deployments and services
kubectl apply -f k8s/

# Watch the deployment progress
kubectl get pods -w
```

7. **Verify the Deployment**

```bash
# Check if all pods are running
kubectl get pods

# Check if services are created
kubectl get services

# Get the external IP for the query engine
kubectl get service query-engine
```

8. **Basic Monitoring and Management**

```bash
# View logs for a service (replace PODNAME with actual pod name)
kubectl logs -f PODNAME

# View deployment status
kubectl get deployments

# View persistent volume claims
kubectl get pvc
```

9. **Common Troubleshooting Commands**

```bash
# Describe a pod to see issues
kubectl describe pod PODNAME

# Get detailed information about a service
kubectl describe service query-engine

# Check persistent volume status
kubectl describe pvc crawl-output-pvc
kubectl describe pvc index-output-pvc
```

10. **Scaling the Application**

```bash
# Scale the number of crawler pods
kubectl scale deployment crawler --replicas=5

# Scale the query engine
kubectl scale deployment query-engine --replicas=4
```

11. **Cleanup When Needed**

```bash
# Delete all resources created by the deployment
kubectl delete -f k8s/

# Delete the cluster when you're done (to save costs)
gcloud container clusters delete open-search-cluster \
    --region=us-central1 \
    --project=open-search-engine-project
```

**Important Notes:**

1. **Costs**: 
   - GKE clusters cost money even when idle
   - You're charged for:
     - Node compute time
     - Persistent disk storage
     - Load balancer usage
   - Consider deleting the cluster when not in use

2. **Security**:
   - The query-engine service is exposed to the internet
   - Consider adding authentication if needed
   - Keep your Google Cloud credentials secure

3. **Data Persistence**:
   - Data in persistent volumes remains after pod restarts
   - But deleting the cluster will delete the volumes
   - Consider backing up important data

4. **Monitoring**:
   - Use Google Cloud Console to:
     - Monitor resource usage
     - View logs
     - Set up alerts
     - Track costs

Would you like me to explain any of these steps in more detail or help with a specific part of the deployment?