const counters = new Map<string, number>();

export function incrementMetric(name: string, value = 1) {
  counters.set(name, (counters.get(name) ?? 0) + value);
}

export function metricsText() {
  return Array.from(counters.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, value]) => `${name} ${value}`)
    .join("\n");
}

export function logInfo(event: string, metadata: Record<string, unknown> = {}) {
  console.log(
    JSON.stringify({
      level: "info",
      event,
      ...metadata,
      at: new Date().toISOString(),
    }),
  );
}
