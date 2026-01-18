import { scrollInfo } from './track.mjs';
import { observeTimeline } from './observe.mjs';
import { supportsScrollTimeline } from './supports.mjs';

function scrollTimelineFallback({ source, axis = "y" }) {
    // ScrollTimeline records progress as a percentage CSSUnitValue
    const currentTime = { value: 0 };
    const cancel = scrollInfo((info) => {
        currentTime.value = info[axis].progress * 100;
    }, { container: source, axis });
    return { currentTime, cancel };
}
const timelineCache = new Map();
function getTimeline({ source = document.documentElement, axis = "y", } = {}) {
    if (!timelineCache.has(source)) {
        timelineCache.set(source, {});
    }
    const elementCache = timelineCache.get(source);
    if (!elementCache[axis]) {
        elementCache[axis] = supportsScrollTimeline()
            ? new ScrollTimeline({ source, axis })
            : scrollTimelineFallback({ source, axis });
    }
    return elementCache[axis];
}
function scroll(onScroll, options) {
    const timeline = getTimeline(options);
    if (typeof onScroll === "function") {
        return observeTimeline(onScroll, timeline);
    }
    else {
        return onScroll.attachTimeline(timeline);
    }
}

export { scroll };
