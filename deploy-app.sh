#!/bin/bash
# TritonTube Complete Application Deployment Script
# This script builds and deploys the backend and frontend after infrastructure is set up
set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║         TritonTube Application Deployment Script              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "terraform" ]; then
    echo "Error: Must be run from the tritontube-server directory"
    exit 1
fi

# Get AWS configuration
if ! command -v aws &> /dev/null; then
    echo "Error: AWS CLI not installed"
    exit 1
fi

AWS_REGION=$(aws configure get region)
if [ -z "$AWS_REGION" ]; then
    AWS_REGION="us-west-1"
    echo "Using default region: $AWS_REGION"
fi

echo "Deployment Configuration:"
echo "   AWS Region: $AWS_REGION"
echo ""

# Step 1: Install Go dependencies
echo "═══════════════════════════════════════════════════════════════"
echo "Step 1/6: Installing Go dependencies"
echo "═══════════════════════════════════════════════════════════════"
echo ""

echo "Installing PostgreSQL driver..."
go get github.com/lib/pq

echo "Installing AWS SDK v2..."
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/s3

echo "Downloading all dependencies..."
go mod download

echo "Tidying go.mod..."
go mod tidy

echo "   Go dependencies installed"
echo ""

# Step 2: Get ECR repository URL from Terraform
echo "═══════════════════════════════════════════════════════════════"
echo "Step 2/6: Getting deployment configuration from Terraform"
echo "═══════════════════════════════════════════════════════════════"
echo ""

cd terraform

if [ ! -f "terraform.tfstate" ]; then
    echo "Error: Terraform state not found. Run ./terraform/deploy.sh first to create infrastructure"
    exit 1
fi

ECR_URL=$(terraform output -raw ecr_repository_url 2>/dev/null)
FRONTEND_BUCKET=$(terraform output -raw frontend_bucket_id 2>/dev/null)
FRONTEND_CDN_ID=$(terraform output -raw frontend_cdn_id 2>/dev/null)
ALB_DNS=$(terraform output -raw alb_dns_name 2>/dev/null)

if [ -z "$ECR_URL" ]; then
    echo "Error: Could not get ECR URL from Terraform. Ensure infrastructure is deployed"
    exit 1
fi

echo "   ECR Repository: $ECR_URL"
echo "   Frontend Bucket: $FRONTEND_BUCKET"
echo "   ALB DNS: $ALB_DNS"
echo "   ✓ Configuration loaded"
echo ""

cd ..

# Step 3: Build Docker image
echo "═══════════════════════════════════════════════════════════════"
echo "Step 3/6: Building Docker image"
echo "═══════════════════════════════════════════════════════════════"
echo ""

echo "Building tritontube-backend Docker image..."
docker build -t tritontube-backend .

if [ $? -ne 0 ]; then
    echo "Error: Docker build failed"
    exit 1
fi

echo "   Docker image built successfully"
echo ""

# Step 4: Push to ECR
echo "═══════════════════════════════════════════════════════════════"
echo "Step 4/6: Pushing Docker image to ECR"
echo "═══════════════════════════════════════════════════════════════"
echo ""

echo "Logging in to ECR..."
aws ecr get-login-password --region $AWS_REGION | \
    docker login --username AWS --password-stdin $ECR_URL

echo "Tagging image..."
docker tag tritontube-backend:latest $ECR_URL:latest

echo "Pushing to ECR (this may take a few minutes)..."
docker push $ECR_URL:latest

if [ $? -ne 0 ]; then
    echo "Error: Failed to push to ECR"
    exit 1
fi

echo "   Image pushed to ECR"
echo ""

# Step 5: Update and apply Terraform
echo "═══════════════════════════════════════════════════════════════"
echo "Step 5/6: Updating ECS with new container image"
echo "═══════════════════════════════════════════════════════════════"
echo ""

cd terraform

# Check if container_image is already set in terraform.tfvars
if grep -q "^container_image" terraform.tfvars; then
    echo "Updating container_image in terraform.tfvars..."
    sed -i.bak "s|^container_image.*|container_image = \"$ECR_URL:latest\"|" terraform.tfvars
    rm -f terraform.tfvars.bak
else
    echo "Adding container_image to terraform.tfvars..."
    echo "" >> terraform.tfvars
    echo "# Container configuration" >> terraform.tfvars
    echo "container_image = \"$ECR_URL:latest\"" >> terraform.tfvars
fi

echo "Applying Terraform changes..."
terraform apply -auto-approve

if [ $? -ne 0 ]; then
    echo "Error: Terraform apply failed"
    cd ..
    exit 1
fi

echo "   ECS service updated with new image"
echo ""

cd ..

# Step 6: Build and deploy frontend
echo "═══════════════════════════════════════════════════════════════"
echo "Step 6/6: Building and deploying frontend"
echo "═══════════════════════════════════════════════════════════════"
echo ""

if [ ! -f "package.json" ]; then
    echo "Warning: package.json not found, skipping frontend deployment"
else
    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        echo "Installing npm dependencies..."
        npm install
    fi

    echo "Building frontend (React)..."
    npm run build

    if [ $? -ne 0 ]; then
        echo "Error: Frontend build failed"
        exit 1
    fi

    echo "Deploying to S3..."
    aws s3 sync build/ s3://$FRONTEND_BUCKET/ --delete

    echo "Creating CloudFront invalidation..."
    INVALIDATION_ID=$(aws cloudfront create-invalidation \
        --distribution-id $FRONTEND_CDN_ID \
        --paths "/*" \
        --query 'Invalidation.Id' \
        --output text)

    echo "   Invalidation ID: $INVALIDATION_ID"
    echo "   ✓ Frontend deployed to S3 and CloudFront"
fi

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║              Deployment Complete!                              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Get final URLs
cd terraform
FRONTEND_URL=$(terraform output -raw frontend_cdn_url 2>/dev/null)
VIDEO_CDN=$(terraform output -raw video_cdn_domain 2>/dev/null)
cd ..

echo "Your application is now deployed:"
echo ""
echo "   Frontend:     https://$FRONTEND_URL"
echo "   API:          http://$ALB_DNS"
echo "   Video CDN:    https://$VIDEO_CDN"
echo ""
echo "Architecture:"
echo "   ECS Fargate with 3 tasks"
echo "   Application Load Balancer (600s timeout)"
echo "   PostgreSQL RDS (metadata storage)"
echo "   S3 (video content storage)"
echo "   CloudFront (CDN for videos and frontend)"
echo ""
echo "Monitoring:"
echo "   CloudWatch Logs: /ecs/tritontube-backend"
echo "   ECS Console: https://console.aws.amazon.com/ecs/home?region=$AWS_REGION#/clusters/tritontube-cluster"
echo ""
echo "To check deployment status:"
echo "   aws ecs describe-services --cluster tritontube-cluster --services tritontube-service --region $AWS_REGION"
echo ""
echo "To view logs:"
echo "   aws logs tail /ecs/tritontube-backend --follow --region $AWS_REGION"
echo ""
