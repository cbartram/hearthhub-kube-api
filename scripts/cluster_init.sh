#!/usr/bin/bash

set -e

# Load env vars
source .env

# Create the Hearthhub Namespace
kubectl create ns hearthhub
kubectl create ns rabbitmq
kubens hearthhub

# Create secrets in the hearthhub & rabbitmq namespace
kubectl create secret generic aws-creds -n hearthhub --from-literal=AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" --from-literal=AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY"
kubectl create secret generic cognito-secrets -n hearthhub --from-literal=USER_POOL_ID="$USER_POOL_ID" --from-literal=COGNITO_CLIENT_ID="$COGNITO_CLIENT_ID" --from-literal=COGNITO_CLIENT_SECRET="$COGNITO_CLIENT_SECRET"

# Duplicate the secrets in both namespaces since the client resisdes in hearthhub and the server resides in rabbitmq. We need credentials in both to form the connection
kubectl create secret generic rabbitmq-secrets -n rabbitmq --from-literal=RABBITMQ_DEFAULT_USER="$RABBITMQ_DEFAULT_USER" --from-literal=RABBITMQ_DEFAULT_PASS="$RABBITMQ_DEFAULT_PASS"
kubectl create secret generic rabbitmq-secrets -n hearthhub --from-literal=RABBITMQ_DEFAULT_USER="$RABBITMQ_DEFAULT_USER" --from-literal=RABBITMQ_DEFAULT_PASS="$RABBITMQ_DEFAULT_PASS"

echo "Successfully scaffolded Kubernetes cluster for HearthHub!"