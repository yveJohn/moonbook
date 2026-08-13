import weekImage from '../../assets/membership/moonbook-membership-week.webp';
import monthImage from '../../assets/membership/moonbook-membership-month.webp';
import quarterImage from '../../assets/membership/moonbook-membership-quarter.webp';
import yearImage from '../../assets/membership/moonbook-membership-year.webp';
import permanentImage from '../../assets/membership/moonbook-membership-permanent.webp';

const durationImages = new Map<number, string>([
  [7, weekImage],
  [30, monthImage],
  [90, quarterImage],
  [365, yearImage]
]);

export function membershipImageFor(durationDays?: number | null): string | null {
  if (durationDays == null) {
    return permanentImage;
  }
  return durationImages.get(durationDays) ?? null;
}
