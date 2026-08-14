type MessageTree = Record<string, unknown>;

/** Copy only the listed next-intl namespaces onto the client provider payload. */
export function pickMessages(messages: unknown, namespaces: readonly string[]): MessageTree {
  const source = messages && typeof messages === "object" ? (messages as MessageTree) : {};
  const picked: MessageTree = {};
  for (const namespace of namespaces) {
    if (namespace in source) {
      picked[namespace] = source[namespace];
    }
  }
  return picked;
}
