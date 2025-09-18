group "default" {
  targets = ["gorse-master", "gorse-server", "gorse-worker", "gorse-in-one"]
}

target "gorse-master" {
  context = "."
  dockerfile = "cmd/gorse-master/Dockerfile"
  platforms = ["linux/amd64", "linux/arm64", "linux/riscv64"]
  tags = ["zhenghaoz/gorse-master:nightly"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-server" {
  context = "."
  dockerfile = "cmd/gorse-server/Dockerfile"
  platforms = ["linux/amd64", "linux/arm64", "linux/riscv64"]
  tags = ["zhenghaoz/gorse-server:nightly"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-worker" {
  context = "."
  dockerfile = "cmd/gorse-worker/Dockerfile"
  platforms = ["linux/amd64", "linux/arm64", "linux/riscv64"]
  tags = ["zhenghaoz/gorse-worker:nightly"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-in-one" {
  context = "."
  dockerfile = "cmd/gorse-in-one/Dockerfile"
  platforms = ["linux/amd64", "linux/arm64", "linux/riscv64"]
  tags = ["zhenghaoz/gorse-in-one:nightly"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "image" {
  matrix = {
    component = ["gorse-master", "gorse-server", "gorse-worker", "gorse-in-one"]
    variant = ["cuda", "mkl"]
  }
  name = "${component}-${variant}"
  context = "."
  dockerfile = "cmd/${component}/Dockerfile.${variant}"
  platforms = ["linux/amd64"]
  tags = ["zhenghaoz/${component}:nightly-${variant}"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

group "cuda" {
  targets = [
    "gorse-master-cuda",
    "gorse-server-cuda",
    "gorse-worker-cuda",
    "gorse-in-one-cuda",
  ]
}

group "mkl" {
  targets = [
    "gorse-master-mkl",
    "gorse-server-mkl",
    "gorse-worker-mkl",
    "gorse-in-one-mkl",
  ]
}
