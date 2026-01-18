// src/index.ts
import { applyProps } from "react-zdog";
import { Globals } from "@react-spring/core";
import { createStringInterpolator, colors } from "@react-spring/shared";
import { createHost } from "@react-spring/animated";

// src/primitives.ts
import * as Zdog from "react-zdog";
var primitives = {
  Anchor: Zdog.Anchor,
  Shape: Zdog.Shape,
  Group: Zdog.Group,
  Rect: Zdog.Rect,
  RoundedRect: Zdog.RoundedRect,
  Ellipse: Zdog.Ellipse,
  Polygon: Zdog.Polygon,
  Hemisphere: Zdog.Hemisphere,
  Cylinder: Zdog.Cylinder,
  Cone: Zdog.Cone,
  Box: Zdog.Box
};

// src/index.ts
export * from "@react-spring/core";
Globals.assign({
  createStringInterpolator,
  colors
});
var host = createHost(primitives, {
  applyAnimatedValues: applyProps
});
var animated = host.animated;
export {
  animated as a,
  animated
};
