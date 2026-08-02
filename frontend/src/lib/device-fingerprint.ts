const STORAGE_KEY = "avtotest_device_fp";

/** Stable browser device id for B2B classroom station VIP binding. */
export function getDeviceFingerprint(): string {
  if (typeof window === "undefined") return "";
  try {
    let fp = window.localStorage.getItem(STORAGE_KEY);
    if (!fp) {
      fp =
        typeof crypto !== "undefined" && "randomUUID" in crypto
          ? crypto.randomUUID()
          : `fp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 12)}`;
      window.localStorage.setItem(STORAGE_KEY, fp);
    }
    return fp;
  } catch {
    return "";
  }
}

export const DEVICE_FP_HEADER = "X-Device-Fingerprint";
