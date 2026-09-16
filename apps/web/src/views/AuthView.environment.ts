import type { Environment } from "vitest/environments";

const environment: Environment = {
  name: "auth-view-renderer",
  transformMode: "web",
  setup(global) {
    const previousDocument = global.document;
    global.document = {
      activeElement: null,
      createElement: () => ({}),
    };

    return {
      teardown() {
        global.document = previousDocument;
      },
    };
  },
};

export default environment;
