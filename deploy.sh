#!/usr/bin/env bash
set -e

APP_DIR="/home/worker/solana-token-lab"
DOCKER_COMPOSE_FILE="docker-compose.yml"

echo "===> Deploy started: $(date)"
cd "$APP_DIR"

echo "===> Fetching latest code"
git fetch origin
git reset --hard origin/master

echo "===> Building Docker image"
docker compose -f "$DOCKER_COMPOSE_FILE" build ingest

echo "===> Stopping old conrainers"
docker compose -f "$DOCKER_COMPOSE_FILE" down

echo "===> Start new containers"
docker compose --profile ingest up -d --force-recreate

echo "===> Cleaning unused images"
docker image prune -f

echo "===> Deploy finished successfully"
