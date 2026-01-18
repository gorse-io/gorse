import { memo } from '../../../utils/memo.mjs';

const supportsScrollTimeline = memo(() => window.ScrollTimeline !== undefined);

export { supportsScrollTimeline };
