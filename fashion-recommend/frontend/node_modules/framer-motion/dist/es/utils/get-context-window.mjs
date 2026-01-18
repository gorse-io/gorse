// Fixes https://github.com/framer/motion/issues/2270
const getContextWindow = ({ current }) => {
    return current ? current.ownerDocument.defaultView : null;
};

export { getContextWindow };
