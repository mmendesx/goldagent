"use client";
import * as React from "react";
import { motion } from "motion/react";

export interface FadeProps {
  children: React.ReactNode;
  className?: string;
  delay?: number;
  duration?: number;
  inView?: boolean;
}

export function Fade({
  children,
  className,
  delay = 0,
  duration = 0.2,
  inView = false,
}: FadeProps): React.ReactElement {
  const animationProps = inView
    ? {
        initial: { opacity: 0 },
        whileInView: { opacity: 1 },
        viewport: { once: true },
      }
    : {
        initial: { opacity: 0 },
        animate: { opacity: 1 },
      };

  return (
    <motion.div
      className={className}
      {...animationProps}
      transition={{ duration, delay, ease: [0, 0, 0.2, 1] }}
    >
      {children}
    </motion.div>
  );
}

export interface FadesProps {
  children: React.ReactNode;
  holdDelay?: number;
  className?: string;
  inView?: boolean;
}

export function Fades({
  children,
  holdDelay = 0.05,
  className,
  inView = false,
}: FadesProps): React.ReactElement {
  const items = React.Children.toArray(children);
  return (
    <>
      {items.map((child, index) => (
        <Fade
          key={index}
          className={className}
          delay={index * holdDelay}
          inView={inView}
        >
          {child}
        </Fade>
      ))}
    </>
  );
}
