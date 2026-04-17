import { type ReactNode } from "react";
import { useAnimatedNumber } from "../../hooks";

interface MetricItemBaseProps {
  label: string;
  severity?: "medium" | "high";
  badge?: ReactNode;
}

interface MetricItemStaticProps extends MetricItemBaseProps {
  value: string;
  numericValue?: never;
  format?: never;
}

interface MetricItemAnimatedProps extends MetricItemBaseProps {
  value?: never;
  numericValue: number;
  format: (n: number) => string;
}

type MetricItemProps = MetricItemStaticProps | MetricItemAnimatedProps;

function AnimatedValue({
  target,
  format,
  className,
}: {
  target: number;
  format: (n: number) => string;
  className: string;
}) {
  const displayed = useAnimatedNumber(target);
  return <span className={className}>{format(displayed)}</span>;
}

export function MetricItem({ label, severity, badge, ...rest }: MetricItemProps) {
  const valueClassName = [
    "metric-value",
    "mono",
    severity ? `metric-value--${severity}` : "",
  ]
    .filter(Boolean)
    .join(" ");

  const ariaValue =
    "numericValue" in rest && rest.numericValue !== undefined
      ? rest.format!(rest.numericValue)
      : (rest as MetricItemStaticProps).value;

  return (
    <div className="metric-item" aria-label={`${label}: ${ariaValue}`}>
      <span className="metric-label">{label}</span>
      {"numericValue" in rest && rest.numericValue !== undefined ? (
        <AnimatedValue target={rest.numericValue} format={rest.format!} className={valueClassName} />
      ) : (
        <span className={valueClassName}>{(rest as MetricItemStaticProps).value}</span>
      )}
      {badge && <div className="metric-badge">{badge}</div>}
    </div>
  );
}
