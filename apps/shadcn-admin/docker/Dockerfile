# syntax=docker/dockerfile:1
# Build from monorepo root: context `../..`, dockerfile `apps/shadcn-admin/docker/Dockerfile`
# (pnpm lockfile + workspace; Vite envDir is repo root).

FROM node:24-bookworm-slim AS builder

WORKDIR /workspace

ENV CI=true

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages ./packages
COPY apps/shadcn-admin/package.json ./apps/shadcn-admin/

RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile --filter chexi-trading-admin

COPY apps/shadcn-admin ./apps/shadcn-admin

ARG VITE_GOAPI_BASE_URL=
ENV VITE_GOAPI_BASE_URL=$VITE_GOAPI_BASE_URL

ARG VITE_CLERK_PUBLISHABLE_KEY=
ENV VITE_CLERK_PUBLISHABLE_KEY=$VITE_CLERK_PUBLISHABLE_KEY

RUN pnpm --filter chexi-trading-admin run build

FROM nginx:1.27-alpine

COPY apps/shadcn-admin/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /workspace/apps/shadcn-admin/dist /usr/share/nginx/html

EXPOSE 80
