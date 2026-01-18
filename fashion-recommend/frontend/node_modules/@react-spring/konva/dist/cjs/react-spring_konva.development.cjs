"use strict";
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __reExport = (target, mod, secondTarget) => (__copyProps(target, mod, "default"), secondTarget && __copyProps(secondTarget, mod, "default"));
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// src/index.ts
var src_exports = {};
__export(src_exports, {
  a: () => animated,
  animated: () => animated
});
module.exports = __toCommonJS(src_exports);
var import_shared = require("@react-spring/shared");
var import_animated = require("@react-spring/animated");

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
__reExport(src_exports, require("@react-spring/core"), module.exports);
import_shared.Globals.assign({
  createStringInterpolator: import_shared.createStringInterpolator,
  colors: import_shared.colors
});
var host = (0, import_animated.createHost)(primitives, {
  applyAnimatedValues(instance, props) {
    if (!instance.nodeType) return false;
    instance._applyProps(instance, props);
  }
});
var animated = host.animated;
// Annotate the CommonJS export names for ESM import in node:
0 && (module.exports = {
  a,
  animated,
  ...require("@react-spring/core")
});
