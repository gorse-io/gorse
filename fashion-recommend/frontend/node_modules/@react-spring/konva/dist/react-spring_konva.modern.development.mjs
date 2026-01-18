// src/index.ts
import { Globals, createStringInterpolator, colors } from "@react-spring/shared";
import { createHost } from "@react-spring/animated";

// src/primitives.ts
var primitives = [
  "Arc",
  "Arrow",
  "Circle",
  "Ellipse",
  "FastLayer",
  "Group",
  "Image",
  "Label",
  "Layer",
  "Line",
  "Path",
  "Rect",
  "RegularPolygon",
  "Ring",
  "Shape",
  "Sprite",
  "Star",
  "Tag",
  "Text",
  "TextPath",
  "Transformer",
  "Wedge"
];

// src/index.ts
export * from "@react-spring/core";
Globals.assign({
  createStringInterpolator,
  colors
});
var host = createHost(primitives, {
  applyAnimatedValues(instance, props) {
    if (!instance.nodeType) return false;
    instance._applyProps(instance, props);
  }
});
var animated = host.animated;
export {
  animated as a,
  animated
};
