import { raf, Rafz } from '@react-spring/rafz';
export { Timeout, raf } from '@react-spring/rafz';
import { InterpolatorConfig, OneOrMore, InterpolatorArgs, InterpolatorFactory, EasingFunction, Any, Lookup, Arrify, AnyFn } from '@react-spring/types';
import { EffectCallback, useEffect } from 'react';

/**
 * MIT License
 * Copyright (c) Alec Larson
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
declare const $get: unique symbol;
declare const $observers: unique symbol;

/** Returns true if `arg` can be observed. */
declare const hasFluidValue: (arg: any) => arg is FluidValue;
/**
 * Get the current value.
 * If `arg` is not observable, `arg` is returned.
 */
declare const getFluidValue: GetFluidValue;
/** Get the current observer set. Never mutate it directly! */
declare const getFluidObservers: GetFluidObservers;
/** Send an event to an observer. */
declare function callFluidObserver<E extends FluidEvent>(observer: FluidObserver<E>, event: E): void;
/** Send an event to all observers. */
declare function callFluidObservers<E extends FluidEvent>(target: FluidValue<any, E>, event: E): void;
declare function callFluidObservers(target: object, event: FluidEvent): void;
type GetFluidValue = {
    <T, U = never>(target: T | FluidValue<U>): Exclude<T, FluidValue> | U;
};
type GetFluidObservers = {
    <E extends FluidEvent>(target: FluidValue<any, E>): ReadonlySet<FluidObserver<E>> | null;
    (target: object): ReadonlySet<FluidObserver> | null;
};
/** An event sent to `FluidObserver` objects. */
interface FluidEvent<T = any> {
    type: string;
    parent: FluidValue<T>;
}
/**
 * Extend this class for automatic TypeScript support when passing this
 * value to `fluids`-compatible libraries.
 */
declare abstract class FluidValue<T = any, E extends FluidEvent<T> = any> {
    private [$get];
    private [$observers]?;
    constructor(get?: () => T);
    /** Get the current value. */
    protected get?(): T;
    /** Called after an observer is added. */
    protected observerAdded?(count: number, observer: FluidObserver<E>): void;
    /** Called after an observer is removed. */
    protected observerRemoved?(count: number, observer: FluidObserver<E>): void;
}
/** An observer of `FluidValue` objects. */
type FluidObserver<E extends FluidEvent = any> = {
    eventObserved(event: E): void;
} | {
    (event: E): void;
};
/** Add the `FluidValue` type to every property. */
type FluidProps<T> = T extends object ? {
    [P in keyof T]: T[P] | FluidValue<Exclude<T[P], void>>;
} : unknown;
/** Remove the `FluidValue` type from every property. */
type StaticProps<T extends object> = {
    [P in keyof T]: T[P] extends FluidValue<infer U> ? U : T[P];
};
/** Define the getter called by `getFluidValue`. */
declare const setFluidGetter: (target: object, get: () => any) => any;
/** Observe a `fluids`-compatible object. */
declare function addFluidObserver<T, E extends FluidEvent>(target: FluidValue<T, E>, observer: FluidObserver<E>): typeof observer;
declare function addFluidObserver<E extends FluidEvent>(target: object, observer: FluidObserver<E>): typeof observer;
/** Stop observing a `fluids`-compatible object. */
declare function removeFluidObserver<E extends FluidEvent>(target: FluidValue<any, E>, observer: FluidObserver<E>): void;
declare function removeFluidObserver<E extends FluidEvent>(target: object, observer: FluidObserver<E>): void;

interface OpaqueAnimation {
    idle: boolean;
    priority: number;
    advance(dt: number): void;
}
/**
 * The frameloop executes its animations in order of lowest priority first.
 * Animations are retained until idle.
 */
declare const frameLoop: {
    readonly idle: boolean;
    /** Advance the given animation on every frame until idle. */
    start(animation: OpaqueAnimation): void;
    /** Advance all animations by the given time. */
    advance: typeof advance;
    /** Call this when an animation's priority changes. */
    sort(animation: OpaqueAnimation): void;
    /**
     * Clear all animations. For testing purposes.
     *
     * ☠️ Never call this from within the frameloop.
     */
    clear(): void;
};
declare function advance(dt: number): boolean;

declare let createStringInterpolator$1: (config: InterpolatorConfig<string>) => (input: number) => string;
declare let to: <Input, Output>(source: OneOrMore<FluidValue>, args: InterpolatorArgs<Input, Output>) => FluidValue<Output>;
declare let colors$1: {
    [key: string]: number;
} | null;
declare let skipAnimation: boolean;
declare let willAdvance: (animation: OpaqueAnimation) => void;
interface AnimatedGlobals {
    /** Returns a new `Interpolation` object */
    to?: typeof to;
    /** Used to measure frame length. Read more [here](https://developer.mozilla.org/en-US/docs/Web/API/Performance/now) */
    now?: typeof raf.now;
    /** Provide custom color names for interpolation */
    colors?: typeof colors$1;
    /** Make all animations instant and skip the frameloop entirely */
    skipAnimation?: typeof skipAnimation;
    /** Provide custom logic for string interpolation */
    createStringInterpolator?: typeof createStringInterpolator$1;
    /** Schedule a function to run on the next frame */
    requestAnimationFrame?: (cb: () => void) => void;
    /** Event props are called with `batchedUpdates` to reduce extraneous renders */
    batchedUpdates?: typeof raf.batchedUpdates;
    /** @internal Exposed for testing purposes */
    willAdvance?: typeof willAdvance;
    /** sets the global frameLoop setting for the global raf instance */
    frameLoop?: Rafz['frameLoop'];
}
declare const assign: (globals: AnimatedGlobals) => void;

type globals_AnimatedGlobals = AnimatedGlobals;
declare const globals_assign: typeof assign;
declare const globals_skipAnimation: typeof skipAnimation;
declare const globals_to: typeof to;
declare const globals_willAdvance: typeof willAdvance;
declare namespace globals {
  export { type globals_AnimatedGlobals as AnimatedGlobals, globals_assign as assign, colors$1 as colors, createStringInterpolator$1 as createStringInterpolator, globals_skipAnimation as skipAnimation, globals_to as to, globals_willAdvance as willAdvance };
}

declare const clamp: (min: number, max: number, v: number) => number;

type ColorName = keyof typeof colors;
declare const colors: {
    transparent: number;
    aliceblue: number;
    antiquewhite: number;
    aqua: number;
    aquamarine: number;
    azure: number;
    beige: number;
    bisque: number;
    black: number;
    blanchedalmond: number;
    blue: number;
    blueviolet: number;
    brown: number;
    burlywood: number;
    burntsienna: number;
    cadetblue: number;
    chartreuse: number;
    chocolate: number;
    coral: number;
    cornflowerblue: number;
    cornsilk: number;
    crimson: number;
    cyan: number;
    darkblue: number;
    darkcyan: number;
    darkgoldenrod: number;
    darkgray: number;
    darkgreen: number;
    darkgrey: number;
    darkkhaki: number;
    darkmagenta: number;
    darkolivegreen: number;
    darkorange: number;
    darkorchid: number;
    darkred: number;
    darksalmon: number;
    darkseagreen: number;
    darkslateblue: number;
    darkslategray: number;
    darkslategrey: number;
    darkturquoise: number;
    darkviolet: number;
    deeppink: number;
    deepskyblue: number;
    dimgray: number;
    dimgrey: number;
    dodgerblue: number;
    firebrick: number;
    floralwhite: number;
    forestgreen: number;
    fuchsia: number;
    gainsboro: number;
    ghostwhite: number;
    gold: number;
    goldenrod: number;
    gray: number;
    green: number;
    greenyellow: number;
    grey: number;
    honeydew: number;
    hotpink: number;
    indianred: number;
    indigo: number;
    ivory: number;
    khaki: number;
    lavender: number;
    lavenderblush: number;
    lawngreen: number;
    lemonchiffon: number;
    lightblue: number;
    lightcoral: number;
    lightcyan: number;
    lightgoldenrodyellow: number;
    lightgray: number;
    lightgreen: number;
    lightgrey: number;
    lightpink: number;
    lightsalmon: number;
    lightseagreen: number;
    lightskyblue: number;
    lightslategray: number;
    lightslategrey: number;
    lightsteelblue: number;
    lightyellow: number;
    lime: number;
    limegreen: number;
    linen: number;
    magenta: number;
    maroon: number;
    mediumaquamarine: number;
    mediumblue: number;
    mediumorchid: number;
    mediumpurple: number;
    mediumseagreen: number;
    mediumslateblue: number;
    mediumspringgreen: number;
    mediumturquoise: number;
    mediumvioletred: number;
    midnightblue: number;
    mintcream: number;
    mistyrose: number;
    moccasin: number;
    navajowhite: number;
    navy: number;
    oldlace: number;
    olive: number;
    olivedrab: number;
    orange: number;
    orangered: number;
    orchid: number;
    palegoldenrod: number;
    palegreen: number;
    paleturquoise: number;
    palevioletred: number;
    papayawhip: number;
    peachpuff: number;
    peru: number;
    pink: number;
    plum: number;
    powderblue: number;
    purple: number;
    rebeccapurple: number;
    red: number;
    rosybrown: number;
    royalblue: number;
    saddlebrown: number;
    salmon: number;
    sandybrown: number;
    seagreen: number;
    seashell: number;
    sienna: number;
    silver: number;
    skyblue: number;
    slateblue: number;
    slategray: number;
    slategrey: number;
    snow: number;
    springgreen: number;
    steelblue: number;
    tan: number;
    teal: number;
    thistle: number;
    tomato: number;
    turquoise: number;
    violet: number;
    wheat: number;
    white: number;
    whitesmoke: number;
    yellow: number;
    yellowgreen: number;
};

declare function colorToRgba(input: string): string;

declare const rgb: RegExp;
declare const rgba: RegExp;
declare const hsl: RegExp;
declare const hsla: RegExp;
declare const hex3: RegExp;
declare const hex4: RegExp;
declare const hex6: RegExp;
declare const hex8: RegExp;

declare const createInterpolator: InterpolatorFactory;

type Direction = 'start' | 'end';
type StepsEasing = (steps: number, direction?: Direction) => EasingFunction;
/**
 * With thanks to ai easings.net
 * https://github.com/ai/easings.net/blob/master/src/easings/easingsFunctions.ts
 */
interface EasingDictionary {
    linear: (t: number) => number;
    easeInQuad: (t: number) => number;
    easeOutQuad: (t: number) => number;
    easeInOutQuad: (t: number) => number;
    easeInCubic: (t: number) => number;
    easeOutCubic: (t: number) => number;
    easeInOutCubic: (t: number) => number;
    easeInQuart: (t: number) => number;
    easeOutQuart: (t: number) => number;
    easeInOutQuart: (t: number) => number;
    easeInQuint: (t: number) => number;
    easeOutQuint: (t: number) => number;
    easeInOutQuint: (t: number) => number;
    easeInSine: (t: number) => number;
    easeOutSine: (t: number) => number;
    easeInOutSine: (t: number) => number;
    easeInExpo: (t: number) => number;
    easeOutExpo: (t: number) => number;
    easeInOutExpo: (t: number) => number;
    easeInCirc: (t: number) => number;
    easeOutCirc: (t: number) => number;
    easeInOutCirc: (t: number) => number;
    easeInBack: (t: number) => number;
    easeOutBack: (t: number) => number;
    easeInOutBack: (t: number) => number;
    easeInElastic: (t: number) => number;
    easeOutElastic: (t: number) => number;
    easeInOutElastic: (t: number) => number;
    easeInBounce: (t: number) => number;
    easeOutBounce: (t: number) => number;
    easeInOutBounce: (t: number) => number;
    steps: StepsEasing;
}
declare const easings: EasingDictionary;

/**
 * Supports string shapes by extracting numbers so new values can be computed,
 * and recombines those values into new strings of the same shape.  Supports
 * things like:
 *
 *     "rgba(123, 42, 99, 0.36)"           // colors
 *     "-45deg"                            // values with units
 *     "0 2px 2px 0px rgba(0, 0, 0, 0.12)" // CSS box-shadows
 *     "rotate(0deg) translate(2px, 3px)"  // CSS transforms
 */
declare const createStringInterpolator: (config: InterpolatorConfig<string>) => (input: number) => string;

declare const prefix = "react-spring: ";
declare const once: <TFunc extends (...args: any) => any>(fn: TFunc) => (...args: any) => void;
declare function deprecateInterpolate(): void;
declare function deprecateDirectCall(): void;

declare function noop(): void;
declare const defineHidden: (obj: any, key: any, value: any) => any;
type IsType<U> = <T>(arg: T & any) => arg is Narrow<T, U>;
type Narrow<T, U> = [T] extends [Any] ? U : [T] extends [U] ? Extract<T, U> : U;
type PlainObject<T> = Exclude<T & Lookup, Function | readonly any[]>;
declare const is: {
    arr: IsType<readonly any[]>;
    obj: <T>(a: T & any) => a is PlainObject<T>;
    fun: IsType<Function>;
    str: (a: unknown) => a is string;
    num: (a: unknown) => a is number;
    und: (a: unknown) => a is undefined;
};
/** Compare animatable values */
declare function isEqual(a: any, b: any): boolean;
type EachFn<Value, Key, This> = (this: This, value: Value, key: Key) => void;
type Eachable<Value = any, Key = any, This = any> = {
    forEach(cb: EachFn<Value, Key, This>, ctx?: This): void;
};
/** Minifiable `.forEach` call */
declare const each: <Value, Key, This>(obj: Eachable<Value, Key, This>, fn: EachFn<Value, Key, This>) => void;
/** Iterate the properties of an object */
declare function eachProp<T extends object, This>(obj: T, fn: (this: This, value: T extends any[] ? T[number] : T[keyof T], key: string) => void, ctx?: This): void;
declare const toArray: <T>(a: T) => Arrify<Exclude<T, void>>;
/** Copy the `queue`, then iterate it after the `queue` is cleared */
declare function flush<P, T>(queue: Map<P, T>, iterator: (entry: [P, T]) => void): void;
declare function flush<T>(queue: Set<T>, iterator: (value: T) => void): void;
/** Call every function in the queue with the same arguments. */
declare const flushCalls: <T extends AnyFn>(queue: Set<T>, ...args: Parameters<T>) => void;
declare const isSSR: () => boolean;

declare function isAnimatedString(value: unknown): value is string;

/**
 * Whilst user's may not need the scrollLength, it's easier to return
 * the whole state we're storing and let them pick what they want.
 */
interface ScrollAxis {
    current: number;
    progress: number;
    scrollLength: number;
}
interface ScrollInfo {
    time: number;
    x: ScrollAxis;
    y: ScrollAxis;
}

type OnScrollCallback = (info: ScrollInfo) => void;
type OnScrollOptions = {
    /**
     * The root container to measure against
     */
    container?: HTMLElement;
};
declare const onScroll: (callback: OnScrollCallback, { container }?: OnScrollOptions) => () => void;

interface OnResizeOptions {
    container?: HTMLElement;
}
type OnResizeCallback = (rect: Pick<DOMRectReadOnly, 'width' | 'height'> & Partial<Omit<DOMRectReadOnly, 'width' | 'height'>>) => void;
declare const onResize: (callback: OnResizeCallback, { container }?: OnResizeOptions) => (() => void);

type Init<T> = () => T;
/**
 * Creates a constant value over the lifecycle of a component.
 */
declare function useConstant<T>(init: Init<T>): T;

/** Return a function that re-renders this component, if still mounted */
declare function useForceUpdate(): () => void;

declare function useMemoOne<T>(getResult: () => T, inputs?: any[]): T;

declare const useOnce: (effect: EffectCallback) => void;

/** Use a value from the previous render */
declare function usePrev<T>(value: T): T | undefined;

/**
 * Use this to read layout from the DOM and synchronously
 * re-render if the isSSR returns true. Updates scheduled
 * inside `useIsomorphicLayoutEffect` will be flushed
 * synchronously in the browser, before the browser has
 * a chance to paint.
 */
declare const useIsomorphicLayoutEffect: typeof useEffect;

/**
 * Returns `boolean` or `null`, used to automatically
 * set skipAnimations to the value of the user's
 * `prefers-reduced-motion` query.
 *
 * The return value, post-effect, is the value of their prefered setting
 */
declare const useReducedMotion: () => boolean | null;

export { type ColorName, type Direction, type FluidEvent, type FluidObserver, type FluidProps, FluidValue, globals as Globals, type OnResizeCallback, type OnResizeOptions, type OnScrollCallback, type OnScrollOptions, type OpaqueAnimation, type StaticProps, addFluidObserver, callFluidObserver, callFluidObservers, clamp, colorToRgba, colors, createInterpolator, createStringInterpolator, defineHidden, deprecateDirectCall, deprecateInterpolate, each, eachProp, easings, flush, flushCalls, frameLoop, getFluidObservers, getFluidValue, hasFluidValue, hex3, hex4, hex6, hex8, hsl, hsla, is, isAnimatedString, isEqual, isSSR, noop, onResize, onScroll, once, prefix, removeFluidObserver, rgb, rgba, setFluidGetter, toArray, useConstant, useForceUpdate, useIsomorphicLayoutEffect, useMemoOne, useOnce, usePrev, useReducedMotion };
