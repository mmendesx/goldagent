interface MetricItemProps {
  label: string;
  value: string;
  severity?: "low" | "medium" | "high";
}

export function MetricItem({ label, value, severity }: MetricItemProps) {
  const valueClassName = severity ? `metric-value metric-value--${severity}` : "metric-value";
  return (
    <div className="metric-item">
      <span className="metric-label">{label}</span>
      <span className={valueClassName}>{value}</span>
    </div>
  );
}
