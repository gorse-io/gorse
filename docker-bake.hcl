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

group "windows" {
  targets = ["gorse-master-windows", "gorse-server-windows", "gorse-worker-windows", "gorse-in-one-windows"]
}

target "gorse-master-windows" {
  context = "."
  dockerfile = "cmd/gorse-master/Dockerfile.windows"
  platforms = ["windows/amd64"]
  tags = ["zhenghaoz/gorse-master:nightly-windowsservercore"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-server-windows" {
  context = "."
  dockerfile = "cmd/gorse-server/Dockerfile.windows"
  platforms = ["windows/amd64"]
  tags = ["zhenghaoz/gorse-server:nightly-windowsservercore"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-worker-windows" {
  context = "."
  dockerfile = "cmd/gorse-worker/Dockerfile.windows"
  platforms = ["windows/amd64"]
  tags = ["zhenghaoz/gorse-worker:nightly-windowsservercore"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-in-one-windows" {
  context = "."
  dockerfile = "cmd/gorse-in-one/Dockerfile.windows"
  platforms = ["windows/amd64"]
  tags = ["zhenghaoz/gorse-in-one:nightly-windowsservercore"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

group "cuda" {
  targets = ["gorse-master-cuda", "gorse-server-cuda", "gorse-worker-cuda", "gorse-in-one-cuda"]
}

target "gorse-master-cuda" {
  context = "."
  dockerfile = "cmd/gorse-master/Dockerfile.cuda"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["zhenghaoz/gorse-master:nightly-cuda12.8"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-server-cuda" {
  context = "."
  dockerfile = "cmd/gorse-server/Dockerfile.cuda"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["zhenghaoz/gorse-server:nightly-cuda12.8"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-worker-cuda" {
  context = "."
  dockerfile = "cmd/gorse-worker/Dockerfile.cuda"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["zhenghaoz/gorse-worker:nightly-cuda12.8"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-in-one-cuda" {
  context = "."
  dockerfile = "cmd/gorse-in-one/Dockerfile.cuda"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["zhenghaoz/gorse-in-one:nightly-cuda12.8"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

group "mkl" {
  targets = ["gorse-master-mkl", "gorse-server-mkl", "gorse-worker-mkl", "gorse-in-one-mkl"]
}

target "gorse-master-mkl" {
  context = "."
  dockerfile = "cmd/gorse-master/Dockerfile.mkl"
  platforms = ["linux/amd64"]
  tags = ["zhenghaoz/gorse-master:nightly-mkl"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-server-mkl" {
  context = "."
  dockerfile = "cmd/gorse-server/Dockerfile.mkl"
  platforms = ["linux/amd64"]
  tags = ["zhenghaoz/gorse-server:nightly-mkl"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-worker-mkl" {
  context = "."
  dockerfile = "cmd/gorse-worker/Dockerfile.mkl"
  platforms = ["linux/amd64"]
  tags = ["zhenghaoz/gorse-worker:nightly-mkl"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}

target "gorse-in-one-mkl" {
  context = "."
  dockerfile = "cmd/gorse-in-one/Dockerfile.mkl"
  platforms = ["linux/amd64"]
  tags = ["zhenghaoz/gorse-in-one:nightly-mkl"]
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
}
