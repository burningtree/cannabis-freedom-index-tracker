// What a run cost, worked out from the tokens it used and the model's published price. OpenCode
// reports no cost for these models, so the request page does the sum itself. Prices are US dollars
// per million tokens, from https://opencode.ai/docs/console/go: DeepSeek V4.1 Flash costs half as
// much outside its peak hours (01:00-04:00 and 06:00-10:00 UTC, Monday to Friday).
export const MODEL_PRICES = {
  "opencode-go/deepseek-v4.1-flash": { input: 0.15, output: 0.6, peakInput: 0.3, peakOutput: 1.2 },
};

export function isPeak(seconds) {
  const date = new Date((Number(seconds) || 0) * 1000);
  if (Number.isNaN(date.getTime())) return false;
  const day = date.getUTCDay(), hour = date.getUTCHours();
  return day >= 1 && day <= 5 && ((hour >= 1 && hour < 4) || (hour >= 6 && hour < 10));
}

// the run's cost in USD: what OpenCode reported if it did, otherwise its tokens at the price for
// the hour it started; 0 when neither is known
export function runCost(run) {
  const reported = Number(run?.cost_usd || 0);
  if (reported > 0) return reported;
  const price = MODEL_PRICES[run?.model];
  if (!price) return 0;
  const peak = isPeak(run.started_at || run.created_at);
  const input = peak ? price.peakInput : price.input;
  const output = peak ? price.peakOutput : price.output;
  return (Number(run?.input_tokens || 0) * input + Number(run?.output_tokens || 0) * output) / 1e6;
}

// true when the cost shown is worked out from tokens rather than reported by OpenCode
export function costEstimated(run) {
  return !(Number(run?.cost_usd || 0) > 0) && Boolean(MODEL_PRICES[run?.model]);
}

// A run costs a fraction of a cent, so keep four decimals ($0.0001) rather than rounding to cents;
// the smallest runs get six so they do not show as $0.0000. 0 (unknown) is shown as a dash.
export function formatUSD(value) {
  const v = Number(value) || 0;
  if (!v) return "—";
  return `$${v.toFixed(v < 0.0001 ? 6 : 4)}`;
}
