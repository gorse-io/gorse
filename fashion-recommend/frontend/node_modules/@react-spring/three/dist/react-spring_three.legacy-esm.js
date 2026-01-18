// src/index.ts
import { applyProps, addEffect } from "@react-three/fiber";
import { Globals } from "@react-spring/core";
import { createStringInterpolator, colors, raf } from "@react-spring/shared";
import { createHost } from "@react-spring/animated";

// src/primitives.ts
import * as THREE from "three";
import "@react-three/fiber";
var primitives = ["primitive"].concat(
  Object.keys(THREE).filter((key) => /^[A-Z]/.test(key)).map((key) => key[0].toLowerCase() + key.slice(1))
);

// src/index.ts
export * from "@react-spring/core";
Globals.assign({
  createStringInterpolator,
  colors,
  frameLoop: "demand"
});
addEffect(() => {
  raf.advance();
});
var host = createHost(primitives, {
  applyAnimatedValues: applyProps
});
var animated = host.animated;
export {
  animated as a,
  animated
};
