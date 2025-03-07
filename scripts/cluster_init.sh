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
kubectl create secret generic discord-secrets -n hearthhub --from-literal=DISCORD_CLIENT_ID="$DISCORD_CLIENT_ID" --from-literal=DISCORD_CLIENT_SECRET="$DISCORD_CLIENT_SECRET"
kubectl create secret generic cognito-secrets -n hearthhub --from-literal=USER_POOL_ID="$USER_POOL_ID" --from-literal=COGNITO_CLIENT_ID="$COGNITO_CLIENT_ID" --from-literal=COGNITO_CLIENT_SECRET="$COGNITO_CLIENT_SECRET"

# Duplicate the secrets in both namespaces since the client resisdes in hearthhub and the server resides in rabbitmq. We need credentials in both to form the connection
kubectl create secret generic rabbitmq-secrets -n rabbitmq --from-literal=RABBITMQ_DEFAULT_USER="$RABBITMQ_DEFAULT_USER" --from-literal=RABBITMQ_DEFAULT_PASS="$RABBITMQ_DEFAULT_PASS"
kubectl create secret generic rabbitmq-secrets -n hearthhub --from-literal=RABBITMQ_DEFAULT_USER="$RABBITMQ_DEFAULT_USER" --from-literal=RABBITMQ_DEFAULT_PASS="$RABBITMQ_DEFAULT_PASS"

kubectl create secret generic stripe-secrets-test -n hearthhub --from-literal=STRIPE_SECRET_KEY="$STRIPE_TEST_SECRET_KEY" --from-literal=STRIPE_ENDPOINT_SECRET="$STRIPE_TEST_ENDPOINT_SECRET"
kubectl create secret generic stripe-secrets-live -n hearthhub --from-literal=STRIPE_SECRET_KEY="$STRIPE_LIVE_SECRET_KEY" --from-literal=STRIPE_ENDPOINT_SECRET="$STRIPE_LIVE_ENDPOINT_SECRET"

kubectl create secret generic mod-nexus-secrets -n hearthhub --from-literal=MOD_NEXUS_API_KEY="$MOD_NEXUS_API_KEY"

# MySQL DB
kubectl create secret generic mysql-secrets -n hearthhub --from-literal=MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" --from-literal=MYSQL_PASSWORD="$MYSQL_PASSWORD" --from-literal=MYSQL_DATABASE="$MYSQL_DATABASE" --from-literal=MYSQL_USER="$MYSQL_USER"

echo "Successfully scaffolded Kubernetes cluster for HearthHub!"