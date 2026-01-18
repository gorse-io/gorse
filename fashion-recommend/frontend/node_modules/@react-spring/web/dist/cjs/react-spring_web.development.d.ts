import { JSX, ElementType, ForwardRefExoticComponent, CSSProperties, ComponentPropsWithRef } from 'react';
import { FluidValue, FluidProps } from '@react-spring/shared';
import { Merge } from '@react-spring/types';
export * from '@react-spring/core';
import * as csstype from 'csstype';
export { csstype as CSS };

type Primitives = keyof JSX.IntrinsicElements;

type AnimatedPrimitives = {
    [Tag in Primitives]: AnimatedComponent<Tag>;
};
/** The type of the `animated()` function */
type WithAnimated = {
    <T extends ElementType>(wrappedComponent: T): AnimatedComponent<T>;
} & AnimatedPrimitives;
/** The type of an `animated()` component */
type AnimatedComponent<T extends ElementType> = ForwardRefExoticComponent<AnimatedProps<Merge<ComponentPropsWithRef<T>, {
    style?: StyleProps;
}>> & FluidProps<{
    scrollTop?: number;
    scrollLeft?: number;
    viewBox?: string;
}>>;
/** The props of an `animated()` component */
type AnimatedProps<Props extends object> = {
    [P in keyof Props]: P extends 'ref' | 'key' ? Props[P] : AnimatedProp<Props[P]>;
};
type StyleProps = Merge<CSSProperties, TransformProps>;
type StylePropKeys = keyof StyleProps;
type ValidStyleProps<T extends object> = {
    [P in keyof T & StylePropKeys]: T[P] extends StyleProps[P] ? P : never;
}[keyof T & StylePropKeys];
type AnimatedProp<T> = [T, T] extends [infer T, infer DT] ? [DT] extends [never] ? never : DT extends void ? undefined : DT extends string | number ? DT | AnimatedLeaf<T> : DT extends object ? [ValidStyleProps<DT>] extends [never] ? DT extends ReadonlyArray<any> ? AnimatedStyles<DT> : DT : AnimatedStyle<T> : DT | AnimatedLeaf<T> : never;
type AnimatedStyles<T extends ReadonlyArray<any>> = {
    [P in keyof T]: [T[P]] extends [infer DT] ? DT extends object ? [ValidStyleProps<DT>] extends [never] ? DT extends ReadonlyArray<any> ? AnimatedStyles<DT> : DT : {
        [P in keyof DT]: AnimatedProp<DT[P]>;
    } : DT : never;
};
type AnimatedStyle<T> = [T, T] extends [infer T, infer DT] ? DT extends void ? undefined : [DT] extends [never] ? never : DT extends string | number ? DT | AnimatedLeaf<T> : DT extends object ? AnimatedObject<DT> : DT | AnimatedLeaf<T> : never;
type AnimatedObject<T extends object> = {
    [P in keyof T]: AnimatedStyle<T[P]>;
} | (T extends ReadonlyArray<number | string> ? FluidValue<Readonly<T>> : never);
type AnimatedLeaf<T> = NonObject<T> extends infer U ? [U] extends [never] ? never : FluidValue<U> : never;
type NonObject<T> = Extract<T, string | number | ReadonlyArray<string | number>> | Exclude<T, object | void>;
type Angle = number | string;
type Length = number | string;
type TransformProps = {
    transform?: string;
    x?: Length;
    y?: Length;
    z?: Length;
    translate?: Length | readonly [Length, Length];
    translateX?: Length;
    translateY?: Length;
    translateZ?: Length;
    translate3d?: readonly [Length, Length, Length];
    rotate?: Angle;
    rotateX?: Angle;
    rotateY?: Angle;
    rotateZ?: Angle;
    rotate3d?: readonly [number, number, number, Angle];
    scale?: number | readonly [number, number] | string;
    scaleX?: number;
    scaleY?: number;
    scaleZ?: number;
    scale3d?: readonly [number, number, number];
    skew?: Angle | readonly [Angle, Angle];
    skewX?: Angle;
    skewY?: Angle;
    matrix?: readonly [number, number, number, number, number, number];
    matrix3d?: readonly [
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number,
        number
    ];
};

declare const animated: WithAnimated;

export { type AnimatedComponent, type AnimatedProps, type WithAnimated, animated as a, animated };
