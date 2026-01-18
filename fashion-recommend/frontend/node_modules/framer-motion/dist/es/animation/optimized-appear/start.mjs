import { appearStoreId } from './store-id.mjs';
import { animateStyle } from '../animators/waapi/index.mjs';
import { optimizedAppearDataId } from './data-id.mjs';
import { handoffOptimizedAppearAnimation } from './handoff.mjs';
import { appearAnimationStore } from './store.mjs';
import { noop } from '../../utils/noop.mjs';

/**
 * A single time to use across all animations to manually set startTime
 * and ensure they're all in sync.
 */
let startFrameTime;
/**
 * A dummy animation to detect when Chrome is ready to start
 * painting the page and hold off from triggering the real animation
 * until then. We only need one animation to detect paint ready.
 *
 * https://bugs.chromium.org/p/chromium/issues/detail?id=1406850
 */
let readyAnimation;
function startOptimizedAppearAnimation(element, name, keyframes, options, onReady) {
    // Prevent optimised appear animations if Motion has already started animating.
    if (window.HandoffComplete) {
        window.HandoffAppearAnimations = undefined;
        return;
    }
    const id = element.dataset[optimizedAppearDataId];
    if (!id)
        return;
    window.HandoffAppearAnimations = handoffOptimizedAppearAnimation;
    const storeId = appearStoreId(id, name);
    if (!readyAnimation) {
        readyAnimation = animateStyle(element, name, [keyframes[0], keyframes[0]], 
        /**
         * 10 secs is basically just a super-safe duration to give Chrome
         * long enough to get the animation ready.
         */
        { duration: 10000, ease: "linear" });
        appearAnimationStore.set(storeId, {
            animation: readyAnimation,
            startTime: null,
        });
    }
    const startAnimation = () => {
        readyAnimation.cancel();
        const appearAnimation = animateStyle(element, name, keyframes, options);
        /**
         * Record the time of the first started animation. We call performance.now() once
         * here and once in handoff to ensure we're getting
         * close to a frame-locked time. This keeps all animations in sync.
         */
        if (startFrameTime === undefined) {
            startFrameTime = performance.now();
        }
        appearAnimation.startTime = startFrameTime;
        appearAnimationStore.set(storeId, {
            animation: appearAnimation,
            startTime: startFrameTime,
        });
        if (onReady)
            onReady(appearAnimation);
    };
    if (readyAnimation.ready) {
        readyAnimation.ready.then(startAnimation).catch(noop);
    }
    else {
        startAnimation();
    }
}

export { startOptimizedAppearAnimation };
