import { Lookup, ElementType } from '@react-spring/types';

/** An animated number or a native attribute value */
declare class AnimatedValue<T = any> extends Animated {
    protected _value: T;
    done: boolean;
    elapsedTime: number;
    lastPosition: number;
    lastVelocity?: number | null;
    v0?: number | null;
    durationProgress: number;
    constructor(_value: T);
    /** @internal */
    static create(value: any): AnimatedValue<any>;
    getPayload(): Payload;
    getValue(): T;
    setValue(value: T, step?: number): boolean;
    reset(): void;
}

declare const isAnimated: <T = any>(value: any) => value is Animated<T>;
/** Get the owner's `Animated` node. */
declare const getAnimated: <T = any>(owner: any) => Animated<T> | undefined;
/** Set the owner's `Animated` node. */
declare const setAnimated: (owner: any, node: Animated) => any;
/** Get every `AnimatedValue` in the owner's `Animated` node. */
declare const getPayload: (owner: any) => AnimatedValue[] | undefined;
declare abstract class Animated<T = any> {
    /** The cache of animated values */
    protected payload?: Payload;
    constructor();
    /** Get the current value. Pass `true` for only animated values. */
    abstract getValue(animated?: boolean): T;
    /** Set the current value. Returns `true` if the value changed. */
    abstract setValue(value: T): boolean | void;
    /** Reset any animation state. */
    abstract reset(goal?: T): void;
    /** Get every `AnimatedValue` used by this node. */
    getPayload(): Payload;
}
type Payload = readonly AnimatedValue[];

type Value$1 = string | number;
declare class AnimatedString extends AnimatedValue<Value$1> {
    protected _value: number;
    protected _string: string | null;
    protected _toString: (input: number) => string;
    constructor(value: string);
    /** @internal */
    static create(value: string): AnimatedString;
    getValue(): string;
    setValue(value: Value$1): boolean;
    reset(goal?: string): void;
}

/** An object containing `Animated` nodes */
declare class AnimatedObject extends Animated {
    protected source: Lookup;
    constructor(source: Lookup);
    getValue(animated?: boolean): Lookup<any>;
    /** Replace the raw object data */
    setValue(source: Lookup): void;
    reset(): void;
    /** Create a payload set. */
    protected _makePayload(source: Lookup): AnimatedValue<any>[] | undefined;
    /** Add to a payload set. */
    protected _addToPayload(this: Set<AnimatedValue>, source: any): void;
}

type Value = number | string;
type Source = AnimatedValue<Value>[];
/** An array of animated nodes */
declare class AnimatedArray<T extends ReadonlyArray<Value> = Value[]> extends AnimatedObject {
    protected source: Source;
    constructor(source: T);
    /** @internal */
    static create<T extends ReadonlyArray<Value>>(source: T): AnimatedArray<T>;
    getValue(): T;
    setValue(source: T): boolean;
}

type AnimatedType<T = any> = Function & {
    create: (from: any, goal?: any) => T extends ReadonlyArray<number | string> ? AnimatedArray<T> : AnimatedValue<T>;
};

/** Return the `Animated` node constructor for a given value */
declare function getAnimatedType(value: any): AnimatedType;

type AnimatableComponent = string | Exclude<ElementType, string>;

interface HostConfig {
    /** Provide custom logic for native updates */
    applyAnimatedValues: (node: any, props: Lookup) => boolean | void;
    /** Wrap the `style` prop with an animated node */
    createAnimatedStyle: (style: Lookup) => Animated;
    /** Intercept props before they're passed to an animated component */
    getComponentProps: (props: Lookup) => typeof props;
}
type WithAnimated = {
    (Component: AnimatableComponent): any;
    [key: string]: any;
};
declare const createHost: (components: AnimatableComponent[] | {
    [key: string]: AnimatableComponent;
}, { applyAnimatedValues, createAnimatedStyle, getComponentProps, }?: Partial<HostConfig>) => {
    animated: WithAnimated;
};

export { Animated, AnimatedArray, AnimatedObject, AnimatedString, type AnimatedType, AnimatedValue, type HostConfig, type Payload, createHost, getAnimated, getAnimatedType, getPayload, isAnimated, setAnimated };
