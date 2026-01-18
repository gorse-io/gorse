import { noop } from '../utils/noop.mjs';
import { createRenderBatcher } from './batcher.mjs';

const { schedule: frame, cancel: cancelFrame, state: frameData, steps, } = createRenderBatcher(typeof requestAnimationFrame !== "undefined" ? requestAnimationFrame : noop, true);

export { cancelFrame, frame, frameData, steps };
