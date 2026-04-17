import "./Skeleton.css";

export interface SkeletonProps {
  variant?: "text" | "block" | "row";
  width?: string | number;
  height?: string | number;
  className?: string;
}

function toCssValue(value: string | number): string {
  return typeof value === "number" ? `${value}px` : value;
}

export function Skeleton({
  variant = "block",
  width,
  height,
  className,
}: SkeletonProps) {
  const style: React.CSSProperties = {};

  if (width !== undefined) {
    style.width = toCssValue(width);
  }
  if (height !== undefined) {
    style.height = toCssValue(height);
  }

  const classNames = ["skeleton", `skeleton--${variant}`, className]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={classNames}
      style={style}
      aria-hidden="true"
      role="presentation"
    />
  );
}
