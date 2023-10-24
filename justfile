set dotenv-load

deploy-docker:
  docker login -u $JUSTFILE_DOCKER_USERNAME -p $JUSTFILE_DOCKER_TOKEN
  docker build -t neurastudios/gorse-master:latest -f cmd/gorse-master/Dockerfile .
  docker build -t neurastudios/gorse-server:latest -f cmd/gorse-server/Dockerfile .
  docker build -t neurastudios/gorse-worker:latest -f cmd/gorse-worker/Dockerfile .
  docker push neurastudios/gorse-master:latest
  docker push neurastudios/gorse-server:latest
  docker push neurastudios/gorse-worker:latest