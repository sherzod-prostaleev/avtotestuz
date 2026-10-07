import { CheckoutDone, botFrom, doneMetadata } from "../checkout-done";

export const metadata = doneMetadata;

// The bot username rides in the path, not a query string, because Payme embeds
// the return URL raw in a ';'-joined key=value payload. An invalid segment
// renders text only: a payer coming back from a payment must never see a 404.
export default async function CheckoutDoneBotPage({ params }: { params: Promise<{ bot: string }> }) {
  const { bot } = await params;
  return <CheckoutDone bot={botFrom(bot)} />;
}
