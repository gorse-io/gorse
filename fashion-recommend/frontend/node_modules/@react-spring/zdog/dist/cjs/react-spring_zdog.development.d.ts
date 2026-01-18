import { ElementType, ForwardRefExoticComponent, ComponentPropsWithRef } from 'react';
import { FluidValue } from '@react-spring/shared';
import * as Zdog from 'react-zdog';
export * from '@react-spring/core';

type ZdogExports = typeof Zdog;
type ZdogElements = {
    [P in keyof ZdogExports]: P extends 'Illustration' ? never : ZdogExports[P] extends ElementType ? P : never;
}[keyof ZdogExports];
declare const primitives: {
    [key in ZdogElements]: ElementType;
};

type Primitives = typeof primitives;
type AnimatedPrimitives = {
    [P in keyof Primitives]: AnimatedComponent<Primitives[P]>;
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
type AnimatedProp<T> = [T, T] extends [infer T, infer DT] ? [DT] extends [never] ? never : DT extends void ? undefined : DT extends object ? AnimatedStyle<T> : DT | AnimatedLeaf<T> : never;
type AnimatedStyle<T> = [T, T] extends [infer T, infer DT] ? DT extends void ? undefined : [DT] extends [never] ? never : DT extends object ? {
    [P in keyof DT]: AnimatedStyle<DT[P]>;
} : DT | AnimatedLeaf<T> : never;
type AnimatedLeaf<T> = Exclude<T, object | void> | Extract<T, ReadonlyArray<number | string>> extends infer U ? [U] extends [never] ? never : FluidValue<U | Exclude<T, object | void>> : never;

declare const animated: WithAnimated;

export { type AnimatedComponent, type AnimatedProps, type WithAnimated, animated as a, animated };
