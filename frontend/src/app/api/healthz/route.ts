export const dynamic = "force-dynamic";
export const runtime = "nodejs";

/** Cheap liveness for the web container. Do not SSR a locale document here. */
export function GET() {
  return new Response("ok", {
    status: 200,
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}
