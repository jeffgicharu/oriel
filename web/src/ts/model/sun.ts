// Where the sun is in the sky, from the NOAA solar calculator. Accurate to
// well under a degree, which is plenty for shadows on a building.

export interface SunPosition {
  /** Degrees clockwise from north. */
  azimuth: number;
  /** Degrees above the horizon; negative at night. */
  altitude: number;
}

const rad = Math.PI / 180;

export function sunPosition(date: Date, lat: number, lon: number): SunPosition {
  const jd = date.getTime() / 86400000 + 2440587.5;
  const jc = (jd - 2451545) / 36525;
  const meanLong = (280.46646 + jc * (36000.76983 + jc * 0.0003032)) % 360;
  const meanAnom = 357.52911 + jc * (35999.05029 - 0.0001537 * jc);
  const ecc = 0.016708634 - jc * (0.000042037 + 0.0000001267 * jc);
  const centre =
    Math.sin(rad * meanAnom) * (1.914602 - jc * (0.004817 + 0.000014 * jc)) +
    Math.sin(rad * 2 * meanAnom) * (0.019993 - 0.000101 * jc) +
    Math.sin(rad * 3 * meanAnom) * 0.000289;
  const trueLong = meanLong + centre;
  const omega = 125.04 - 1934.136 * jc;
  const apparentLong = trueLong - 0.00569 - 0.00478 * Math.sin(rad * omega);
  const meanObliquity = 23 + (26 + (21.448 - jc * (46.815 + jc * (0.00059 - jc * 0.001813))) / 60) / 60;
  const obliquity = meanObliquity + 0.00256 * Math.cos(rad * omega);
  const declination = Math.asin(Math.sin(rad * obliquity) * Math.sin(rad * apparentLong)) / rad;
  const y = Math.tan((rad * obliquity) / 2) ** 2;
  const equationOfTime =
    (4 / rad) *
    (y * Math.sin(2 * rad * meanLong) -
      2 * ecc * Math.sin(rad * meanAnom) +
      4 * ecc * y * Math.sin(rad * meanAnom) * Math.cos(2 * rad * meanLong) -
      0.5 * y * y * Math.sin(4 * rad * meanLong) -
      1.25 * ecc * ecc * Math.sin(2 * rad * meanAnom));
  const minutes = date.getUTCHours() * 60 + date.getUTCMinutes() + date.getUTCSeconds() / 60;
  let solarTime = (minutes + equationOfTime + 4 * lon) % 1440;
  if (solarTime < 0) solarTime += 1440;
  const hourAngle = solarTime / 4 < 0 ? solarTime / 4 + 180 : solarTime / 4 - 180;
  const cosZenith =
    Math.sin(rad * lat) * Math.sin(rad * declination) +
    Math.cos(rad * lat) * Math.cos(rad * declination) * Math.cos(rad * hourAngle);
  const zenith = Math.acos(Math.max(-1, Math.min(1, cosZenith))) / rad;
  const denom = Math.cos(rad * lat) * Math.sin(rad * zenith);
  let azimuth = 0;
  if (Math.abs(denom) > 1e-6) {
    const a = Math.max(-1, Math.min(1, (Math.sin(rad * lat) * Math.cos(rad * zenith) - Math.sin(rad * declination)) / denom));
    const base = Math.acos(a) / rad;
    azimuth = hourAngle > 0 ? (base + 180) % 360 : (540 - base) % 360;
  }
  return { azimuth, altitude: 90 - zenith };
}

/** A unit vector pointing at the sun, in the model's axes (x east, y up, z south). */
export function sunDirection(p: SunPosition): [number, number, number] {
  const az = p.azimuth * rad;
  const alt = p.altitude * rad;
  return [Math.sin(az) * Math.cos(alt), Math.sin(alt), -Math.cos(az) * Math.cos(alt)];
}

/** A moment today in Nairobi (UTC+3, no daylight saving). */
export function nairobiTime(hours: number, minutes: number, base = new Date()): Date {
  const local = new Date(base.getTime() + 3 * 3600_000);
  return new Date(Date.UTC(local.getUTCFullYear(), local.getUTCMonth(), local.getUTCDate(), hours - 3, minutes));
}
