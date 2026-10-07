import { CheckoutDone, doneMetadata } from "./checkout-done";

export const metadata = doneMetadata;

// No bot in the path (not configured server-side): text only.
export default function CheckoutDonePage() {
  return <CheckoutDone bot={null} />;
}
