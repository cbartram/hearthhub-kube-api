#!/bin/bash
#
# Note: The local server makes real requests to the kubernetes cluster however, when depending on websockets for actions
# like deleting the server this RabbitMQ instance will not help since it won't have any consumers.
#

# The user/password values are for the local rabbitmq server during development
# and are not used in production.
docker run -d \
  --name rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER="hearthhub" \
  -e RABBITMQ_DEFAULT_PASS="O3iU0YfG716C" \
  -m 1024m \
  --cpus=0.3 \
  rabbitmq:4.0.5-management
