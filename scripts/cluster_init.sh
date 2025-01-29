#!/usr/bin/bash

set -e

# Load env vars
source .env

# Create the Hearthhub Namespace
kubectl create ns hearthhub
kubens hearthhub

# Create secrets in the hearthhub namespace
kubectl create secret generic aws-creds -n hearthhub --from-literal=AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" --from-literal=AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY"
kubectl create secret generic cognito-secrets -n hearthhub --from-literal=USER_POOL_ID="$USER_POOL_ID" --from-literal=COGNITO_CLIENT_ID="$COGNITO_CLIENT_ID" --from-literal=COGNITO_CLIENT_SECRET="$COGNITO_CLIENT_SECRET"

echo "Successfully scaffolded Kubernetes cluster for HearthHub!"