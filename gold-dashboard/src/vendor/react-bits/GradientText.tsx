"use client";
import * as React from "react";

export interface GradientTextProps {
  children: React.ReactNode;
  colors?: string[];
  animationSpeed?: number;
  direction?: "horizontal" | "vertical" | "diagonal";
  showBorder?: boolean;
  pauseOnHover?: boolean;
  className?: string;
}

const STYLE_ID = "rb-gradient-text-keyframes";

function injectStyles(): void {
  if (typeof document === "undefined") return;
  if (document.getElementById(STYLE_ID)) return;

  const style = document.createElement("style");
  style.id = STYLE_ID;
  style.textContent = `
    @keyframes rb-gradient-shift {
      0% { background-position: 0% 50%; }
      50% { background-position: 100% 50%; }
      100% { background-position: 0% 50%; }
    }
    .rb-gradient-text {
      background-clip: text;
      -webkit-background-clip: text;
      color: transparent;
      background-size: 300% 300%;
      animation: rb-gradient-shift var(--rb-anim-speed, 6s) linear infinite;
    }
    .rb-gradient-text--pause-on-hover:hover {
      animation-play-state: paused;
    }
    @media (prefers-reduced-motion: reduce) {
      .rb-gradient-text {
        animation: none;
        background-position: 0% 50%;
      }
    }
  `;
  document.head.appendChild(style);
}

function buildGradient(colors: string[], direction: GradientTextProps["direction"]): string {
  const stops = colors.join(", ");
  switch (direction) {
    case "vertical":
      return `linear-gradient(to bottom, ${stops})`;
    case "diagonal":
      return `linear-gradient(135deg, ${stops})`;
    default:
      return `linear-gradient(to right, ${stops})`;
  }
}

export function GradientText({
  children,
  colors = ["#f0b429", "#ffd700", "#f0b429"],
  animationSpeed = 6,
  direction = "horizontal",
  showBorder = false,
  pauseOnHover = false,
  className,
}: GradientTextProps): React.ReactElement {
  React.useEffect(() => {
    injectStyles();
  }, []);

  const gradient = buildGradient(colors, direction);

  const classNames = [
    "rb-gradient-text",
    pauseOnHover ? "rb-gradient-text--pause-on-hover" : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");

  const textStyle: React.CSSProperties = {
    backgroundImage: gradient,
    ["--rb-anim-speed" as string]: `${animationSpeed}s`,
  };

  const textSpan = (
    <span className={classNames} style={textStyle}>
      {children}
    </span>
  );

  if (!showBorder) return textSpan;

  const borderStyle: React.CSSProperties = {
    padding: "0.125em 0.5em",
    border: "1px solid transparent",
    backgroundImage: `${gradient}, ${gradient}`,
    backgroundOrigin: "border-box",
    backgroundClip: "padding-box, border-box",
    borderRadius: "0.25em",
    display: "inline-block",
  };

  return <span style={borderStyle}>{textSpan}</span>;
}

export default GradientText;
