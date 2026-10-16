import { describe, expect, it } from "vitest";
import { availableActions, invitationLabel } from "../workflow";
describe("recruitment workflow", () => {
  it("offers only legal actions and closes terminal states", () => {
    expect(availableActions("interviewing")).toEqual(["send_offer", "reject"]);
    expect(availableActions("offered")).toEqual(["hire", "reject"]);
    expect(availableActions("hired")).toEqual([]);
    expect(availableActions("rejected")).toEqual([]);
  });
  it("retains accepted and revoked status even after expiry", () => {
    expect(invitationLabel({ status: "accepted", expired: true })).toBe(
      "已接受",
    );
    expect(invitationLabel({ status: "revoked", expired: true })).toBe(
      "已撤销",
    );
    expect(invitationLabel({ status: "pending", expired: true })).toBe(
      "已过期",
    );
    expect(invitationLabel({ status: "pending", expired: false })).toBe(
      "待接受",
    );
  });
});
