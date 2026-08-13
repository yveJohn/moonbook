import { describe, expect, it } from 'vitest';
import { membershipImageFor } from './membershipImages';

const membershipAssets = import.meta.glob('../../assets/membership/*', {
  eager: true,
  import: 'default',
  query: '?inline'
}) as Record<string, string>;

function dataUrlByteLength(dataUrl: string) {
  const base64 = dataUrl.slice(dataUrl.indexOf(',') + 1);
  const padding = base64.endsWith('==') ? 2 : base64.endsWith('=') ? 1 : 0;
  return (base64.length * 3) / 4 - padding;
}

describe('membershipImageFor', () => {
  it.each([
    [7, 'moonbook-membership-week.webp'],
    [30, 'moonbook-membership-month.webp'],
    [90, 'moonbook-membership-quarter.webp'],
    [365, 'moonbook-membership-year.webp']
  ])('maps %s days to %s', (days, fileName) => {
    expect(membershipImageFor(days)).toContain(fileName);
  });

  it('maps a nullish duration to permanent and unknown duration to no image', () => {
    expect(membershipImageFor(null)).toContain('moonbook-membership-permanent.webp');
    expect(membershipImageFor(undefined)).toContain('moonbook-membership-permanent.webp');
    expect(membershipImageFor(180)).toBeNull();
  });

  it('keeps exactly five mapped WebP assets below the total size budget', () => {
    const expectedFiles = [
      'moonbook-membership-month.webp',
      'moonbook-membership-permanent.webp',
      'moonbook-membership-quarter.webp',
      'moonbook-membership-week.webp',
      'moonbook-membership-year.webp'
    ];
    const assetFiles = Object.keys(membershipAssets)
      .map((assetPath) => assetPath.split('/').at(-1))
      .sort();
    const assetDataUrls = Object.values(membershipAssets);
    const totalBytes = assetDataUrls
      .reduce((sum, dataUrl) => sum + dataUrlByteLength(dataUrl), 0);

    expect(assetFiles).toEqual(expectedFiles);
    expect(assetDataUrls).toHaveLength(expectedFiles.length);
    for (const dataUrl of assetDataUrls) {
      expect(dataUrl).toMatch(/^data:image\/webp;base64,/);
      const header = atob(dataUrl.slice(dataUrl.indexOf(',') + 1)).slice(0, 12);
      expect(header.slice(0, 4)).toBe('RIFF');
      expect(header.slice(8, 12)).toBe('WEBP');
    }
    expect(totalBytes).toBeLessThan(1_500_000);
  });
});
