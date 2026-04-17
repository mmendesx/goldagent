import { describe, it, expect } from "vitest";
import {
  formatCurrency,
  formatPercent,
  formatPrice,
  getDrawdownSeverity,
} from "./index";

describe("formatCurrency", () => {
  it("formats a numeric string with two decimal places and a dollar sign", () => {
    expect(formatCurrency("1234.567")).toBe("$1,234.57");
  });

  it("formats zero correctly", () => {
    expect(formatCurrency("0")).toBe("$0.00");
  });

  it("returns em dash for a NaN number value", () => {
    expect(formatCurrency(NaN)).toBe("—");
  });

  it("returns em dash for a non-numeric string", () => {
    expect(formatCurrency("not-a-number")).toBe("—");
  });

  it("formats a plain number", () => {
    expect(formatCurrency(42)).toBe("$42.00");
  });
});

describe("formatPercent", () => {
  it("formats a percentage string with two decimal places", () => {
    expect(formatPercent("5.5")).toBe("5.50%");
  });

  it("formats zero percent", () => {
    expect(formatPercent("0")).toBe("0.00%");
  });

  it("returns em dash for invalid input", () => {
    expect(formatPercent("bad")).toBe("—");
  });
});

describe("formatPrice", () => {
  it("formats large values with comma separator and two decimals", () => {
    expect(formatPrice("65000.50")).toBe("65,000.50");
  });

  it("formats small fractional values with four to eight decimals", () => {
    expect(formatPrice("0.0001")).toBe("0.0001");
  });

  it("formats mid-range values between 1 and 1000", () => {
    expect(formatPrice("9.99")).toBe("9.99");
  });

  it("returns em dash for invalid input", () => {
    expect(formatPrice("bad")).toBe("—");
  });
});

describe("getDrawdownSeverity", () => {
  it("returns undefined for drawdown below 10%", () => {
    expect(getDrawdownSeverity(3)).toBeUndefined();
  });

  it("returns undefined for drawdown just below 10%", () => {
    expect(getDrawdownSeverity(9.99)).toBeUndefined();
  });

  it("returns medium for drawdown at 10%", () => {
    expect(getDrawdownSeverity(10)).toBe("medium");
  });

  it("returns medium for drawdown between 10% and 15%", () => {
    expect(getDrawdownSeverity(12.5)).toBe("medium");
  });

  it("returns high for drawdown at 15%", () => {
    expect(getDrawdownSeverity(15)).toBe("high");
  });

  it("returns high for drawdown above 15%", () => {
    expect(getDrawdownSeverity(20)).toBe("high");
  });
});
