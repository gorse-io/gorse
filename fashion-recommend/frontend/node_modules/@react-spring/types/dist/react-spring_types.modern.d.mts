import * as React from 'react';
import { MutableRefObject, ReactElement } from 'react';

/** These types can be animated */
type Animatable<T = any> = T extends number ? number : T extends string ? string : T extends ReadonlyArray<number | string> ? Array<number | string> extends T ? ReadonlyArray<number | string> : {
    [P in keyof T]: Animatable<T[P]>;
} : never;

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

/** Ensure each type of `T` is an array */
type Arrify<T> = [T, T] extends [infer T, infer DT] ? DT extends ReadonlyArray<any> ? Array<DT[number]> extends DT ? ReadonlyArray<T extends ReadonlyArray<infer U> ? U : T> : DT : ReadonlyArray<T extends ReadonlyArray<infer U> ? U : T> : never;
/** Override the property types of `A` with `B` and merge any new properties */
type Merge<A, B> = Remap<{
    [P in keyof A]: P extends keyof B ? B[P] : A[P];
} & Omit<B, keyof A>>;
/** Return the keys of `T` with values that are assignable to `U` */
type AssignableKeys<T, U> = T extends object ? U extends object ? {
    [P in Extract<keyof T, keyof U>]: T[P] extends U[P] ? P : never;
}[Extract<keyof T, keyof U>] : never : never;
/** Better type errors for overloads with generic types */
type Constrain<T, U> = [T] extends [Any] ? U : [T] extends [U] ? T : U;
/** Try to simplify `&` out of an object type */
type Remap<T> = {} & {
    [P in keyof T]: T[P];
};
type Pick<T, K extends keyof T> = {} & {
    [P in K]: T[P];
};
type Omit<T, K> = Pick<T, Exclude<keyof T, K>>;
type Partial<T> = {} & {
    [P in keyof T]?: T[P] | undefined;
};
type Overwrite<T, U> = Remap<Omit<T, keyof U> & U>;
type MergeUnknown<T, U> = Remap<T & Omit<U, keyof T>>;
type MergeDefaults<T extends object, U extends Partial<T>> = Remap<Omit<T, keyof U> & Partial<Pick<T, Extract<keyof U, keyof T>>>>;
type OneOrMore<T> = T | readonly T[];
type Falsy = false | null | undefined;
type NoInfer<T> = [T][T extends any ? 0 : never];
type StaticProps<T> = Omit<T, keyof T & 'prototype'>;
interface Lookup<T = any> {
    [key: string]: T;
}
type LoosePick<T, K> = {} & Pick<T, K & keyof T>;
/** Intersected with other object types to allow for unknown properties */
interface UnknownProps extends Lookup<unknown> {
}
/** Use `[T] extends [Any]` to know if a type parameter is `any` */
declare class Any {
    private _;
}
type AnyFn<In extends ReadonlyArray<any> = any[], Out = any> = (...args: In) => Out;
/** Ensure the given type is an object type */
type ObjectType<T> = T extends object ? T : {};
/** Intersect a union of objects but merge property types with _unions_ */
type ObjectFromUnion<T extends object> = Remap<{
    [P in keyof Intersect<T>]: T extends infer U ? P extends keyof U ? U[P] : never : never;
}>;
/** Convert a union to an intersection */
type Intersect<U> = (U extends any ? (k: U) => void : never) extends (k: infer I) => void ? I : never;
type AllKeys<T> = T extends any ? keyof T : never;
type Exclusive<T> = AllKeys<T> extends infer K ? T extends any ? Remap<LoosePick<T, K> & {
    [P in Exclude<K & keyof any, keyof T>]?: undefined;
}> : never : never;
/** An object that needs to be manually disposed of */
interface Disposable {
    dispose(): void;
}
type RefProp<T> = MutableRefObject<T | null | undefined>;
type ElementType<P = any> = React.ElementType<P> | LeafFunctionComponent<P>;
type LeafFunctionComponent<P> = {
    (props: P): ReactElement | null;
    displayName?: string;
};
type ComponentPropsWithRef<T extends ElementType> = T extends React.ComponentClass<infer P> ? React.PropsWithoutRef<P> & React.RefAttributes<InstanceType<T>> : React.PropsWithRef<React.ComponentProps<T>>;
type ComponentType<P = {}> = React.ComponentClass<P> | LeafFunctionComponent<P>;

type EasingFunction = (t: number) => number;
type ExtrapolateType = 'identity' | 'clamp' | 'extend';
interface InterpolatorFactory {
    <Input, Output>(interpolator: InterpolatorFn<Input, Output>): typeof interpolator;
    <Output>(config: InterpolatorConfig<Output>): (input: number) => Animatable<Output>;
    <Output>(range: readonly number[], output: readonly Constrain<Output, Animatable>[], extrapolate?: ExtrapolateType): (input: number) => Animatable<Output>;
    <Input, Output>(...args: InterpolatorArgs<Input, Output>): InterpolatorFn<Input, Output>;
}
type InterpolatorArgs<Input = any, Output = any> = [InterpolatorFn<Arrify<Input>, Output>] | [InterpolatorConfig<Output>] | [
    readonly number[],
    readonly Constrain<Output, Animatable>[],
    (ExtrapolateType | undefined)?
];
type InterpolatorFn<Input, Output> = (...inputs: Arrify<Input>) => Output;
type InterpolatorConfig<Output = Animatable> = {
    /**
     * What happens when the spring goes below its target value.
     *
     *  - `extend` continues the interpolation past the target value
     *  - `clamp` limits the interpolation at the max value
     *  - `identity` sets the value to the interpolation input as soon as it hits the boundary
     *
     * @default 'extend'
     */
    extrapolateLeft?: ExtrapolateType;
    /**
     * What happens when the spring exceeds its target value.
     *
     *  - `extend` continues the interpolation past the target value
     *  - `clamp` limits the interpolation at the max value
     *  - `identity` sets the value to the interpolation input as soon as it hits the boundary
     *
     * @default 'extend'
     */
    extrapolateRight?: ExtrapolateType;
    /**
     * What happens when the spring exceeds its target value.
     * Shortcut to set `extrapolateLeft` and `extrapolateRight`.
     *
     *  - `extend` continues the interpolation past the target value
     *  - `clamp` limits the interpolation at the max value
     *  - `identity` sets the value to the interpolation input as soon as it hits the boundary
     *
     * @default 'extend'
     */
    extrapolate?: ExtrapolateType;
    /**
     * Input ranges mapping the interpolation to the output values.
     *
     * @example
     *
     *   range: [0, 0.5, 1], output: ['yellow', 'orange', 'red']
     *
     * @default [0,1]
     */
    range?: readonly number[];
    /**
     * Output values from the interpolation function. Should match the length of the `range` array.
     */
    output: readonly Constrain<Output, Animatable>[];
    /**
     * Transformation to apply to the value before interpolation.
     */
    map?: (value: number) => number;
    /**
     * Custom easing to apply in interpolator.
     */
    easing?: EasingFunction;
};

export { type AllKeys, type Animatable, Any, type AnyFn, type Arrify, type AssignableKeys, type ComponentPropsWithRef, type ComponentType, type Constrain, type Disposable, type EasingFunction, type ElementType, type Exclusive, type ExtrapolateType, type Falsy, type InterpolatorArgs, type InterpolatorConfig, type InterpolatorFactory, type InterpolatorFn, type Lookup, type LoosePick, type Merge, type MergeDefaults, type MergeUnknown, type NoInfer, type ObjectFromUnion, type ObjectType, type Omit, type OneOrMore, type Overwrite, type Partial, type Pick, type RefProp, type Remap, type StaticProps, type UnknownProps };
