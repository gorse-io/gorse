"use strict";
var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
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
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// src/index.ts
var src_exports = {};
__export(src_exports, {
  a: () => animated,
  animated: () => animated
});
module.exports = __toCommonJS(src_exports);
var import_fiber2 = require("@react-three/fiber");
var import_core = require("@react-spring/core");
var import_shared = require("@react-spring/shared");
var import_animated = require("@react-spring/animated");

// src/primitives.ts
var THREE = __toESM(require("three"));
var import_fiber = require("@react-three/fiber");
var primitives = ["primitive"].concat(
  Object.keys(THREE).filter((key) => /^[A-Z]/.test(key)).map((key) => key[0].toLowerCase() + key.slice(1))
);

// src/index.ts
__reExport(src_exports, require("@react-spring/core"), module.exports);
import_core.Globals.assign({
  createStringInterpolator: import_shared.createStringInterpolator,
  colors: import_shared.colors,
  frameLoop: "demand"
});
(0, import_fiber2.addEffect)(() => {
  import_shared.raf.advance();
});
var host = (0, import_animated.createHost)(primitives, {
  applyAnimatedValues: import_fiber2.applyProps
});
var animated = host.animated;
// Annotate the CommonJS export names for ESM import in node:
0 && (module.exports = {
  a,
  animated,
  ...require("@react-spring/core")
});
