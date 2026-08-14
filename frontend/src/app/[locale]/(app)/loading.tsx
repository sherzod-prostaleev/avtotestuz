export default function AppLoading() {
  return (
    <div
      className="flex min-h-[50vh] items-center justify-center"
      role="status"
      aria-live="polite"
    >
      <div
        className="h-8 w-8 animate-spin rounded-full border-2 border-accent border-t-transparent"
        aria-hidden="true"
      />
    </div>
  );
}
