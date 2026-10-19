import { describe, expect, it } from 'vitest';
import { normalizeLogs } from '../helpers/helpers';
import { formatUpstream } from '../helpers/upstream';

describe('query log ECH display', () => {
    const address = 'https://dns.example:443/dns-query';

    it.each([
        ['accepted', true, false, `${address} (ECH)`],
        ['ordinary', false, false, address],
        ['historical', undefined, false, address],
        ['cached', true, true, address],
    ])('%s response', (_name, ech, cached, expected) => {
        const [row] = normalizeLogs([
            {
                question: { name: 'example.org', type: 'A' },
                upstream: address,
                upstream_ech: ech,
                cached,
            },
        ]);
        expect(row.upstream).toBe(address);
        expect(formatUpstream(row.upstream, row.upstream_ech, row.cached)).toBe(expected);
    });

    it('does not mark local responses', () => {
        expect(formatUpstream('', true, false)).toBe('');
    });
});
