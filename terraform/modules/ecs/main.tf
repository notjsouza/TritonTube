# ECR Repository
resource "aws_ecr_repository" "backend" {
  name                 = "${var.project_name}-backend"
  image_tag_mutability = "MUTABLE"
  force_delete         = true  # Allow deletion even with images

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = {
    Name = "${var.project_name}-backend"
  }
}

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = {
    Name = "${var.project_name}-cluster"
  }
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "ecs" {
  name              = "/ecs/${var.project_name}"
  retention_in_days = 7

  tags = {
    Name = "${var.project_name}-ecs-logs"
  }
}

# Security Group for ALB
resource "aws_security_group" "alb" {
  name        = "${var.project_name}-alb-sg"
  description = "Security group for ALB"
  vpc_id      = var.vpc_id

  ingress {
    description = "HTTP from anywhere"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS from anywhere"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-alb-sg"
  }
}

# Security Group for ECS Tasks
resource "aws_security_group" "ecs_tasks" {
  name        = "${var.project_name}-ecs-tasks-sg"
  description = "Security group for ECS tasks"
  vpc_id      = var.vpc_id

  ingress {
    description     = "Allow from ALB"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-ecs-tasks-sg"
  }
}

# Application Load Balancer
resource "aws_lb" "main" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.public_subnet_ids

  enable_deletion_protection = false
  idle_timeout               = 300 # 5 minutes for video upload processing

  tags = {
    Name = "${var.project_name}-alb"
  }
}

# Target Group
resource "aws_lb_target_group" "main" {
  name        = "${var.project_name}-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    path                = "/api/videos"
    matcher             = "200"
  }

  deregistration_delay = 30

  tags = {
    Name = "${var.project_name}-tg"
  }
}

# ALB Listener
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main.arn
  }
}

# Dead-letter queue — receives messages that fail processing maxReceiveCount times
resource "aws_sqs_queue" "upload_jobs_dlq" {
  name                      = "${var.project_name}-upload-jobs-dlq"
  message_retention_seconds = 1209600 # 14 days — long enough to investigate and replay

  tags = {
    Name = "${var.project_name}-upload-jobs-dlq"
  }
}

# Main SQS queue for upload processing jobs
resource "aws_sqs_queue" "upload_jobs" {
  name                       = "${var.project_name}-upload-jobs"
  delay_seconds              = 0
  visibility_timeout_seconds = 1800 # 30 minutes to allow long FFmpeg processing
  message_retention_seconds  = 1209600 # 14 days
  receive_wait_time_seconds  = 20

  # After 3 failed processing attempts, route to DLQ
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.upload_jobs_dlq.arn
    maxReceiveCount     = 3
  })

  tags = {
    Name = "${var.project_name}-upload-jobs"
  }
}

# IAM role and policy for ECS tasks (allows SQS SendMessage for API and SQS Receive/Delete + S3 access for worker)
# DynamoDB Table for video metadata
resource "aws_dynamodb_table" "video_metadata" {
  name           = "${var.project_name}-video-metadata"
  billing_mode   = "PAY_PER_REQUEST" # On-demand pricing
  hash_key       = "id"

  attribute {
    name = "id"
    type = "S"
  }

  tags = {
    Name = "${var.project_name}-video-metadata"
  }
}

/*
  Worker ECS task & service: lightweight task that runs the `worker` image (same ECR repo) and polls SQS.
  This is created as a separate task definition & service but reuses the cluster and log group.
*/

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.project_name}-worker"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.worker_cpu
  memory                   = var.worker_memory
  execution_role_arn       = var.task_execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([
    {
      name  = "${var.project_name}-worker"
  image = var.worker_image != "" ? var.worker_image : var.container_image
      command = ["./worker"]

      environment = [
        { name = "SQS_QUEUE_URL", value = aws_sqs_queue.upload_jobs.id },
        { name = "S3_BUCKET_NAME", value = var.s3_bucket },
        { name = "S3_UPLOADS_BUCKET_NAME", value = var.uploads_bucket },
        { name = "METADATA_TYPE", value = "dynamodb" },
        { name = "METADATA_OPTIONS", value = aws_dynamodb_table.video_metadata.name },
        { name = "CONTENT_TYPE", value = "s3" },
        { name = "CONTENT_OPTIONS", value = var.s3_bucket },
        { name = "AWS_REGION", value = data.aws_region.current.name }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.ecs.name
          "awslogs-region"        = data.aws_region.current.name
          "awslogs-stream-prefix" = "worker"
        }
      }

      essential = true
    }
  ])

  tags = {
    Name = "${var.project_name}-worker-task"
  }
}

resource "aws_ecs_service" "worker" {
  name            = "${var.project_name}-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }

  tags = {
    Name = "${var.project_name}-worker"
  }
}

# ECS Task Definition
resource "aws_ecs_task_definition" "main" {
  family                   = "${var.project_name}-backend"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.container_cpu
  memory                   = var.container_memory
  execution_role_arn       = var.task_execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([
    {
      name  = "${var.project_name}-backend"
      image = var.container_image

      portMappings = [
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]

      environment = [
        {
          name  = "METADATA_TYPE"
          value = "dynamodb"
        },
        {
          name  = "METADATA_OPTIONS"
          value = aws_dynamodb_table.video_metadata.name
        },
        {
          name  = "CONTENT_TYPE"
          value = "s3"
        },
        {
          name  = "CONTENT_OPTIONS"
          value = var.s3_bucket
        },
        {
          name  = "S3_BUCKET_NAME"
          value = var.s3_bucket
        },
        {
          name  = "S3_UPLOADS_BUCKET_NAME"
          value = var.uploads_bucket
        },
        {
          name  = "SQS_QUEUE_URL"
          value = aws_sqs_queue.upload_jobs.id
        },
        {
          name  = "AWS_REGION"
          value = data.aws_region.current.name
        },
        {
          name  = "CDN_DOMAIN"
          value = var.cdn_domain
        }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.ecs.name
          "awslogs-region"        = data.aws_region.current.name
          "awslogs-stream-prefix" = "backend"
        }
      }

      essential = true
    }
  ])

  tags = {
    Name = "${var.project_name}-backend-task"
  }
}

# ECS Service
resource "aws_ecs_service" "main" {
  name            = "${var.project_name}-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.main.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.main.arn
    container_name   = "${var.project_name}-backend"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]

  tags = {
    Name = "${var.project_name}-service"
  }
}

# Auto Scaling Target
resource "aws_appautoscaling_target" "ecs" {
  max_capacity       = 10
  min_capacity       = 2
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.main.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

# Auto Scaling Policy - CPU
resource "aws_appautoscaling_policy" "ecs_cpu" {
  name               = "${var.project_name}-cpu-autoscaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 70.0
    scale_in_cooldown  = 60
    scale_out_cooldown = 60
  }
}

# Auto Scaling Policy - Memory
resource "aws_appautoscaling_policy" "ecs_memory" {
  name               = "${var.project_name}-memory-autoscaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageMemoryUtilization"
    }
    target_value       = 80.0
    scale_in_cooldown  = 60
    scale_out_cooldown = 60
  }
}

# ── Worker Auto Scaling ────────────────────────────────────────────────────────

# Register the worker service as an autoscaling target
resource "aws_appautoscaling_target" "worker" {
  count = var.enable_autoscaling ? 1 : 0

  min_capacity       = var.worker_min_count
  max_capacity       = var.worker_max_count
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.worker.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

# Alarm: queue has waiting messages → scale out
resource "aws_cloudwatch_metric_alarm" "worker_scale_out" {
  count = var.enable_autoscaling ? 1 : 0

  alarm_name          = "${var.project_name}-worker-scale-out"
  alarm_description   = "Workers needed: messages are waiting in the upload queue"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  metric_name         = "ApproximateNumberOfMessagesVisible"
  namespace           = "AWS/SQS"
  period              = 60
  statistic           = "Maximum"
  threshold           = 1
  treat_missing_data  = "notBreaching"

  dimensions = {
    QueueName = aws_sqs_queue.upload_jobs.name
  }

  alarm_actions = var.enable_autoscaling ? [aws_appautoscaling_policy.worker_scale_out[0].arn] : []

  tags = {
    Name = "${var.project_name}-worker-scale-out"
  }
}

# Alarm: queue empty (visible + in-flight = 0) for 5 consecutive minutes → scale in
# Using metric math on both Visible and NotVisible avoids scaling in while
# workers are processing messages (NotVisible > 0) but the visible queue is empty.
resource "aws_cloudwatch_metric_alarm" "worker_scale_in" {
  count = var.enable_autoscaling ? 1 : 0

  alarm_name          = "${var.project_name}-worker-scale-in"
  alarm_description   = "No pending uploads: scale in workers after queue fully drains"
  comparison_operator = "LessThanOrEqualToThreshold"
  evaluation_periods  = 5
  threshold           = 0
  treat_missing_data  = "notBreaching"

  metric_query {
    id = "m_visible"
    metric {
      namespace   = "AWS/SQS"
      metric_name = "ApproximateNumberOfMessagesVisible"
      period      = 60
      stat        = "Average"
      dimensions = {
        QueueName = aws_sqs_queue.upload_jobs.name
      }
    }
  }

  metric_query {
    id = "m_not_visible"
    metric {
      namespace   = "AWS/SQS"
      metric_name = "ApproximateNumberOfMessagesNotVisible"
      period      = 60
      stat        = "Average"
      dimensions = {
        QueueName = aws_sqs_queue.upload_jobs.name
      }
    }
  }

  metric_query {
    id          = "m_total"
    expression  = "m_visible + m_not_visible"
    label       = "TotalMessages"
    return_data = true
  }

  alarm_actions = var.enable_autoscaling ? [aws_appautoscaling_policy.worker_scale_in[0].arn] : []

  tags = {
    Name = "${var.project_name}-worker-scale-in"
  }
}

# Scale-out policy: add workers proportional to queue depth
# Bounds are relative to alarm threshold (1), so:
#   [0, 4)  → queue has 1-4  msgs → +1 worker
#   [4, 9)  → queue has 5-9  msgs → +2 workers
#   [9, ∞)  → queue has 10+  msgs → +4 workers
resource "aws_appautoscaling_policy" "worker_scale_out" {
  count = var.enable_autoscaling ? 1 : 0

  name               = "${var.project_name}-worker-scale-out"
  policy_type        = "StepScaling"
  resource_id        = aws_appautoscaling_target.worker[0].resource_id
  scalable_dimension = aws_appautoscaling_target.worker[0].scalable_dimension
  service_namespace  = aws_appautoscaling_target.worker[0].service_namespace

  step_scaling_policy_configuration {
    adjustment_type         = "ChangeInCapacity"
    cooldown                = 60
    metric_aggregation_type = "Maximum"

    step_adjustment {
      metric_interval_lower_bound = 0
      metric_interval_upper_bound = 4
      scaling_adjustment          = 1
    }

    step_adjustment {
      metric_interval_lower_bound = 4
      metric_interval_upper_bound = 9
      scaling_adjustment          = 2
    }

    step_adjustment {
      metric_interval_lower_bound = 9
      scaling_adjustment          = 4
    }
  }
}

# Scale-in policy: remove 1 worker at a time
# Cooldown matches the SQS visibility timeout (1800s) so we don't terminate
# a task that is mid-job but temporarily showing an empty visible queue.
resource "aws_appautoscaling_policy" "worker_scale_in" {
  count = var.enable_autoscaling ? 1 : 0

  name               = "${var.project_name}-worker-scale-in"
  policy_type        = "StepScaling"
  resource_id        = aws_appautoscaling_target.worker[0].resource_id
  scalable_dimension = aws_appautoscaling_target.worker[0].scalable_dimension
  service_namespace  = aws_appautoscaling_target.worker[0].service_namespace

  step_scaling_policy_configuration {
    adjustment_type         = "ChangeInCapacity"
    cooldown                = 1800 # matches SQS visibility timeout
    metric_aggregation_type = "Maximum"

    step_adjustment {
      metric_interval_upper_bound = 0
      scaling_adjustment          = -1
    }
  }
}

# Data source for current region
data "aws_region" "current" {}
