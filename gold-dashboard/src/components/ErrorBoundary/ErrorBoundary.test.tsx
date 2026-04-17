// Environment: vitest with environment: 'node' and no @testing-library/react.
// We test the class component directly — instantiating it and exercising its
// static and instance methods without a renderer.

import { describe, it, expect, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";

// ---------------------------------------------------------------------------
// getDerivedStateFromError — static lifecycle method
// ---------------------------------------------------------------------------

describe("ErrorBoundary.getDerivedStateFromError", () => {
  it("returns state with the thrown error", () => {
    const error = new Error("something exploded");
    const state = ErrorBoundary.getDerivedStateFromError(error);
    expect(state.error).toBe(error);
  });

  it("preserves error identity across call", () => {
    const error = new TypeError("type mismatch");
    const state = ErrorBoundary.getDerivedStateFromError(error);
    expect(state.error).toBeInstanceOf(TypeError);
    expect(state.error?.message).toBe("type mismatch");
  });
});

// ---------------------------------------------------------------------------
// reset() — clears error state and calls onReset callback
// ---------------------------------------------------------------------------

describe("ErrorBoundary reset()", () => {
  it("calls onReset prop when reset is invoked", () => {
    const onReset = vi.fn();
    // Construct the instance with minimal props (no children in node env).
    const boundary = new ErrorBoundary({ children: null, onReset });
    // Simulate being in error state.
    boundary.state = { error: new Error("boom") };

    boundary.reset();

    expect(onReset).toHaveBeenCalledOnce();
  });

  it("sets error state to null when reset is invoked", () => {
    const boundary = new ErrorBoundary({ children: null });
    boundary.state = { error: new Error("boom") };

    const setStateSpy = vi.spyOn(boundary, "setState");
    boundary.reset();

    expect(setStateSpy).toHaveBeenCalledWith({ error: null });
  });

  it("does not throw when onReset prop is absent", () => {
    const boundary = new ErrorBoundary({ children: null });
    boundary.state = { error: new Error("boom") };
    expect(() => boundary.reset()).not.toThrow();
  });
});

// ---------------------------------------------------------------------------
// componentDidUpdate — resetKeys change triggers a reset when in error state
// ---------------------------------------------------------------------------

describe("ErrorBoundary componentDidUpdate", () => {
  it("resets when resetKeys change while in error state", () => {
    const onReset = vi.fn();
    const boundary = new ErrorBoundary({
      children: null,
      onReset,
      resetKeys: [2],
    });
    boundary.state = { error: new Error("stale error") };

    const setStateSpy = vi.spyOn(boundary, "setState");
    // Simulate previous props had different resetKeys.
    boundary.componentDidUpdate({ children: null, resetKeys: [1] });

    expect(setStateSpy).toHaveBeenCalledWith({ error: null });
    expect(onReset).toHaveBeenCalledOnce();
  });

  it("does not reset when resetKeys are the same reference", () => {
    const resetKeys = [1, 2];
    const boundary = new ErrorBoundary({
      children: null,
      resetKeys,
    });
    boundary.state = { error: new Error("stale error") };

    const setStateSpy = vi.spyOn(boundary, "setState");
    // Same resetKeys reference → prevProps.resetKeys === this.props.resetKeys
    boundary.componentDidUpdate({ children: null, resetKeys });

    expect(setStateSpy).not.toHaveBeenCalled();
  });

  it("does not reset when there is no current error", () => {
    const boundary = new ErrorBoundary({
      children: null,
      resetKeys: [2],
    });
    boundary.state = { error: null };

    const setStateSpy = vi.spyOn(boundary, "setState");
    boundary.componentDidUpdate({ children: null, resetKeys: [1] });

    expect(setStateSpy).not.toHaveBeenCalled();
  });

  it("does not reset when resetKeys values are unchanged across update", () => {
    const boundary = new ErrorBoundary({
      children: null,
      resetKeys: [1, 2],
    });
    boundary.state = { error: new Error("error") };

    const setStateSpy = vi.spyOn(boundary, "setState");
    // Different array reference but same element values.
    boundary.componentDidUpdate({ children: null, resetKeys: [1, 2] });

    expect(setStateSpy).not.toHaveBeenCalled();
  });
});
