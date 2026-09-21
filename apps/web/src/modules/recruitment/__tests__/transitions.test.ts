import { describe, expect, it } from "vitest";

import {
  PROSPECT_STATUSES,
  canMarkFirstPass,
  canSendEmail,
  nextStatusAfterEmail,
  prospectStatusMeta,
} from "../status";

describe("recruitment status", () => {
  it("maps every status to a non-empty label", () => {
    for (const status of PROSPECT_STATUSES) {
      expect(prospectStatusMeta(status).label.length).toBeGreaterThan(0);
    }
  });

  it("moves first_passed -> ready_second on pass email", () => {
    expect(nextStatusAfterEmail("first_passed", "pass")).toBe("ready_second");
  });

  it("moves ready_second -> hired on pass email", () => {
    expect(nextStatusAfterEmail("ready_second", "pass")).toBe("hired");
  });

  it("moves any sendable status -> rejected on reject email", () => {
    expect(nextStatusAfterEmail("ready_first", "reject")).toBe("rejected");
    expect(nextStatusAfterEmail("ready_second", "reject")).toBe("rejected");
  });

  it("only allows reject email from ready_first / ready_second", () => {
    expect(canSendEmail("ready_first", "reject")).toBe(true);
    expect(canSendEmail("ready_second", "reject")).toBe(true);
    expect(canSendEmail("first_passed", "reject")).toBe(false);
    expect(canSendEmail("hired", "reject")).toBe(false);
  });

  it("only allows pass email from first_passed / ready_second", () => {
    expect(canSendEmail("first_passed", "pass")).toBe(true);
    expect(canSendEmail("ready_second", "pass")).toBe(true);
    expect(canSendEmail("ready_first", "pass")).toBe(false);
    expect(canSendEmail("hired", "pass")).toBe(false);
  });

  it("only allows marking first pass from ready_first", () => {
    expect(canMarkFirstPass("ready_first")).toBe(true);
    expect(canMarkFirstPass("first_passed")).toBe(false);
  });
});
