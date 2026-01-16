# Kubernetes Deployment

This guide covers deploying OpenStatus on Kubernetes using the Helm chart.

## Prerequisites

- Kubernetes cluster (1.24+)
- Helm 3.x
- kubectl configured for your cluster
- Container registry with OpenStatus images

## Building and Pushing Images

First, build and push the images to your registry:

```bash
# Set your registry
REGISTRY=your-registry.com/openstatus

# Build and push images
docker build -t $REGISTRY/api:latest ./api
docker build -t $REGISTRY/frontend:latest ./frontend
docker build -t $REGISTRY/admin:latest ./admin

docker push $REGISTRY/api:latest
docker push $REGISTRY/frontend:latest
docker push $REGISTRY/admin:latest
```

## Quick Start

### 1. Create values file

```bash
cat > my-values.yaml << 'EOF'
# Required: Generate with: openssl rand -base64 32
config:
  jwtSecretKey: "your-secret-key-here"

# Image registry
api:
  image:
    repository: your-registry.com/openstatus/api
frontend:
  image:
    repository: your-registry.com/openstatus/frontend
admin:
  image:
    repository: your-registry.com/openstatus/admin

# MySQL passwords
mysql:
  mode: standalone
  password: "change-me"
  rootPassword: "change-me-root"

# Ingress
ingress:
  enabled: true
  className: nginx
  hosts:
    frontend: status.example.com
    admin: admin.status.example.com
    api: api.status.example.com
EOF
```

### 2. Install

```bash
helm install openstatus ./helm/openstatus -f my-values.yaml
```

### 3. Verify

```bash
kubectl get pods
kubectl get ingress
```

## Configuration Options

### MySQL Modes

#### Standalone MySQL (default)

Simple MySQL deployment suitable for testing or small deployments:

```yaml
mysql:
  mode: standalone
  database: statuspage
  user: statuspage
  password: "secure-password"
  rootPassword: "secure-root-password"
  standalone:
    persistence:
      enabled: true
      size: 10Gi
      storageClass: ""  # Use default
```

#### MOCO (MySQL Operator)

For production, use [MOCO](https://cybozu-go.github.io/moco/) for managed MySQL:

```yaml
mysql:
  mode: moco
  database: statuspage
  user: statuspage
  password: "secure-password"
  moco:
    clusterName: openstatus-mysql
    replicas: 1  # Single node
    version: "8.4"
    storageSize: 10Gi
```

**Note:** MOCO must be installed on your cluster first:

```bash
# Install MOCO operator
kubectl apply -f https://github.com/cybozu-go/moco/releases/latest/download/moco.yaml
```

### Ingress Configuration

#### Basic (no TLS)

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    frontend: status.example.com
    admin: admin.status.example.com
    api: api.status.example.com
```

#### With TLS (cert-manager)

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  tls:
    enabled: true
    secretName: openstatus-tls
  hosts:
    frontend: status.example.com
    admin: admin.status.example.com
    api: api.status.example.com
```

#### Traefik

```yaml
ingress:
  enabled: true
  className: traefik
  annotations:
    traefik.ingress.kubernetes.io/router.tls: "true"
  tls:
    enabled: true
  hosts:
    frontend: status.example.com
    admin: admin.status.example.com
```

### Email Configuration

#### SMTP

```yaml
email:
  provider: smtp
  fromAddress: noreply@example.com
  fromName: "Status Page"
  smtp:
    host: smtp.example.com
    port: 587
    username: user
    password: password
    tls: false
```

#### SendGrid

```yaml
email:
  provider: sendgrid
  fromAddress: noreply@example.com
  fromName: "Status Page"
  sendgrid:
    apiKey: "SG.xxxxx"
```

### Using Existing Secrets

For production, create secrets manually and reference them:

```bash
# Create secrets
kubectl create secret generic openstatus-config \
  --from-literal=jwt-secret-key=$(openssl rand -base64 32)

kubectl create secret generic openstatus-mysql \
  --from-literal=mysql-password=your-password \
  --from-literal=mysql-root-password=your-root-password

kubectl create secret generic openstatus-email \
  --from-literal=smtp-password=your-smtp-password
```

```yaml
config:
  existingSecret: openstatus-config

mysql:
  existingSecret: openstatus-mysql

email:
  existingSecret: openstatus-email
```

### Private Registry

```yaml
global:
  imagePullSecrets:
    - my-registry-secret
```

## Resource Tuning

Adjust resources based on your load:

```yaml
api:
  replicaCount: 2
  resources:
    limits:
      cpu: 1000m
      memory: 512Mi
    requests:
      cpu: 200m
      memory: 256Mi

frontend:
  replicaCount: 2
  resources:
    limits:
      cpu: 500m
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

## Upgrading

```bash
helm upgrade openstatus ./helm/openstatus -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall openstatus

# If you want to delete PVCs too (WARNING: deletes data!)
kubectl delete pvc -l app.kubernetes.io/instance=openstatus
```

## Troubleshooting

### Check pod status

```bash
kubectl get pods
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Check API health

```bash
kubectl port-forward svc/openstatus-api 8080:8080
curl http://localhost:8080/api/v1/status
```

### MySQL connection issues

```bash
# Check MySQL pod
kubectl logs <mysql-pod>

# Test connection from API pod
kubectl exec -it <api-pod> -- sh
nc -zv <mysql-host> 3306
```

### MOCO MySQL status

```bash
kubectl get mysqlcluster
kubectl describe mysqlcluster openstatus-mysql
```
