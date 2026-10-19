/** Format the recorded upstream without changing its address in API data. */
export const formatUpstream = (upstream: string, echAccepted?: boolean, cached?: boolean): string =>
    upstream && echAccepted === true && !cached ? `${upstream} (ECH)` : upstream;
