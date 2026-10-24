variable "GORSE_VERSION" {
  default = ""
}

variable "GORSE_GIT_COMMIT" {
  default = ""
}

variable "GORSE_BUILD_TIME" {
  default = ""
}

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

target "openblas" {
  args = {
    VERSION    = GORSE_VERSION
    GIT_COMMIT = GORSE_GIT_COMMIT
    BUILD_TIME = GORSE_BUILD_TIME
  }
  matrix = {
    component = components
  }
  name       = component
  context    = "."
  dockerfile = "cmd/${component}/Dockerfile.openblas"
  platforms  = ["linux/amd64", "linux/arm64", "linux/riscv64"]
  tags       = [for v in versions : "zhenghaoz/${component}:${v}"]
  cache-from = ["type=registry,ref=ghcr.io/gorse-io/gorse-buildcache:${component}-openblas"]
  cache-to   = ["type=registry,ref=ghcr.io/gorse-io/gorse-buildcache:${component}-openblas,mode=max"]
}

target "cuda" {
  args = {
    VERSION    = GORSE_VERSION
    GIT_COMMIT = GORSE_GIT_COMMIT
    BUILD_TIME = GORSE_BUILD_TIME
  }
  matrix = {
    component = components
  }
  name       = "${component}-cuda"
  context    = "."
  dockerfile = "cmd/${component}/Dockerfile.cuda"
  platforms  = ["linux/amd64"]
  tags       = [for v in versions : "zhenghaoz/${component}:${v}-cuda12.8"]
  cache-from = ["type=registry,ref=ghcr.io/gorse-io/gorse-buildcache:${component}-cuda"]
  cache-to   = ["type=registry,ref=ghcr.io/gorse-io/gorse-buildcache:${component}-cuda,mode=max"]
}

target "mkl" {
  args = {
    VERSION    = GORSE_VERSION
    GIT_COMMIT = GORSE_GIT_COMMIT
    BUILD_TIME = GORSE_BUILD_TIME
  }
  matrix = {
    component = components
  }
  name       = "${component}-mkl"
  context    = "."
  dockerfile = "cmd/${component}/Dockerfile.mkl"
  platforms  = ["linux/amd64"]
  tags       = [for v in versions : "zhenghaoz/${component}:${v}-mkl"]
  cache-from = ["type=registry,ref=ghcr.io/gorse-io/gorse-buildcache:${component}-mkl"]
  cache-to   = ["type=registry,ref=ghcr.io/gorse-io/gorse-buildcache:${component}-mkl,mode=max"]
}
