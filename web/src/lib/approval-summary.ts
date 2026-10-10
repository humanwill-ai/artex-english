// Old activity records keep their original text. Parse all shipped formats so
// changing display language cannot disconnect an approval card from its tool.
export function approvalSummary(summary: string): { pendingId: number | null; toolName: string } {
  const id = /\(#(\d+)\)/.exec(summary);
  const tool =
    /Approval requested for tool\s+(\S+)\s+\(#/i.exec(summary) ??
    /도구\s+(\S+)\s+승인/.exec(summary) ??
    /工具\s+(\S+)\s+/.exec(summary);
  return { pendingId: id ? Number.parseInt(id[1], 10) : null, toolName: tool?.[1] ?? summary };
}
