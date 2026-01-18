import { JSX, ForwardRefExoticComponent, CSSProperties, FC } from 'react';
import { ElementType, AssignableKeys, ComponentPropsWithRef } from '@react-spring/types';
import { FluidValue } from '@react-spring/shared';
export * from '@react-spring/core';

type Primitives = keyof JSX.IntrinsicElements;

type AnimatedPrimitives = {
    [P in Primitives]: AnimatedComponent<FC<JSX.IntrinsicElements[P]>>;
};
/** The type of the `animated()` function */
type WithAnimated = {
    <T extends ElementType>(wrappedComponent: T): AnimatedComponent<T>;
} & AnimatedPrimitives;
/** The type of an `animated()` component */
type AnimatedComponent<T extends ElementType> = ForwardRefExoticComponent<AnimatedProps<ComponentPropsWithRef<T>>>;
/** The props of an `animated()` component */
type AnimatedProps<Props extends object> = {
    [P in keyof Props]: P extends 'ref' | 'key' ? Props[P] : AnimatedProp<Props[P]>;
};
type AnimatedProp<T> = [T, T] extends [infer T, infer DT] ? [DT] extends [never] ? never : DT extends void ? undefined : DT extends object ? [AssignableKeys<DT, CSSProperties>] extends [never] ? DT extends ReadonlyArray<any> ? AnimatedStyles<DT> : DT : AnimatedStyle<T> : DT | AnimatedLeaf<T> : never;
type AnimatedStyles<T extends ReadonlyArray<any>> = {
    [P in keyof T]: [T[P]] extends [infer DT] ? DT extends object ? [AssignableKeys<DT, CSSProperties>] extends [never] ? DT extends ReadonlyArray<any> ? AnimatedStyles<DT> : DT : {
        [P in keyof DT]: AnimatedProp<DT[P]>;
    } : DT : never;
};
type AnimatedStyle<T> = [T, T] extends [infer T, infer DT] ? DT extends void ? undefined : [DT] extends [never] ? never : DT extends object ? {
    [P in keyof DT]: AnimatedStyle<DT[P]>;
} : DT | AnimatedLeaf<T> : never;
type AnimatedLeaf<T> = Exclude<T, object | void> | Extract<T, ReadonlyArray<number | string>> extends infer U ? [U] extends [never] ? never : FluidValue<U | Exclude<T, object | void>> : never;

declare const animated: WithAnimated;

export { type AnimatedComponent, type AnimatedProps, type WithAnimated, animated as a, animated };
