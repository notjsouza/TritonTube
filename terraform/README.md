# TritonTube Terraform Infrastructure

Complete Infrastructure as Code for deploying TritonTube to AWS using Terraform.

## 📁 Project Structure

```
terraform/
├── main.tf                    # Main configuration
├── variables.tf               # Input variables
├── outputs.tf                 # Output values
├── terraform.tfvars.example   # Example configuration
├── README.md                  # This file
└── modules/
    ├── s3/                    # S3 buckets for video, frontend, uploads
    ├── cloudfront/            # CDN distributions
    ├── networking/            # VPC, subnets, NAT gateways
    ├── iam/                   # IAM roles and policies
    └── ecs/                   # ECS cluster, ALB, DynamoDB, auto-scaling
```

## What Gets Created

This Terraform configuration creates:

- **Networking**: VPC with public/private subnets across 2 AZs
- **Storage**: 3 S3 buckets (video content, frontend, uploads)
- **CDN**: 2 CloudFront distributions (video + frontend)
- **Database**: DynamoDB table for metadata (serverless)
- **Compute**: ECS Fargate cluster with auto-scaling
- **Load Balancing**: Application Load Balancer
- **Container Registry**: ECR repository for Docker images
- **IAM**: Roles and policies for ECS tasks
- **Monitoring**: CloudWatch logs and metrics

## Quick Start

### 1. Prerequisites

```bash
# Install Terraform
# https://developer.hashicorp.com/terraform/downloads

# Verify installation
terraform version

# Configure AWS CLI
aws configure
```

### 2. Initialize Terraform

```bash
cd terraform
terraform init
```

### 3. Configure Variables

```bash
# Copy example file
cp terraform.tfvars.example terraform.tfvars

# Edit with your container image URL after pushing to ECR
```

### 4. Review Plan

```bash
# See what will be created
terraform plan
```

### 5. Deploy Infrastructure

```bash
# Deploy everything
terraform apply

# Type 'yes' to confirm
```

## Deployment Time

- Initial deployment: **~8-10 minutes**
  - VPC & Networking: ~2 min
  - DynamoDB Table: ~1 min
  - S3 & CloudFront: ~3 min
  - ECS & ALB: ~3 min

## Configuration

### Required Variables

Create `terraform.tfvars` with:

```hcl
aws_region   = "us-west-1"
project_name = "tritontube"
environment  = "prod"

# Container image (update after ECR push)
# Note: Both backend and worker services use the same image
# Worker uses command override in task definition to run ./worker binary
container_image = "<account-id>.dkr.ecr.us-west-1.amazonaws.com/tritontube-backend:latest"
```

### Optional Variables

```hcl
# ECS Configuration
container_cpu    = 512   # CPU units (1024 = 1 vCPU)
container_memory = 1024  # Memory in MB
desired_count    = 2     # Number of tasks

# Worker Configuration
# Worker binary is selected via ECS command override: ["./worker"]
worker_cpu           = 512
worker_memory        = 1024
worker_desired_count = 1

# Auto Scaling
min_capacity = 2
max_capacity = 10

# Networking
vpc_cidr = "10.0.0.0/16"
```

## 🐳 Deploy Application

After infrastructure is created:

### 1. Build and Push Docker Image

The Dockerfile builds a single image containing both the web API and worker binaries. The worker service uses an ECS command override to run the worker binary.

```bash
# Get ECR login
aws ecr get-login-password --region us-west-1 | \
  docker login --username AWS --password-stdin \
  <account-id>.dkr.ecr.us-west-1.amazonaws.com

# Get ECR repository URL from Terraform output
ECR_URL=$(terraform output -raw ecr_repository_url)

# Build and push
docker build -t tritontube-backend ../.
docker tag tritontube-backend:latest $ECR_URL:latest
docker push $ECR_URL:latest
```

### 2. Update ECS Service

```bash
# Update container_image in terraform.tfvars
container_image = "<your-ecr-url>:latest"

# Apply changes
terraform apply
```

### 3. Deploy Frontend

```bash
# Get bucket name
FRONTEND_BUCKET=$(terraform output -raw frontend_bucket_name)

# Build frontend
cd ../
npm run build

# Deploy to S3
aws s3 sync build/ s3://$FRONTEND_BUCKET/ --delete

# Get CloudFront distribution ID
CDN_ID=$(terraform output -raw frontend_cdn_id)

# Invalidate cache
aws cloudfront create-invalidation \
  --distribution-id $CDN_ID \
  --paths "/*"
```

## 📊 View Outputs

```bash
# See all outputs
terraform output

# Get specific output
terraform output alb_dns_name
terraform output video_cdn_domain
terraform output frontend_cdn_domain
```

## 🔄 Update Infrastructure

```bash
# Make changes to .tf files or terraform.tfvars

# Preview changes
terraform plan

# Apply changes
terraform apply
```

## 🗑️ Destroy Infrastructure

```bash
# CAUTION: This deletes EVERYTHING

# Preview what will be deleted
terraform plan -destroy

# Delete all resources
terraform destroy
```

## 📁 State Management

### Local State (Default)

Terraform stores state in `terraform.tfstate` locally.

**⚠️ IMPORTANT**: Add to `.gitignore`:
```
terraform.tfstate
terraform.tfstate.backup
terraform.tfvars
.terraform/
```

### Remote State (Recommended for Teams)

Uncomment the backend configuration in `main.tf`:

```hcl
terraform {
  backend "s3" {
    bucket         = "tritontube-terraform-state"
    key            = "prod/terraform.tfstate"
    region         = "us-west-1"
    encrypt        = true
    dynamodb_table = "tritontube-terraform-locks"
  }
}
```

Create the S3 bucket and DynamoDB table first:

```bash
# Create state bucket
aws s3 mb s3://tritontube-terraform-state --region us-west-1

# Enable versioning
aws s3api put-bucket-versioning \
  --bucket tritontube-terraform-state \
  --versioning-configuration Status=Enabled

# Create DynamoDB table for locking
aws dynamodb create-table \
  --table-name tritontube-terraform-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

## 🔐 Security Best Practices

1. **Never commit `terraform.tfvars`** - contains configuration values
2. **Never commit `*.tfstate` files** - contain infrastructure details
3. **Enable MFA** on AWS account
4. **Use IAM roles** instead of access keys when possible
5. **Review security groups** before opening to public
6. **Enable CloudTrail** for audit logging
7. **Use remote state** (S3) for team collaboration

## 💰 Cost Optimization

### Development Environment

```hcl
# terraform.tfvars
desired_count     = 1              # Instead of 2
container_cpu     = 256            # Instead of 512
container_memory  = 512            # Instead of 1024
worker_desired_count = 0           # Disable workers if not needed
```

### Stop Resources When Not in Use

```bash
# Scale ECS to 0
aws ecs update-service \
  --cluster tritontube-cluster \
  --service tritontube-service \
  --desired-count 0
```

## 🧪 Multiple Environments

Create separate directories:

```
terraform/
├── environments/
│   ├── dev/
│   │   ├── main.tf
│   │   └── terraform.tfvars
│   ├── staging/
│   │   ├── main.tf
│   │   └── terraform.tfvars
│   └── prod/
│       ├── main.tf
│       └── terraform.tfvars
```

Or use Terraform workspaces:

```bash
# Create workspaces
terraform workspace new dev
terraform workspace new staging
terraform workspace new prod

# Switch between environments
terraform workspace select dev
terraform apply

terraform workspace select prod
terraform apply
```

## 📚 Common Commands

```bash
# Format code
terraform fmt -recursive

# Validate configuration
terraform validate

# Show current state
terraform show

# List resources
terraform state list

# Import existing resource
terraform import aws_s3_bucket.example my-bucket

# Refresh state
terraform refresh

# Target specific resource
terraform apply -target=module.s3

# Use specific var file
terraform apply -var-file="dev.tfvars"
```

## 🐛 Troubleshooting

### "Error acquiring state lock"

Someone else is running Terraform, or previous run failed.

```bash
# Force unlock (use with caution)
terraform force-unlock <lock-id>
```

### "Resource already exists"

Import existing resource:

```bash
terraform import module.s3.aws_s3_bucket.video_content tritontube-video-content
```

### "Invalid credentials"

```bash
# Reconfigure AWS CLI
aws configure

# Or use environment variables
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
```

### Check logs

```bash
# Enable debug logging
export TF_LOG=DEBUG
terraform apply

# Disable
unset TF_LOG
```

## 📖 Additional Resources

- [Terraform Documentation](https://www.terraform.io/docs)
- [AWS Provider Documentation](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [Terraform Best Practices](https://www.terraform-best-practices.com/)

## 🎉 Success!

After successful deployment:

- **Frontend**: `https://<frontend-cdn-domain>`
- **API**: `http://<alb-dns-name>`
- **Video CDN**: `https://<video-cdn-domain>`

View all URLs:
```bash
terraform output deployment_summary
```
