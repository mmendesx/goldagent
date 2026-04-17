"use client";
import * as React from "react";
import { motion, AnimatePresence } from "motion/react";

export interface SlidingNumberProps {
  number: number;
  decimalPlaces?: number;
  thousandSeparator?: string;
  decimalSeparator?: string;
  className?: string;
  delay?: number;
  padStart?: boolean;
}

// Digits 0-9 stacked in a column; we slide to the current digit by translating Y.
function DigitColumn({
  digit,
  delay = 0,
}: {
  digit: string;
  delay?: number;
}): React.ReactElement {
  const DIGITS = ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9"];
  const index = DIGITS.indexOf(digit);

  // Non-numeric characters (separators, minus) are rendered as static spans
  if (index === -1) {
    return <span>{digit}</span>;
  }

  return (
    <span
      aria-hidden="true"
      style={{
        display: "inline-block",
        overflow: "hidden",
        height: "1em",
        verticalAlign: "top",
      }}
    >
      <motion.span
        style={{ display: "flex", flexDirection: "column" }}
        animate={{ y: `${-index}em` }}
        transition={{ type: "spring", bounce: 0.15, duration: 0.4, delay }}
      >
        {DIGITS.map((d) => (
          <span key={d} style={{ height: "1em", display: "block", lineHeight: 1 }}>
            {d}
          </span>
        ))}
      </motion.span>
    </span>
  );
}

function formatNumber(
  value: number,
  decimalPlaces: number,
  thousandSeparator: string,
  decimalSeparator: string,
  padStart: boolean,
): string {
  const isNegative = value < 0;
  const absValue = Math.abs(value);

  const [intPart, rawDecimalPart] = absValue.toFixed(decimalPlaces).split(".");

  // Apply thousand separator
  let formattedInt = intPart;
  if (thousandSeparator) {
    formattedInt = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, thousandSeparator);
  }

  // Pad integer part with leading zeros when requested
  if (padStart && !thousandSeparator) {
    formattedInt = formattedInt.padStart(2, "0");
  }

  let result = formattedInt;
  if (decimalPlaces > 0 && rawDecimalPart !== undefined) {
    result += decimalSeparator + rawDecimalPart;
  }

  return isNegative ? `-${result}` : result;
}

export function SlidingNumber({
  number,
  decimalPlaces = 0,
  thousandSeparator = ",",
  decimalSeparator = ".",
  className,
  delay = 0,
  padStart = false,
}: SlidingNumberProps): React.ReactElement {
  const formatted = formatNumber(
    number,
    decimalPlaces,
    thousandSeparator,
    decimalSeparator,
    padStart,
  );

  // Split into individual characters; separators are static, digits animate
  const chars = formatted.split("");

  return (
    <span
      className={className}
      aria-label={formatted}
      style={{ display: "inline-flex", alignItems: "baseline" }}
    >
      <AnimatePresence mode="popLayout" initial={false}>
        {chars.map((char, index) => {
          const isDigit = /\d/.test(char);
          const key = `${index}-${char}`;

          if (!isDigit) {
            return (
              <motion.span
                key={key}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12 }}
              >
                {char}
              </motion.span>
            );
          }

          return (
            <motion.span
              key={key}
              layout
              initial={{ opacity: 0, y: "-0.5em" }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: "0.5em" }}
              transition={{ type: "spring", bounce: 0.15, duration: 0.4, delay }}
              style={{ display: "inline-block" }}
            >
              <DigitColumn digit={char} delay={delay} />
            </motion.span>
          );
        })}
      </AnimatePresence>
    </span>
  );
}
