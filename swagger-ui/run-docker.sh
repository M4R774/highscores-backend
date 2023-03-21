#!/bin/bash
docker kill swagger
docker run -d -n swagger --restart always -p 8888:8080 -e SWAGGER_JSON=/openapi-definition.yaml -v ./swagger-ui/openapi-definition.yaml:/openapi-definition.yaml pentusha/swagger-ui-crossbuild:latest
