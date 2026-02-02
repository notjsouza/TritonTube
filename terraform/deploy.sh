#!/bin/bash
# TritonTube Terraform Quick Start Script
set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║         TritonTube Infrastructure Setup Script                 ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v terraform &> /dev/null; then
    echo "   Terraform is not installed. Please install it first:"
    echo "   https://developer.hashicorp.com/terraform/downloads"
    exit 1
fi

if ! command -v aws &> /dev/null; then
    echo "   AWS CLI is not installed. Please install it first:"
    echo "   https://aws.amazon.com/cli/"
    exit 1
fi

if ! command -v docker &> /dev/null; then
    echo "   Docker is not installed. Please install it first:"
    echo "   https://www.docker.com/get-started"
    exit 1
fi

echo "All prerequisites installed"
echo ""

# Check AWS credentials
echo "Checking AWS credentials..."
if ! aws sts get-caller-identity &> /dev/null; then
    echo "AWS credentials not configured. Run: aws configure"
    exit 1
fi

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
AWS_REGION=$(aws configure get region)

echo "   AWS configured"
echo "   Account ID: $AWS_ACCOUNT_ID"
echo "   Region: $AWS_REGION"
echo ""

# Initialize Terraform
echo "Initializing Terraform..."
terraform init
echo ""

# Check if terraform.tfvars exists
if [ ! -f "terraform.tfvars" ]; then
    echo "Creating terraform.tfvars from example..."
    cp terraform.tfvars.example terraform.tfvars
    
    # Update with AWS account ID
    sed -i.bak "s/992382698108/$AWS_ACCOUNT_ID/g" terraform.tfvars
    sed -i.bak "s/us-west-1/$AWS_REGION/g" terraform.tfvars
    rm terraform.tfvars.bak
    
    echo "   ✓ Created terraform.tfvars"
    echo ""
    echo "   IMPORTANT: Please edit terraform/terraform.tfvars and set:"
    echo "   1. Your container_image after building and pushing to ECR"
    echo "   2. Your preferred aws_region (currently: $AWS_REGION)"
    echo "   3. Your project_name if different"
    echo ""
    echo "After editing, run this script again or run:"
    echo "   cd terraform"
    echo "   terraform plan"
    echo "   terraform apply"
    exit 0
fi

# Validate configuration
echo "Validating Terraform configuration..."
terraform validate
echo ""

# Show plan
echo "Generating deployment plan..."
echo ""
terraform plan -out=tfplan
echo ""

# Ask for confirmation
read -p "Do you want to deploy this infrastructure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Deployment cancelled"
    rm -f tfplan
    exit 0
fi

# Apply
echo ""
echo "Deploying infrastructure (this will take 15-20 minutes)..."
terraform apply tfplan
rm -f tfplan

# Save outputs
echo ""
echo "Saving deployment information..."
terraform output > deployment-info.txt
terraform output -json > deployment-info.json

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                  Deployment Complete!                          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Display important information
ECR_URL=$(terraform output -raw ecr_repository_url)
ALB_DNS=$(terraform output -raw alb_dns_name)
VIDEO_CDN=$(terraform output -raw video_cdn_domain)
FRONTEND_CDN=$(terraform output -raw frontend_cdn_domain)

echo "   Important URLs:"
echo ""
echo "   ECR Repository:  $ECR_URL"
echo "   API Endpoint:    http://$ALB_DNS"
echo "   Video CDN:       https://$VIDEO_CDN"
echo "   Frontend CDN:    https://$FRONTEND_CDN"
echo ""
echo "   Next Steps:"
echo ""
echo "1. Install Go dependencies and build Docker image:"
echo "   cd ../"
echo "   go mod download"
echo "   go mod tidy"
echo ""
echo "2. Build and push Docker images:"
echo "   aws ecr get-login-password --region $AWS_REGION | \\"
echo "     docker login --username AWS --password-stdin $ECR_URL"
echo "   docker build -t tritontube-backend ."
echo "   docker tag tritontube-backend:latest $ECR_URL:latest"
echo "   docker push $ECR_URL:latest"
echo ""
echo "3. Update terraform/terraform.tfvars with:"
echo "   container_image = \"$ECR_URL:latest\""
echo ""
echo "4. Apply updated configuration:"
echo "   cd terraform/"
echo "   terraform apply"
echo ""
echo "5. Build and deploy frontend:"
echo "   cd ../"
echo "   npm run build"
echo "   aws s3 sync build/ s3://tritontube-frontend/ --delete"
echo ""
echo "6. Invalidate CloudFront cache:"
echo "   aws cloudfront create-invalidation \\"
echo "     --distribution-id \$(cd terraform && terraform output -raw frontend_cdn_id) \\"
echo "     --paths \"/*\""
echo ""
echo "   Or run the automated deployment script:"
echo "   ./deploy-app.sh"
echo ""
echo "   For detailed instructions, see:"
echo "   - terraform/README.md"
echo ""
echo "   Infrastructure uses DynamoDB for metadata storage (serverless)"
echo ""
