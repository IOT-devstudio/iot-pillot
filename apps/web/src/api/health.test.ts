import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchHealth } from "./health";

afterEach(() => vi.restoreAllMocks());

describe("health api", () => {
  it("requests the backend root health endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { status: "ok", time: "2026-09-17T10:00:00Z" },
        }),
        { status: 200 },
      ),
    );

    await expect(fetchHealth()).resolves.toEqual({
      status: "ok",
      time: "2026-09-17T10:00:00Z",
    });
    expect(fetchMock).toHaveBeenCalledWith("/health");
  });
});
