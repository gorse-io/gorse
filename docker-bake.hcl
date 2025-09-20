variable "VERSIONS" {
  default = "nightly"
}

variable versions {
  default = split(",", VERSIONS)
}

variable components {
  default = ["gorse-master", "gorse-server", "gorse-worker", "gorse-in-one"]
}

group "default" {
  targets = ["gorse-master", "gorse-server", "gorse-worker", "gorse-in-one"]
}

target "image" {
  matrix = {
    component = components
  }
  name       = component
  context    = "."
  dockerfile = "cmd/${component}/Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64", "linux/riscv64"]
  tags       = [for v in versions : "zhenghaoz/${component}:${v}"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}

target "cuda" {
  matrix = {
    component = components
  }
  name       = "${component}-cuda"
  context    = "."
  dockerfile = "cmd/${component}/Dockerfile.cuda"
  platforms  = ["linux/amd64"]
  tags       = [for v in versions : "zhenghaoz/${component}:${v}-cuda12.8"]
  cache-from = ["type=s3,endpoint_url=https://b172f19b7e057975835d8d311a7b0dbd.r2.cloudflarestorage.com,bucket=github,region=auto"]
  cache-to   = ["type=s3,endpoint_url=https://b172f19b7e057975835d8d311a7b0dbd.r2.cloudflarestorage.com,bucket=github,region=auto,mode=max"]
}

target "mkl" {
  matrix = {
    component = components
  }
  name       = "${component}-mkl"
  context    = "."
  dockerfile = "cmd/${component}/Dockerfile.mkl"
  platforms  = ["linux/amd64"]
  tags       = [for v in versions : "zhenghaoz/${component}:${v}-mkl"]
  cache-from = ["type=s3,endpoint_url=https://b172f19b7e057975835d8d311a7b0dbd.r2.cloudflarestorage.com,bucket=github,region=auto"]
  cache-to   = ["type=s3,endpoint_url=https://b172f19b7e057975835d8d311a7b0dbd.r2.cloudflarestorage.com,bucket=github,region=auto,mode=max"]
}
