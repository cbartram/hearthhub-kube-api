#!/bin/bash

# Note: The local server makes real requests to the kubernetes cluster however, when depending on websockets for actions
# like deleting the server this RabbitMQ instance will not help since the sidecar and file manager on the kube cluster
# send messages to their rabbitmq queue not this local one. Since react consumes from local rabbit mq and messages are
# sent to cluster rabbitmq no websocket messages will comes in during local development

# The user/password values are for the local rabbitmq server during development
# and are not used in production. Make sure your `.env` file has these values for RABBIT_MQ_DEFAULT_USER and PASS.
docker run -d \
  --name rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER="hearthhub" \
  -e RABBITMQ_DEFAULT_PASS="O3iU0YfG716C" \
  -m 1024m \
  --cpus=0.3 \
  rabbitmq:4.0.5-management
