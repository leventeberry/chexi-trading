# Terraform Infrastructure

This directory is reserved for future Terraform infrastructure for Chexi. No modules are implemented yet.

The first AWS target is documented in `../../docs/AWS_DEPLOYMENT_PLAN.md`.

## Intended AWS Stack

- ECS Fargate for the existing API container.
- ECR for API container images.
- RDS PostgreSQL for the database.
- ElastiCache Redis for cache, rate limiting, and queue-related Redis usage.
- Secrets Manager for runtime secrets.
- Application Load Balancer for HTTPS ingress.
- CloudWatch Logs and alarms for observability.
- S3 for backups/assets.
- Route 53 and ACM for DNS and TLS.

## Planned Structure

```text
infra/terraform/
  README.md
  modules/
    network/
    ecs-api/
    rds-postgres/
    elasticache-redis/
    alb/
    secrets/
    observability/
    s3/
    dns/
  environments/
    dev/
      backend.tf
      providers.tf
      main.tf
      variables.tf
      terraform.tfvars.example
    staging/
      backend.tf
      providers.tf
      main.tf
      variables.tf
      terraform.tfvars.example
    production/
      backend.tf
      providers.tf
      main.tf
      variables.tf
      terraform.tfvars.example
```

## Environment Model

- `dev`: low-cost cloud sandbox for validating infrastructure and ECS startup.
- `staging`: production-like release validation with sanitized data.
- `production`: customer-facing environment with deletion protection, backups, scaling, and alerts.

## Implementation Rules

- Keep local development on Docker Compose in `infra/docker`.
- Do not store real secrets in Terraform files or committed tfvars.
- Use S3 remote state and DynamoDB state locking before shared environments are applied.
- Keep modules reusable and environment-specific values in `environments/<env>`.
- Add security scanning, formatting, validation, and plan review to CI before applying infrastructure.

## Future Checklist

- [ ] Create remote state resources.
- [ ] Add provider/version constraints.
- [ ] Add environment skeletons.
- [ ] Implement network module.
- [ ] Implement ECR and ECS API module.
- [ ] Implement RDS PostgreSQL module.
- [ ] Implement ElastiCache Redis module.
- [ ] Implement ALB, Route 53, and ACM modules.
- [ ] Implement Secrets Manager module.
- [ ] Implement CloudWatch logs, alarms, and dashboards.
- [ ] Implement S3 backups/assets module.
- [ ] Add Terraform CI checks and deployment approvals.
