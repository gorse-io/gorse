import { InterpolatorFn, InterpolatorArgs, EasingFunction, Lookup, Any, UnknownProps, Falsy, OneOrMore, Remap, ObjectFromUnion, Constrain, ObjectType, Merge, NoInfer, InterpolatorConfig, Animatable, ExtrapolateType } from '@react-spring/types';
export * from '@react-spring/types';
import { FluidValue, Timeout, FluidProps } from '@react-spring/shared';
export { Globals, createInterpolator, easings, useIsomorphicLayoutEffect, useReducedMotion } from '@react-spring/shared';
import { AnimatedValue, Animated } from '@react-spring/animated';
import * as react from 'react';
import { ReactNode, JSX, DependencyList, MutableRefObject, RefObject } from 'react';

/**
 * An `Interpolation` is a memoized value that's computed whenever one of its
 * `FluidValue` dependencies has its value changed.
 *
 * Other `FrameValue` objects can depend on this. For example, passing an
 * `Interpolation` as the `to` prop of a `useSpring` call will trigger an
 * animation toward the memoized value.
 */
declare class Interpolation<Input = any, Output = any> extends FrameValue<Output> {
    /** The source of input values */
    readonly source: unknown;
    /** Useful for debugging. */
    key?: string;
    /** Equals false when in the frameloop */
    idle: boolean;
    /** The function that maps inputs values to output */
    readonly calc: InterpolatorFn<Input, Output>;
    /** The inputs which are currently animating */
    protected _active: Set<FluidValue<any, any>>;
    constructor(
    /** The source of input values */
    source: unknown, args: InterpolatorArgs<Input, Output>);
    advance(_dt?: number): void;
    protected _get(): Output;
    protected _start(): void;
    protected _attach(): void;
    protected _detach(): void;
    /** @internal */
    eventObserved(event: FrameValue.Event): void;
}

/**
 * A kind of `FluidValue` that manages an `AnimatedValue` node.
 *
 * Its underlying value can be accessed and even observed.
 */
declare abstract class FrameValue<T = any> extends FluidValue<T, FrameValue.Event<T>> {
    readonly id: number;
    abstract key?: string;
    abstract get idle(): boolean;
    protected _priority: number;
    get priority(): number;
    set priority(priority: number);
    /** Get the current value */
    get(): T;
    /** Create a spring that maps our value to another value */
    to<Out>(...args: InterpolatorArgs<T, Out>): Interpolation<T, Out>;
    /** @deprecated Use the `to` method instead. */
    interpolate<Out>(...args: InterpolatorArgs<T, Out>): Interpolation<T, Out>;
    toJSON(): T;
    protected observerAdded(count: number): void;
    protected observerRemoved(count: number): void;
    /** @internal */
    abstract advance(dt: number): void;
    /** @internal */
    abstract eventObserved(_event: FrameValue.Event): void;
    /** Called when the first child is added. */
    protected _attach(): void;
    /** Called when the last child is removed. */
    protected _detach(): void;
    /** Tell our children about our new value */
    protected _onChange(value: T, idle?: boolean): void;
    /** Tell our children about our new priority */
    protected _onPriorityChange(priority: number): void;
}
declare namespace FrameValue {
    /** A parent changed its value */
    interface ChangeEvent<T = any> {
        parent: FrameValue<T>;
        type: 'change';
        value: T;
        idle: boolean;
    }
    /** A parent changed its priority */
    interface PriorityEvent<T = any> {
        parent: FrameValue<T>;
        type: 'priority';
        priority: number;
    }
    /** A parent is done animating */
    interface IdleEvent<T = any> {
        parent: FrameValue<T>;
        type: 'idle';
    }
    /** Events sent to children of `FrameValue` objects */
    type Event<T = any> = ChangeEvent<T> | PriorityEvent<T> | IdleEvent<T>;
}

declare class AnimationConfig {
    /**
     * With higher tension, the spring will resist bouncing and try harder to stop at its end value.
     *
     * When tension is zero, no animation occurs.
     *
     * @default 170
     */
    tension: number;
    /**
     * The damping ratio coefficient, or just the damping ratio when `speed` is defined.
     *
     * When `speed` is defined, this value should be between 0 and 1.
     *
     * Higher friction means the spring will slow down faster.
     *
     * @default 26
     */
    friction: number;
    /**
     * The natural frequency (in seconds), which dictates the number of bounces
     * per second when no damping exists.
     *
     * When defined, `tension` is derived from this, and `friction` is derived
     * from `tension` and `damping`.
     */
    frequency?: number;
    /**
     * The damping ratio, which dictates how the spring slows down.
     *
     * Set to `0` to never slow down. Set to `1` to slow down without bouncing.
     * Between `0` and `1` is for you to explore.
     *
     * Only works when `frequency` is defined.
     *
     * @default 1
     */
    damping: number;
    /**
     * Higher mass means more friction is required to slow down.
     *
     * Defaults to 1, which works fine most of the time.
     *
     * @default 1
     */
    mass: number;
    /**
     * The initial velocity of one or more values.
     *
     * @default 0
     */
    velocity: number | number[];
    /**
     * The smallest velocity before the animation is considered "not moving".
     *
     * When undefined, `precision` is used instead.
     */
    restVelocity?: number;
    /**
     * The smallest distance from a value before that distance is essentially zero.
     *
     * This helps in deciding when a spring is "at rest". The spring must be within
     * this distance from its final value, and its velocity must be lower than this
     * value too (unless `restVelocity` is defined).
     *
     * @default 0.01
     */
    precision?: number;
    /**
     * For `duration` animations only. Note: The `duration` is not affected
     * by this property.
     *
     * Defaults to `0`, which means "start from the beginning".
     *
     * Setting to `1+` makes an immediate animation.
     *
     * Setting to `0.5` means "start from the middle of the easing function".
     *
     * Any number `>= 0` and `<= 1` makes sense here.
     */
    progress?: number;
    /**
     * Animation length in number of milliseconds.
     */
    duration?: number;
    /**
     * The animation curve. Only used when `duration` is defined.
     *
     * Defaults to quadratic ease-in-out.
     */
    easing: EasingFunction;
    /**
     * Avoid overshooting by ending abruptly at the goal value.
     *
     * @default false
     */
    clamp: boolean;
    /**
     * When above zero, the spring will bounce instead of overshooting when
     * exceeding its goal value. Its velocity is multiplied by `-1 + bounce`
     * whenever its current value equals or exceeds its goal. For example,
     * setting `bounce` to `0.5` chops the velocity in half on each bounce,
     * in addition to any friction.
     */
    bounce?: number;
    /**
     * "Decay animations" decelerate without an explicit goal value.
     * Useful for scrolling animations.
     *
     * Use `true` for the default exponential decay factor (`0.998`).
     *
     * When a `number` between `0` and `1` is given, a lower number makes the
     * animation slow down faster. And setting to `1` would make an unending
     * animation.
     *
     * @default false
     */
    decay?: boolean | number;
    /**
     * While animating, round to the nearest multiple of this number.
     * The `from` and `to` values are never rounded, as well as any value
     * passed to the `set` method of an animated value.
     */
    round?: number;
    constructor();
}

/** The object type of the `config` prop. */
type SpringConfig = Partial<AnimationConfig>;
/** The object given to the `onRest` prop and `start` promise. */
interface AnimationResult<T extends Readable = any> {
    value: T extends Readable<infer U> ? U : never;
    /** When true, no animation ever started. */
    noop?: boolean;
    /** When true, the animation was neither cancelled nor stopped prematurely. */
    finished?: boolean;
    /** When true, the animation was cancelled before it could finish. */
    cancelled?: boolean;
}
/** The promised result of an animation. */
type AsyncResult<T extends Readable = any> = Promise<AnimationResult<T>>;
/** Map an object type to allow `SpringValue` for any property */
type Springify<T> = Lookup<SpringValue<unknown> | undefined> & {
    [P in keyof T]: T[P] | SpringValue<T[P]>;
};
/**
 * The set of `SpringValue` objects returned by a `useSpring` call (or similar).
 */
type SpringValues<T extends Lookup = any> = [T] extends [Any] ? Lookup<SpringValue<unknown> | undefined> : {
    [P in keyof T]: SpringWrap<T[P]>;
};
type SpringWrap<T> = [
    Exclude<T, FluidValue>,
    Extract<T, readonly any[]>
] extends [object | void, never] ? never : SpringValue<Exclude<T, FluidValue | void>> | Extract<T, void>;

/** @internal */
interface Readable<T = any> {
    get(): T;
}
/** @internal */
type InferState<T extends Readable> = T extends Controller<infer State> ? State : T extends SpringValue<infer U> ? U : unknown;
/** @internal */
type InferProps<T extends Readable> = T extends Controller<infer State> ? ControllerUpdate<State> : T extends SpringValue<infer U> ? SpringUpdate<U> : Lookup;
/** @internal */
type InferTarget<T> = T extends object ? T extends ReadonlyArray<number | string> ? SpringValue<T> : Controller<T> : SpringValue<T>;
/** @internal */
interface AnimationTarget<T = any> extends Readable<T> {
    start(props: any): AsyncResult<this>;
    stop: Function;
    item?: unknown;
}
/** @internal */
interface AnimationRange<T> {
    to: T | FluidValue<T> | undefined;
    from: T | FluidValue<T> | undefined;
}
/** @internal */
type AnimationResolver<T extends Readable> = (result: AnimationResult<T> | AsyncResult<T>) => void;
/** @internal */
type EventKey = Exclude<keyof ReservedEventProps, 'onResolve' | 'onDestroyed'>;
/** @internal */
type PickEventFns<T> = {
    [P in Extract<keyof T, EventKey>]?: Extract<T[P], Function>;
};

/** An animation being executed by the frameloop */
declare class Animation<T = any> {
    changed: boolean;
    values: readonly AnimatedValue[];
    toValues: readonly number[] | null;
    fromValues: readonly number[];
    to: T | FluidValue<T>;
    from: T | FluidValue<T>;
    config: AnimationConfig;
    immediate: boolean;
}
interface Animation<T> extends PickEventFns<SpringProps<T>> {
}

type AsyncTo<T> = SpringChain<T> | SpringToFn<T>;
/** @internal */
type RunAsyncProps<T extends AnimationTarget = any> = InferProps<T> & {
    callId: number;
    parentId?: number;
    cancel: boolean;
    to?: any;
};
/** @internal */
interface RunAsyncState<T extends AnimationTarget = any> {
    paused: boolean;
    pauseQueue: Set<() => void>;
    resumeQueue: Set<() => void>;
    timeouts: Set<Timeout>;
    delayed?: boolean;
    asyncId?: number;
    asyncTo?: AsyncTo<InferState<T>>;
    promise?: AsyncResult<T>;
    cancelId?: number;
}
/** This error is thrown to signal an interrupted async animation. */
declare class BailSignal extends Error {
    result: AnimationResult;
    constructor();
}

interface DefaultSpringProps<T> extends Pick<SpringProps<T>, 'pause' | 'cancel' | 'immediate' | 'config'>, PickEventFns<SpringProps<T>> {
}
/**
 * Only numbers, strings, and arrays of numbers/strings are supported.
 * Non-animatable strings are also supported.
 */
declare class SpringValue<T = any> extends FrameValue<T> {
    /** The property name used when `to` or `from` is an object. Useful when debugging too. */
    key?: string;
    /** The animation state */
    animation: Animation<T>;
    /** The queue of pending props */
    queue?: SpringUpdate<T>[];
    /** Some props have customizable default values */
    defaultProps: DefaultSpringProps<T>;
    /** The state for `runAsync` calls */
    protected _state: RunAsyncState<SpringValue<T>>;
    /** The promise resolvers of pending `start` calls */
    protected _pendingCalls: Set<AnimationResolver<this>>;
    /** The counter for tracking `scheduleProps` calls */
    protected _lastCallId: number;
    /** The last `scheduleProps` call that changed the `to` prop */
    protected _lastToId: number;
    protected _memoizedDuration: number;
    constructor(from: Exclude<T, object>, props?: SpringUpdate<T>);
    constructor(props?: SpringUpdate<T>);
    /** Equals true when not advancing on each frame. */
    get idle(): boolean;
    get goal(): T;
    get velocity(): VelocityProp<T>;
    /**
     * When true, this value has been animated at least once.
     */
    get hasAnimated(): boolean;
    /**
     * When true, this value has an unfinished animation,
     * which is either active or paused.
     */
    get isAnimating(): boolean;
    /**
     * When true, all current and future animations are paused.
     */
    get isPaused(): boolean;
    /**
     *
     *
     */
    get isDelayed(): boolean | undefined;
    /** Advance the current animation by a number of milliseconds */
    advance(dt: number): void;
    /** Set the current value, while stopping the current animation */
    set(value: T | FluidValue<T>): this;
    /**
     * Freeze the active animation in time, as well as any updates merged
     * before `resume` is called.
     */
    pause(): void;
    /** Resume the animation if paused. */
    resume(): void;
    /** Skip to the end of the current animation. */
    finish(): this;
    /** Push props into the pending queue. */
    update(props: SpringUpdate<T>): this;
    /**
     * Update this value's animation using the queue of pending props,
     * and unpause the current animation (if one is frozen).
     *
     * When arguments are passed, a new animation is created, and the
     * queued animations are left alone.
     */
    start(): AsyncResult<this>;
    start(props: SpringUpdate<T>): AsyncResult<this>;
    start(to: T, props?: SpringProps<T>): AsyncResult<this>;
    /**
     * Stop the current animation, and cancel any delayed updates.
     *
     * Pass `true` to call `onRest` with `cancelled: true`.
     */
    stop(cancel?: boolean): this;
    /** Restart the animation. */
    reset(): void;
    /** @internal */
    eventObserved(event: FrameValue.Event): void;
    /**
     * Parse the `to` and `from` range from the given `props` object.
     *
     * This also ensures the initial value is available to animated components
     * during the render phase.
     */
    protected _prepareNode(props: {
        to?: any;
        from?: any;
        reverse?: boolean;
        default?: any;
    }): {
        to: any;
        from: any;
    };
    /** Every update is processed by this method before merging. */
    protected _update({ ...props }: SpringProps<T>, isLoop?: boolean): AsyncResult<SpringValue<T>>;
    /** Merge props into the current animation */
    protected _merge(range: AnimationRange<T>, props: RunAsyncProps<SpringValue<T>>, resolve: AnimationResolver<SpringValue<T>>): void;
    /** Update the `animation.to` value, which might be a `FluidValue` */
    protected _focus(value: T | FluidValue<T>): void;
    protected _attach(): void;
    protected _detach(): void;
    /**
     * Update the current value from outside the frameloop,
     * and return the `Animated` node.
     */
    protected _set(arg: T | FluidValue<T>, idle?: boolean): Animated | undefined;
    protected _onStart(): void;
    protected _onChange(value: T, idle?: boolean): void;
    protected _start(): void;
    protected _resume(): void;
    /**
     * Exit the frameloop and notify `onRest` listeners.
     *
     * Always wrap `_stop` calls with `batchedUpdates`.
     */
    protected _stop(goal?: any, cancel?: boolean): void;
}

/** Queue of pending updates for a `Controller` instance. */
interface ControllerQueue<State extends Lookup = Lookup> extends Array<ControllerUpdate<State, any> & {
    /** The keys affected by this update. When null, all keys are affected. */
    keys: string[] | null;
}> {
}
declare class Controller<State extends Lookup = Lookup> {
    readonly id: number;
    /** The animated values */
    springs: SpringValues<State>;
    /** The queue of props passed to the `update` method. */
    queue: ControllerQueue<State>;
    /**
     * The injected ref. When defined, render-based updates are pushed
     * onto the `queue` instead of being auto-started.
     */
    ref?: SpringRef<State>;
    /** Custom handler for flushing update queues */
    protected _flush?: ControllerFlushFn<this>;
    /** These props are used by all future spring values */
    protected _initialProps?: Lookup;
    /** The counter for tracking `scheduleProps` calls */
    protected _lastAsyncId: number;
    /** The values currently being animated */
    protected _active: Set<FrameValue<any>>;
    /** The values that changed recently */
    protected _changed: Set<FrameValue<any>>;
    /** Equals false when `onStart` listeners can be called */
    protected _started: boolean;
    private _item?;
    /** State used by the `runAsync` function */
    protected _state: RunAsyncState<this>;
    /** The event queues that are flushed once per frame maximum */
    protected _events: {
        onStart: Map<OnStart<SpringValue<State>, Controller<State>, any>, AnimationResult<any>>;
        onChange: Map<OnChange<SpringValue<State>, Controller<State>, any>, AnimationResult<any>>;
        onRest: Map<OnRest<SpringValue<State>, Controller<State>, any>, AnimationResult<any>>;
    };
    constructor(props?: ControllerUpdate<State> | null, flush?: ControllerFlushFn<any>);
    /**
     * Equals `true` when no spring values are in the frameloop, and
     * no async animation is currently active.
     */
    get idle(): boolean;
    get item(): any;
    set item(item: any);
    /** Get the current values of our springs */
    get(): State & UnknownProps;
    /** Set the current values without animating. */
    set(values: Partial<State>): void;
    /** Push an update onto the queue of each value. */
    update(props: ControllerUpdate<State> | Falsy): this;
    /**
     * Start the queued animations for every spring, and resolve the returned
     * promise once all queued animations have finished or been cancelled.
     *
     * When you pass a queue (instead of nothing), that queue is used instead of
     * the queued animations added with the `update` method, which are left alone.
     */
    start(props?: OneOrMore<ControllerUpdate<State>> | null): AsyncResult<this>;
    /** Stop all animations. */
    stop(): this;
    /** Stop animations for the given keys. */
    stop(keys: OneOrMore<string>): this;
    /** Cancel all animations. */
    stop(cancel: boolean): this;
    /** Cancel animations for the given keys. */
    stop(cancel: boolean, keys: OneOrMore<string>): this;
    /** Stop some or all animations. */
    stop(keys?: OneOrMore<string>): this;
    /** Cancel some or all animations. */
    stop(cancel: boolean, keys?: OneOrMore<string>): this;
    /** Freeze the active animation in time */
    pause(keys?: OneOrMore<string>): this;
    /** Resume the animation if paused. */
    resume(keys?: OneOrMore<string>): this;
    /** Call a function once per spring value */
    each(iterator: (spring: SpringValue, key: string) => void): void;
    /** @internal Called at the end of every animation frame */
    protected _onFrame(): void;
    /** @internal */
    eventObserved(event: FrameValue.Event): void;
}

/** Replace the type of each `T` property with `never` (unless compatible with `U`) */
type Valid<T, U> = NeverProps<T, InvalidKeys<T, U>>;
/** Replace the type of each `P` property with `never` */
type NeverProps<T, P extends keyof T> = Remap<Pick<T, Exclude<keyof T, P>> & {
    [K in P]: never;
}>;
/** Return a union type of every key whose `T` value is incompatible with its `U` value */
type InvalidKeys<T, U> = {
    [P in keyof T & keyof U]: T[P] extends U[P] ? never : P;
}[keyof T & keyof U];
/** Unwrap any `FluidValue` object types */
type RawValues<T extends object> = {
    [P in keyof T]: T[P] extends FluidValue<infer U> ? U : T[P];
};
/**
 * For testing whether a type is an object but not an array.
 *
 *     T extends IsPlainObject<T> ? true : false
 *
 * When `any` is passed, the resolved type is `true | false`.
 */
type IsPlainObject<T> = T extends ReadonlyArray<any> ? Any : T extends object ? object : Any;
type StringKeys<T> = T extends IsPlainObject<T> ? string & keyof T : string;

/** The flush function that handles `start` calls */
type ControllerFlushFn<T extends Controller<any> = Controller> = (ctrl: T, queue: ControllerQueue<InferState<T>>) => AsyncResult<T>;
/**
 * An async function that can update or stop the animations of a spring.
 * Typically defined as the `to` prop.
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
interface SpringToFn<T = any> {
    (start: StartFn<T>, stop: StopFn<T>): Promise<any> | void;
}
type StartFn<T> = InferTarget<T> extends {
    start: infer T;
} ? T : never;
type StopFn<T> = InferTarget<T> extends {
    stop: infer T;
} ? T : never;
/**
 * Update the props of an animation.
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
type SpringUpdateFn<T = any> = T extends IsPlainObject<T> ? UpdateValuesFn<T> : UpdateValueFn<T>;
interface AnyUpdateFn<T extends SpringValue | Controller<any>, Props extends object = InferProps<T>, State = InferState<T>> {
    (to: SpringTo<State>, props?: Props): AsyncResult<T>;
    (props: {
        to?: SpringToFn<T> | Falsy;
    } & Props): AsyncResult<T>;
    (props: {
        to?: SpringChain<State> | Falsy;
    } & Props): AsyncResult<T>;
}
/**
 * Update the props of a `Controller` object or `useSpring` call.
 *
 * The `T` parameter must be a set of animated values (as an object type).
 */
interface UpdateValuesFn<State extends Lookup = Lookup> extends AnyUpdateFn<Controller<State>> {
    (props: InlineToProps<State> & ControllerProps<State>): AsyncResult<Controller<State>>;
    (props: {
        to?: GoalValues<State> | Falsy;
    } & ControllerProps<State>): AsyncResult<Controller<State>>;
}
/**
 * Update the props of a spring.
 *
 * The `T` parameter must be a primitive type for a single animated value.
 */
interface UpdateValueFn<T = any> extends AnyUpdateFn<SpringValue<T>> {
    (props: {
        to?: GoalValue<T>;
    } & SpringProps<T>): AsyncResult<SpringValue<T>>;
}
type EventHandler<TResult extends Readable = any, TSource = unknown, Item = undefined> = (result: AnimationResult<TResult>, ctrl: TSource, item?: Item) => void;
/**
 * Called before the first frame of every animation.
 * From inside the `requestAnimationFrame` callback.
 */
type OnStart<TResult extends Readable, TSource, Item = undefined> = EventHandler<TResult, TSource, Item>;
/** Called when a `SpringValue` changes */
type OnChange<TResult extends Readable, TSource, Item = undefined> = EventHandler<TResult, TSource, Item>;
type OnPause<TResult extends Readable, TSource, Item = undefined> = EventHandler<TResult, TSource, Item>;
type OnResume<TResult extends Readable, TSource, Item = undefined> = EventHandler<TResult, TSource, Item>;
/** Called once the animation comes to a halt */
type OnRest<TResult extends Readable, TSource, Item = undefined> = EventHandler<TResult, TSource, Item>;
type OnResolve<TResult extends Readable, TSource, Item = undefined> = EventHandler<TResult, TSource, Item>;
/**
 * Called after an animation is updated by new props,
 * even if the animation remains idle.
 */
type OnProps<T = unknown> = (props: Readonly<SpringProps<T>>, spring: SpringValue<T>) => void;

declare enum TransitionPhase {
    /** This transition is being mounted */
    MOUNT = "mount",
    /** This transition is entering or has entered */
    ENTER = "enter",
    /** This transition had its animations updated */
    UPDATE = "update",
    /** This transition will expire after animating */
    LEAVE = "leave"
}

/** The phases of a `useTransition` item */
type TransitionKey = 'initial' | 'enter' | 'update' | 'leave';
/**
 * Extract a union of animated values from a set of `useTransition` props.
 */
type TransitionValues<Props extends object> = unknown & ForwardProps<ObjectFromUnion<Constrain<ObjectType<Props[TransitionKey & keyof Props] extends infer T ? T extends ReadonlyArray<infer Element> ? Element : T extends (...args: any[]) => infer Return ? Return extends ReadonlyArray<infer ReturnElement> ? ReturnElement : Return : T : never>, {}>>>;
type UseTransitionProps<Item = any> = Merge<Omit<ControllerProps<UnknownProps, Item>, 'onResolve'>, {
    from?: TransitionFrom<Item>;
    initial?: TransitionFrom<Item>;
    enter?: TransitionTo<Item>;
    update?: TransitionTo<Item>;
    leave?: TransitionTo<Item>;
    keys?: ItemKeys<Item>;
    sort?: (a: Item, b: Item) => number;
    trail?: number;
    exitBeforeEnter?: boolean;
    /**
     * When `true` or `<= 0`, each item is unmounted immediately after its
     * `leave` animation is finished.
     *
     * When `false`, items are never unmounted.
     *
     * When `> 0`, this prop is used in a `setTimeout` call that forces a
     * rerender if the component that called `useTransition` doesn't rerender
     * on its own after an item's `leave` animation is finished.
     */
    expires?: boolean | number | ((item: Item) => boolean | number);
    config?: SpringConfig | ((item: Item, index: number, state: TransitionPhase) => AnimationProps['config']);
    /**
     * Called after a transition item is unmounted.
     */
    onDestroyed?: (item: Item, key: Key) => void;
    /**
     * Used to access the imperative API.
     *
     * Animations never auto-start when `ref` is defined.
     */
    ref?: SpringRef;
}>;
type TransitionComponentProps<Item, Props extends object = any> = unknown & UseTransitionProps<Item> & {
    keys?: ItemKeys<NoInfer<Item>>;
    items: OneOrMore<Item>;
    children: TransitionRenderFn<NoInfer<Item>, PickAnimated<Props>>;
};
type Key = string | number;
type ItemKeys<T = any> = OneOrMore<Key> | ((item: T) => Key) | null;
/** The function returned by `useTransition` */
interface TransitionFn<Item = any, State extends Lookup = Lookup> {
    (render: TransitionRenderFn<Item, State>): JSX.Element;
}
interface TransitionRenderFn<Item = any, State extends Lookup = Lookup> {
    (values: SpringValues<State>, item: Item, transition: TransitionState<Item, State>, index: number): ReactNode;
}
interface TransitionState<Item = any, State extends Lookup = Lookup> {
    key: any;
    item: Item;
    ctrl: Controller<State>;
    phase: TransitionPhase;
    expired?: boolean;
    expirationId?: number;
}
type TransitionFrom<Item> = Falsy | GoalProp<UnknownProps> | ((item: Item, index: number) => GoalProp<UnknownProps> | Falsy);
type TransitionTo<Item, State extends Lookup = Lookup> = Falsy | OneOrMore<ControllerUpdate<State, Item>> | Function | ((item: Item, index: number) => ControllerUpdate<State, Item> | SpringChain<State> | SpringToFn<State> | Falsy);
interface Change {
    phase: TransitionPhase;
    springs: SpringValues<UnknownProps>;
    payload: ControllerUpdate;
}

/**
 * Move all non-reserved props into the `to` prop.
 */
type InferTo<T extends object> = Merge<{
    to: ForwardProps<T>;
}, Pick<T, keyof T & keyof ReservedProps>>;
/**
 * The props of a `useSpring` call or its async `update` function.
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
type SpringUpdate<T = any> = ToProps<T> & SpringProps<T>;
type SpringsUpdate<State extends Lookup = UnknownProps> = OneOrMore<ControllerUpdate<State>> | ((index: number, ctrl: Controller<State>) => ControllerUpdate<State> | null);
/**
 * Use the `SpringUpdate` type if you need the `to` prop to exist.
 * For function types, prefer one overload per possible `to` prop
 * type (for better type inference).
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
interface SpringProps<T = any> extends AnimationProps<T> {
    from?: GoalValue<T>;
    loop?: LoopProp<SpringUpdate>;
    /**
     * Called after an animation is updated by new props,
     * even if the animation remains idle.
     */
    onProps?: EventProp<OnProps<T>>;
    /**
     * Called when an animation moves for the first time.
     */
    onStart?: EventProp<OnStart<SpringValue<T>, SpringValue<T>>>;
    /**
     * Called when a spring has its value changed.
     */
    onChange?: EventProp<OnChange<SpringValue<T>, SpringValue<T>>>;
    onPause?: EventProp<OnPause<SpringValue<T>, SpringValue<T>>>;
    onResume?: EventProp<OnResume<SpringValue<T>, SpringValue<T>>>;
    /**
     * Called when all animations come to a stand-still.
     */
    onRest?: EventProp<OnRest<SpringValue<T>, SpringValue<T>>>;
}
/**
 * A union type of all possible `to` prop values.
 *
 * This is not recommended for function types. Instead, you should declare
 * an overload for each `to` type. See `SpringUpdateFn` for an example.
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
type ToProps<T = any> = {
    to?: GoalProp<T> | SpringToFn<T> | SpringChain<T>;
} | ([T] extends [IsPlainObject<T>] ? InlineToProps<T> : never);
/**
 * A value or set of values that can be animated from/to.
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
type GoalProp<T> = [T] extends [IsPlainObject<T>] ? GoalValues<T> | Falsy : GoalValue<T>;
/** A set of values for a `Controller` to animate from/to. */
type GoalValues<T> = FluidProps<T> extends infer Props ? {
    [P in keyof Props]?: Props[P] | null;
} : never;
/**
 * A value that `SpringValue` objects can animate from/to.
 *
 * The `UnknownProps` type lets you pass in { a: 1 } if the `key`
 * property of `SpringValue` equals "a".
 */
type GoalValue<T> = T | FluidValue<T> | UnknownProps | null | undefined;
/**
 * Where `to` is inferred from non-reserved props
 *
 * The `T` parameter can be a set of animated values (as an object type)
 * or a primitive type for a single animated value.
 */
type InlineToProps<T = any> = Remap<GoalValues<T> & {
    to?: undefined;
}>;
/** A serial queue of spring updates. */
interface SpringChain<T = any> extends Array<[
    T
] extends [IsPlainObject<T>] ? ControllerUpdate<T> : SpringTo<T> | SpringUpdate<T>> {
}
/** A value that any `SpringValue` or `Controller` can animate to. */
type SpringTo<T = any> = ([T] extends [IsPlainObject<T>] ? never : T | FluidValue<T>) | SpringChain<T> | SpringToFn<T> | Falsy;
type ControllerUpdate<State extends Lookup = Lookup, Item = undefined> = unknown & ToProps<State> & ControllerProps<State, Item>;
/**
 * Props for `Controller` methods and constructor.
 */
interface ControllerProps<State extends Lookup = Lookup, Item = undefined> extends AnimationProps<State> {
    ref?: SpringRef<State>;
    from?: GoalValues<State> | Falsy;
    loop?: LoopProp<ControllerUpdate>;
    /**
     * Called when the # of animating values exceeds 0
     *
     * Also accepts an object for per-key events
     */
    onStart?: OnStart<SpringValue<State>, Controller<State>, Item> | {
        [P in keyof State]?: OnStart<SpringValue<State[P]>, Controller<State>, Item>;
    };
    /**
     * Called when the # of animating values hits 0
     *
     * Also accepts an object for per-key events
     */
    onRest?: OnRest<SpringValue<State>, Controller<State>, Item> | {
        [P in keyof State]?: OnRest<SpringValue<State[P]>, Controller<State>, Item>;
    };
    /**
     * Called once per frame when animations are active
     *
     * Also accepts an object for per-key events
     */
    onChange?: OnChange<SpringValue<State>, Controller<State>, Item> | {
        [P in keyof State]?: OnChange<SpringValue<State[P]>, Controller<State>, Item>;
    };
    onPause?: OnPause<SpringValue<State>, Controller<State>, Item> | {
        [P in keyof State]?: OnPause<SpringValue<State[P]>, Controller<State>, Item>;
    };
    onResume?: OnResume<SpringValue<State>, Controller<State>, Item> | {
        [P in keyof State]?: OnResume<SpringValue<State[P]>, Controller<State>, Item>;
    };
    /**
     * Called after an animation is updated by new props.
     * Useful for manipulation
     *
     * Also accepts an object for per-key events
     */
    onProps?: OnProps<State> | {
        [P in keyof State]?: OnProps<State[P]>;
    };
    /**
     * Called when the promise for this update is resolved.
     */
    onResolve?: OnResolve<SpringValue<State>, Controller<State>, Item>;
}
type LoopProp<T extends object> = boolean | T | (() => boolean | T);
type VelocityProp<T = any> = T extends ReadonlyArray<number | string> ? number[] : number;
/** For props that can be set on a per-key basis. */
type MatchProp<T> = boolean | OneOrMore<StringKeys<T>> | ((key: StringKeys<T>) => boolean);
/** Event props can be customized per-key. */
type EventProp<T> = T | Lookup<T | undefined>;
/**
 * Most of the reserved animation props, except `to`, `from`, `loop`,
 * and the event props.
 */
interface AnimationProps<T = any> {
    /**
     * Configure the spring behavior for each key.
     */
    config?: SpringConfig | ((key: StringKeys<T>) => SpringConfig);
    /**
     * Milliseconds to wait before applying the other props.
     */
    delay?: number | ((key: StringKeys<T>) => number);
    /**
     * When true, props jump to their goal values instead of animating.
     */
    immediate?: MatchProp<T>;
    /**
     * Cancel all animations by using `true`, or some animations by using a key
     * or an array of keys.
     */
    cancel?: MatchProp<T>;
    /**
     * Pause all animations by using `true`, or some animations by using a key
     * or an array of keys.
     */
    pause?: MatchProp<T>;
    /**
     * Start the next animations at their values in the `from` prop.
     */
    reset?: MatchProp<T>;
    /**
     * Swap the `to` and `from` props.
     */
    reverse?: boolean;
    /**
     * Override the default props with this update.
     */
    default?: boolean | SpringProps<T>;
}
/**
 * Extract the custom props that are treated like `to` values
 */
type ForwardProps<T extends object> = RawValues<Omit<Constrain<T, {}>, keyof ReservedProps>>;
/**
 * Property names that are reserved for animation config
 */
interface ReservedProps extends ReservedEventProps {
    config?: any;
    from?: any;
    to?: any;
    ref?: any;
    loop?: any;
    pause?: any;
    reset?: any;
    cancel?: any;
    reverse?: any;
    immediate?: any;
    default?: any;
    delay?: any;
    items?: any;
    trail?: any;
    sort?: any;
    expires?: any;
    initial?: any;
    enter?: any;
    update?: any;
    leave?: any;
    children?: any;
    keys?: any;
    callId?: any;
    parentId?: any;
}
interface ReservedEventProps {
    onProps?: any;
    onStart?: any;
    onChange?: any;
    onPause?: any;
    onResume?: any;
    onRest?: any;
    onResolve?: any;
    onDestroyed?: any;
}
/**
 * Pick the properties of these object props...
 *
 *     "to", "from", "initial", "enter", "update", "leave"
 *
 * ...as well as any forward props.
 */
type PickAnimated<Props extends object, Fwd = true> = unknown & ([Props] extends [Any] ? Lookup : [object] extends [Props] ? Lookup : ObjectFromUnion<Props extends {
    from: infer From;
} ? From extends () => any ? ReturnType<From> : ObjectType<From> : TransitionKey & keyof Props extends never ? ToValues<Props, Fwd> : TransitionValues<Props>>);
/**
 * Pick the values of the `to` prop. Forward props are *not* included.
 */
type ToValues<Props extends object, AndForward = true> = unknown & (AndForward extends true ? ForwardProps<Props> : unknown) & (Props extends {
    to?: any;
} ? Exclude<Props['to'], Function | ReadonlyArray<any>> extends infer To ? ForwardProps<[To] extends [object] ? To : Partial<Extract<To, object>>> : never : unknown);

interface ControllerUpdateFn<State extends Lookup = Lookup> {
    (i: number, ctrl: Controller<State>): ControllerUpdate<State> | Falsy;
}
interface SpringRef<State extends Lookup = Lookup> {
    (props?: ControllerUpdate<State> | ControllerUpdateFn<State>): AsyncResult<Controller<State>>[];
    current: Controller<State>[];
    /** Add a controller to this ref */
    add(ctrl: Controller<State>): void;
    /** Remove a controller from this ref */
    delete(ctrl: Controller<State>): void;
    /** Pause all animations. */
    pause(): this;
    /** Pause animations for the given keys. */
    pause(keys: OneOrMore<string>): this;
    /** Pause some or all animations. */
    pause(keys?: OneOrMore<string>): this;
    /** Resume all animations. */
    resume(): this;
    /** Resume animations for the given keys. */
    resume(keys: OneOrMore<string>): this;
    /** Resume some or all animations. */
    resume(keys?: OneOrMore<string>): this;
    /** Update the state of each controller without animating. */
    set(values: Partial<State>): void;
    /** Update the state of each controller without animating based on their passed state. */
    set(values: (index: number, ctrl: Controller<State>) => Partial<State>): void;
    /** Start the queued animations of each controller. */
    start(): AsyncResult<Controller<State>>[];
    /** Update every controller with the same props. */
    start(props: ControllerUpdate<State>): AsyncResult<Controller<State>>[];
    /** Update controllers based on their state. */
    start(props: ControllerUpdateFn<State>): AsyncResult<Controller<State>>[];
    /** Start animating each controller. */
    start(props?: ControllerUpdate<State> | ControllerUpdateFn<State>): AsyncResult<Controller<State>>[];
    /** Stop all animations. */
    stop(): this;
    /** Stop animations for the given keys. */
    stop(keys: OneOrMore<string>): this;
    /** Cancel all animations. */
    stop(cancel: boolean): this;
    /** Cancel animations for the given keys. */
    stop(cancel: boolean, keys: OneOrMore<string>): this;
    /** Stop some or all animations. */
    stop(keys?: OneOrMore<string>): this;
    /** Cancel some or all animations. */
    stop(cancel: boolean, keys?: OneOrMore<string>): this;
    /** Add the same props to each controller's update queue. */
    update(props: ControllerUpdate<State>): this;
    /** Generate separate props for each controller's update queue. */
    update(props: ControllerUpdateFn<State>): this;
    /** Add props to each controller's update queue. */
    update(props: ControllerUpdate<State> | ControllerUpdateFn<State>): this;
    _getProps(arg: ControllerUpdate<State> | ControllerUpdateFn<State>, ctrl: Controller<State>, index: number): ControllerUpdate<State> | Falsy;
}
declare const SpringRef: <State extends Lookup = Lookup<any>>() => SpringRef<State>;

/**
 * Used to orchestrate animation hooks in sequence with one another.
 * This is best used when you specifically want to orchestrate different
 * types of animation hook e.g. `useSpring` & `useTransition` in
 * sequence as opposed to multiple `useSpring` hooks.
 *
 *
 * ```jsx
 * export const MyComponent = () => {
 *  //...
 *  useChain([springRef, transitionRef])
 *  //...
 * }
 * ```
 *
 * @param refs – An array of `SpringRef`s.
 * @param timeSteps – Optional array of numbers that define the
 * delay between each animation from 0-1. The length should correlate
 * to the length of `refs`.
 * @param timeFrame – Optional number that defines the total duration
 *
 * @public
 */
declare function useChain(refs: ReadonlyArray<SpringRef>, timeSteps?: number[], timeFrame?: number): void;

/**
 * The props that `useSpring` recognizes.
 */
type UseSpringProps<Props extends object = any> = unknown & PickAnimated<Props> extends infer State ? State extends Lookup ? Remap<ControllerUpdate<State> & {
    /**
     * Used to access the imperative API.
     *
     * When defined, the render animation won't auto-start.
     */
    ref?: SpringRef<State>;
}> : never : never;
/**
 * The `props` function is only called on the first render, unless
 * `deps` change (when defined). State is inferred from forward props.
 */
declare function useSpring<Props extends object>(props: Function | (() => (Props & Valid<Props, UseSpringProps<Props>>) | UseSpringProps), deps?: readonly any[] | undefined): PickAnimated<Props> extends infer State ? State extends Lookup ? [SpringValues<State>, SpringRef<State>] : never : never;
/**
 * Updated on every render, with state inferred from forward props.
 */
declare function useSpring<Props extends object>(props: (Props & Valid<Props, UseSpringProps<Props>>) | UseSpringProps): SpringValues<PickAnimated<Props>>;
/**
 * Updated only when `deps` change, with state inferred from forward props.
 */
declare function useSpring<Props extends object>(props: (Props & Valid<Props, UseSpringProps<Props>>) | UseSpringProps, deps: readonly any[] | undefined): PickAnimated<Props> extends infer State ? State extends Lookup ? [SpringValues<State>, SpringRef<State>] : never : never;

type UseSpringsProps<State extends Lookup = Lookup> = unknown & ControllerUpdate<State> & {
    ref?: SpringRef<State>;
};
/**
 * When the `deps` argument exists, the `props` function is called whenever
 * the `deps` change on re-render.
 *
 * Without the `deps` argument, the `props` function is only called once.
 */
declare function useSprings<Props extends UseSpringProps>(length: number, props: (i: number, ctrl: Controller) => Props, deps?: readonly any[]): PickAnimated<Props> extends infer State ? State extends Lookup<any> ? [SpringValues<State>[], SpringRef<State>] : never : never;
/**
 * Animations are updated on re-render.
 */
declare function useSprings<Props extends UseSpringsProps>(length: number, props: Props[] & UseSpringsProps<PickAnimated<Props>>[]): SpringValues<PickAnimated<Props>>[];
/**
 * When the `deps` argument exists, you get the `update` and `stop` function.
 */
declare function useSprings<Props extends UseSpringsProps>(length: number, props: Props[] & UseSpringsProps<PickAnimated<Props>>[], deps: DependencyList): PickAnimated<Props> extends infer State ? State extends Lookup<any> ? [SpringValues<State>[], SpringRef<State>] : never : never;

declare const useSpringRef: <State extends Lookup = Lookup<any>>() => SpringRef<State>;

/**
 * Creates a constant single `SpringValue` that can be interacted
 * with imperatively. This is an advanced API and does not react
 * to updates from the parent component e.g. passing a new initial value
 *
 *
 * ```jsx
 * export const MyComponent = () => {
 *   const opacity = useSpringValue(1)
 *
 *   return <animated.div style={{ opacity }} />
 * }
 * ```
 *
 * @param initial – The initial value of the `SpringValue`.
 * @param props – Typically the same props as `useSpring` e.g. `config`, `loop` etc.
 *
 * @public
 */
declare const useSpringValue: <T>(initial: Exclude<T, object>, props?: SpringUpdate<T>) => SpringValue<T>;

type UseTrailProps<Props extends object = any> = UseSpringProps<Props>;
declare function useTrail<Props extends object>(length: number, props: (i: number, ctrl: Controller) => UseTrailProps | (Props & Valid<Props, UseTrailProps<Props>>), deps?: readonly any[]): PickAnimated<Props> extends infer State ? State extends Lookup<any> ? [SpringValues<State>[], SpringRef<State>] : never : never;
/**
 * This hook is an abstraction around `useSprings` and is designed to
 * automatically orchestrate the springs to stagger one after the other
 *
 * ```jsx
 * export const MyComponent = () => {
 *  const trails = useTrail(3, {opacity: 0})
 *
 *  return trails.map(styles => <animated.div style={styles} />)
 * }
 * ```
 *
 * @param length – The number of springs you want to create
 * @param propsArg – The props to pass to the internal `useSprings` hook,
 * therefore is the same as `useSprings`.
 *
 * @public
 */
declare function useTrail<Props extends object>(length: number, props: UseTrailProps | (Props & Valid<Props, UseTrailProps<Props>>)): SpringValues<PickAnimated<Props>>[];
/**
 * This hook is an abstraction around `useSprings` and is designed to
 * automatically orchestrate the springs to stagger one after the other
 *
 * ```jsx
 * export const MyComponent = () => {
 *  const trails = useTrail(3, {opacity: 0}, [])
 *
 *  return trails.map(styles => <animated.div style={styles} />)
 * }
 * ```
 *
 * @param length – The number of springs you want to create
 * @param propsArg – The props to pass to the internal `useSprings` hook,
 * therefore is the same as `useSprings`.
 * @param deps – The optional array of dependencies to pass to the internal
 * `useSprings` hook, therefore is the same as `useSprings`.
 *
 * @public
 */
declare function useTrail<Props extends object>(length: number, props: UseTrailProps | (Props & Valid<Props, UseTrailProps<Props>>), deps: readonly any[]): PickAnimated<Props> extends infer State ? State extends Lookup<any> ? [SpringValues<State>[], SpringRef<State>] : never : never;

declare function useTransition<Item, Props extends object>(data: OneOrMore<Item>, props: () => UseTransitionProps<Item> | (Props & Valid<Props, UseTransitionProps<Item>>), deps?: any[]): PickAnimated<Props> extends infer State ? State extends Lookup ? [TransitionFn<Item, PickAnimated<Props>>, SpringRef<State>] : never : never;
declare function useTransition<Item, Props extends object>(data: OneOrMore<Item>, props: UseTransitionProps<Item> | (Props & Valid<Props, UseTransitionProps<Item>>)): TransitionFn<Item, PickAnimated<Props>>;
declare function useTransition<Item, Props extends object>(data: OneOrMore<Item>, props: UseTransitionProps<Item> | (Props & Valid<Props, UseTransitionProps<Item>>), deps: any[] | undefined): PickAnimated<Props> extends infer State ? State extends Lookup ? [TransitionFn<Item, State>, SpringRef<State>] : never : never;

interface UseScrollOptions extends Omit<SpringProps, 'to' | 'from'> {
    container?: MutableRefObject<HTMLElement>;
}
/**
 * A small utility abstraction around our signature useSpring hook. It's a great way to create
 * a scroll-linked animation. With either the raw value of distance or a 0-1 progress value.
 * You can either use the scroll values of the whole document, or just a specific element.
 *
 *
 ```jsx
    import { useScroll, animated } from '@react-spring/web'

    function MyComponent() {
      const { scrollYProgress } = useScroll()

      return (
        <animated.div style={{ opacity: scrollYProgress }}>
          Hello World
        </animated.div>
      )
    }
  ```
 *
 * @param {UseScrollOptions} useScrollOptions options for the useScroll hook.
 * @param {MutableRefObject<HTMLElement>} useScrollOptions.container the container to listen to scroll events on, defaults to the window.
 *
 * @returns {SpringValues<{scrollX: number; scrollY: number; scrollXProgress: number; scrollYProgress: number}>} SpringValues the collection of values returned from the inner hook
 */
declare const useScroll: ({ container, ...springOptions }?: UseScrollOptions) => SpringValues<{
    scrollX: number;
    scrollY: number;
    scrollXProgress: number;
    scrollYProgress: number;
}>;

interface UseResizeOptions extends Omit<SpringProps, 'to' | 'from'> {
    container?: MutableRefObject<HTMLElement | null | undefined>;
}
/**
 * A small abstraction around the `useSpring` hook. It returns a `SpringValues`
 * object with the `width` and `height` of the element it's attached to & doesn't
 * necessarily have to be attached to the window, by passing a `container` you
 * can observe that element's size instead.
 *
 ```jsx
    import { useResize, animated } from '@react-spring/web'

    function MyComponent() {
      const { width } = useResize()

      return (
        <animated.div style={{ width }}>
          Hello World
        </animated.div>
      )
    }
  ```
 *
 * @param {UseResizeOptions} UseResizeOptions options for the useScroll hook.
 * @param {MutableRefObject<HTMLElement>} UseResizeOptions.container the container to listen to scroll events on, defaults to the window.
 *
 * @returns {SpringValues<{width: number; height: number;}>} SpringValues the collection of values returned from the inner hook
 */
declare const useResize: ({ container, ...springOptions }: UseResizeOptions) => SpringValues<{
    width: number;
    height: number;
}>;

interface IntersectionArgs extends Omit<IntersectionObserverInit, 'root' | 'threshold'> {
    root?: React.MutableRefObject<HTMLElement>;
    once?: boolean;
    amount?: 'any' | 'all' | number | number[];
}
declare function useInView(args?: IntersectionArgs): [RefObject<any>, boolean];
declare function useInView<Props extends object>(
/**
 * TODO: make this narrower to only accept reserved props.
 */
props: () => Props & Valid<Props, UseSpringProps<Props>>, args?: IntersectionArgs): PickAnimated<Props> extends infer State ? State extends Lookup ? [RefObject<any>, SpringValues<State>] : never : never;

type SpringComponentProps<State extends object = UnknownProps> = unknown & UseSpringProps<State> & {
    children: (values: SpringValues<State>) => React.JSX.Element | null;
};
declare function Spring<State extends object>(props: {
    from: State;
    to?: SpringChain<NoInfer<State>> | SpringToFn<NoInfer<State>>;
} & Omit<SpringComponentProps<NoInfer<State>>, 'from' | 'to'>): JSX.Element | null;
declare function Spring<State extends object>(props: {
    to: State;
} & Omit<SpringComponentProps<NoInfer<State>>, 'to'>): JSX.Element | null;

type TrailComponentProps<Item, Props extends object = any> = unknown & UseSpringProps<Props> & {
    items: readonly Item[];
    children: (item: NoInfer<Item>, index: number) => ((values: SpringValues<PickAnimated<Props>>) => ReactNode) | Falsy;
};
declare function Trail<Item, Props extends TrailComponentProps<Item>>({ items, children, ...props }: Props & Valid<Props, TrailComponentProps<Item, Props>>): (string | number | bigint | boolean | react.ReactElement<unknown, string | react.JSXElementConstructor<any>> | Iterable<ReactNode> | Promise<string | number | bigint | boolean | react.ReactPortal | react.ReactElement<unknown, string | react.JSXElementConstructor<any>> | Iterable<ReactNode> | null | undefined> | null | undefined)[];

declare function Transition<Item, Props extends TransitionComponentProps<Item>>(props: TransitionComponentProps<Item> | (Props & Valid<Props, TransitionComponentProps<Item, Props>>)): JSX.Element;

/** Map the value of one or more dependencies */
declare const to: Interpolator;
/** @deprecated Use the `to` export instead */
declare const interpolate: Interpolator;
/** Extract the raw value types that are being interpolated */
type Interpolated<T extends ReadonlyArray<any>> = {
    [P in keyof T]: T[P] extends infer Element ? Element extends FluidValue<infer U> ? U : Element : never;
};
/**
 * This interpolates one or more `FluidValue` objects.
 * The exported `interpolate` function uses this type.
 */
interface Interpolator {
    <Input extends ReadonlyArray<any>, Output>(parents: Input, interpolator: (...args: Interpolated<Input>) => Output): Interpolation<Output>;
    <Input, Output>(parent: FluidValue<Input> | Input, interpolator: InterpolatorFn<Input, Output>): Interpolation<Output>;
    <Out>(parents: OneOrMore<FluidValue>, config: InterpolatorConfig<Out>): Interpolation<Animatable<Out>>;
    <Out>(parents: OneOrMore<FluidValue<number>> | FluidValue<number[]>, range: readonly number[], output: readonly Constrain<Out, Animatable>[], extrapolate?: ExtrapolateType): Interpolation<Animatable<Out>>;
}

declare const config: {
    readonly default: {
        readonly tension: 170;
        readonly friction: 26;
    };
    readonly gentle: {
        readonly tension: 120;
        readonly friction: 14;
    };
    readonly wobbly: {
        readonly tension: 180;
        readonly friction: 12;
    };
    readonly stiff: {
        readonly tension: 210;
        readonly friction: 20;
    };
    readonly slow: {
        readonly tension: 280;
        readonly friction: 60;
    };
    readonly molasses: {
        readonly tension: 280;
        readonly friction: 120;
    };
};

/** Advance all animations by the given time */
declare const update: (dt: number) => boolean;

/**
 * This context affects all new and existing `SpringValue` objects
 * created with the hook API or the renderprops API.
 */
interface ISpringContext {
    /** Pause all new and existing animations. */
    pause?: boolean;
    /** Force all new and existing animations to be immediate. */
    immediate?: boolean;
}
declare const SpringContext: react.Context<ISpringContext>;

/**
 * Clone the given `props` and move all non-reserved props
 * into the `to` prop.
 */
declare function inferTo<T extends object>(props: T): InferTo<T>;

export { AnimationConfig, type AnimationProps, type AnimationResult, type AsyncResult, BailSignal, type Change, Controller, type ControllerFlushFn, type ControllerProps, type ControllerUpdate, type EventProp, type ForwardProps, FrameValue, type GoalProp, type GoalValue, type GoalValues, type InferTo, type InlineToProps, type Interpolated, Interpolation, type Interpolator, type IntersectionArgs, type ItemKeys, type LoopProp, type MatchProp, type OnChange, type OnPause, type OnProps, type OnResolve, type OnRest, type OnResume, type OnStart, type PickAnimated, type ReservedEventProps, type ReservedProps, Spring, type SpringChain, type SpringComponentProps, type SpringConfig, SpringContext, type SpringProps, SpringRef, type SpringTo, type SpringToFn, type SpringUpdate, type SpringUpdateFn, SpringValue, type SpringValues, type Springify, type SpringsUpdate, type ToProps, type ToValues, Trail, type TrailComponentProps, Transition, type TransitionComponentProps, type TransitionFn, type TransitionFrom, type TransitionKey, type TransitionRenderFn, type TransitionState, type TransitionTo, type TransitionValues, type UseResizeOptions, type UseScrollOptions, type UseSpringProps, type UseSpringsProps, type UseTrailProps, type UseTransitionProps, type VelocityProp, config, inferTo, interpolate, to, update, useChain, useInView, useResize, useScroll, useSpring, useSpringRef, useSpringValue, useSprings, useTrail, useTransition };
