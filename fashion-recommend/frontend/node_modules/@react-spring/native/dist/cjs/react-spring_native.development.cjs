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
var import_react_native2 = require("react-native");
var import_animated3 = require("@react-spring/animated");
var import_shared2 = require("@react-spring/shared");

// src/primitives.ts
var import_react_native = require("react-native");
var primitives = {
  View: import_react_native.View,
  Text: import_react_native.Text,
  Image: import_react_native.Image
};

// src/AnimatedStyle.ts
var import_animated2 = require("@react-spring/animated");

// src/AnimatedTransform.ts
var import_shared = require("@react-spring/shared");
var import_animated = require("@react-spring/animated");
var AnimatedTransform = class extends import_animated.AnimatedObject {
  constructor(source) {
    super(source);
  }
  getValue() {
    return this.source ? this.source.map((source) => {
      const transform = {};
      (0, import_shared.eachProp)(source, (source2, key) => {
        transform[key] = (0, import_shared.getFluidValue)(source2);
      });
      return transform;
    }) : [];
  }
  setValue(source) {
    this.source = source;
    this.payload = this._makePayload(source);
  }
  _makePayload(source) {
    if (!source) return [];
    const payload = /* @__PURE__ */ new Set();
    (0, import_shared.each)(source, (transform) => (0, import_shared.eachProp)(transform, this._addToPayload, payload));
    return Array.from(payload);
  }
};

// src/AnimatedStyle.ts
var AnimatedStyle = class extends import_animated2.AnimatedObject {
  constructor(style) {
    super(style);
  }
  setValue(style) {
    super.setValue(
      style && style.transform ? { ...style, transform: new AnimatedTransform(style.transform) } : style
    );
  }
};

// src/index.ts
__reExport(src_exports, require("@react-spring/core"), module.exports);
import_shared2.Globals.assign({
  batchedUpdates: require("react-native").unstable_batchedUpdates,
  createStringInterpolator: import_shared2.createStringInterpolator,
  colors: import_shared2.colors
});
var host = (0, import_animated3.createHost)(primitives, {
  applyAnimatedValues(instance, props) {
    if (import_shared2.is.und(props.children) && instance.setNativeProps) {
      instance.setNativeProps(props);
      return true;
    }
    return false;
  },
  createAnimatedStyle(styles) {
    styles = import_react_native2.StyleSheet.flatten(styles);
    if (import_shared2.is.obj(styles.shadowOffset)) {
      styles.shadowOffset = new import_animated3.AnimatedObject(styles.shadowOffset);
    }
    return new AnimatedStyle(styles);
  }
});
var animated = host.animated;
// Annotate the CommonJS export names for ESM import in node:
0 && (module.exports = {
  a,
  animated,
  ...require("@react-spring/core")
});
