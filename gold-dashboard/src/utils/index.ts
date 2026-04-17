export function formatCurrency(value: string | number, decimals = 2): string {
  const numeric = typeof value === "string" ? parseFloat(value) : value;
  if (Number.isNaN(numeric)) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(numeric);
}

export function formatPercent(value: string | number, decimals = 2): string {
  const numeric = typeof value === "string" ? parseFloat(value) : value;
  if (Number.isNaN(numeric)) return "—";
  return `${numeric.toFixed(decimals)}%`;
}

export function formatCompact(value: string | number): string {
  const numeric = typeof value === "string" ? parseFloat(value) : value;
  if (Number.isNaN(numeric)) return "—";
  return new Intl.NumberFormat("en-US", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(numeric);
}

export function formatPrice(value: string | number): string {
  const numeric = typeof value === "string" ? parseFloat(value) : value;
  if (Number.isNaN(numeric)) return "—";
  if (numeric >= 1000)
    return numeric.toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  if (numeric >= 1)
    return numeric.toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 4,
    });
  return numeric.toLocaleString("en-US", {
    minimumFractionDigits: 4,
    maximumFractionDigits: 8,
  });
}

export function getDrawdownSeverity(percent: number): "medium" | "high" | undefined {
  if (percent >= 15) return "high";
  if (percent >= 10) return "medium";
  return undefined;
}
