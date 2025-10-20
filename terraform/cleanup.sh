#!/bin/bash
# Cleanup script for TritonTube resources
# Run this before terraform destroy if you get "not empty" errors

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║         TritonTube Cleanup Script                             ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Get AWS region
AWS_REGION=$(aws configure get region)
if [ -z "$AWS_REGION" ]; then
    AWS_REGION="us-west-1"
fi

PROJECT_NAME="tritontube"

echo "Cleaning up resources in region: $AWS_REGION"
echo "Project: $PROJECT_NAME"
echo ""

# Check if user wants to proceed
read -p "This will delete ALL data in S3 buckets and ECR. Continue? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Cleanup cancelled"
    exit 0
fi

echo ""

# 1. Empty S3 buckets
echo "═══════════════════════════════════════════════════════════════"
echo "Step 1: Emptying S3 Buckets"
echo "═══════════════════════════════════════════════════════════════"
echo ""

for BUCKET in "${PROJECT_NAME}-video-content" "${PROJECT_NAME}-frontend" "${PROJECT_NAME}-uploads"; do
    if aws s3 ls "s3://$BUCKET" 2>/dev/null; then
        echo "Emptying bucket: $BUCKET"
        aws s3 rm "s3://$BUCKET" --recursive --region $AWS_REGION
        echo "   $BUCKET emptied"
    else
        echo "Bucket $BUCKET doesn't exist or already empty"
    fi
done

echo ""

# 2. Delete ECR images
echo "═══════════════════════════════════════════════════════════════"
echo "Step 2: Deleting ECR Images"
echo "═══════════════════════════════════════════════════════════════"
echo ""

REPO_NAME="${PROJECT_NAME}-backend"
if aws ecr describe-repositories --repository-names $REPO_NAME --region $AWS_REGION 2>/dev/null; then
    echo "Deleting all images in ECR repository: $REPO_NAME"
    
    # Get all image tags
    IMAGE_TAGS=$(aws ecr list-images \
        --repository-name $REPO_NAME \
        --region $AWS_REGION \
        --query 'imageIds[*].imageTag' \
        --output text)
    
    if [ -n "$IMAGE_TAGS" ]; then
        for TAG in $IMAGE_TAGS; do
            aws ecr batch-delete-image \
                --repository-name $REPO_NAME \
                --region $AWS_REGION \
                --image-ids imageTag=$TAG >/dev/null 2>&1 || true
            echo "   Deleted image: $TAG"
        done
        echo "   ✓ All images deleted from $REPO_NAME"
    else
        echo "    No images found in $REPO_NAME"
    fi
else
    echo "     ECR repository $REPO_NAME doesn't exist"
fi

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║              Cleanup Complete!                                 ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "You can now run: terraform destroy"
echo ""
