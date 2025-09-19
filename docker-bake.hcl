variable "VERSIONS" {
  default = "nightly"
}

variable versions {
  default = split(",", VERSIONS)
}

group "default" {
  targets = ["gorse-master", "gorse-server", "gorse-worker", "gorse-in-one"]
}

target "image" {
  matrix = {
    component = ["gorse-master", "gorse-server", "gorse-worker", "gorse-in-one"]
    variant   = ["default", "cuda", "mkl"]
  }
  name       = variant == "default" ? component : "${component}-${variant}"
  context    = "."
  dockerfile = variant == "default" ? "cmd/${component}/Dockerfile" : "cmd/${component}/Dockerfile.${variant}"
  platforms  = variant == "default" ? ["linux/amd64", "linux/arm64", "linux/riscv64"] : ["linux/amd64"]
  tags       = variant == "default" ? [for v in versions : "zhenghaoz/${component}:${v}"] : [for v in versions : "zhenghaoz/${component}:${v}-${variant}"]
  cache-from = variant == "default" ? ["type=gha"] : [
    "type=s3,endpoint_url=https://b172f19b7e057975835d8d311a7b0dbd.r2.cloudflarestorage.com/github,region=auto",
  ]
  cache-to   = variant == "default" ? ["type=gha,mode=max"] : [
    "type=s3,endpoint_url=https://b172f19b7e057975835d8d311a7b0dbd.r2.cloudflarestorage.com/github,region=auto,mode=max"
  ]
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
