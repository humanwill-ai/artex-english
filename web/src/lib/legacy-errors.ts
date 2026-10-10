// Existing deployments and saved mock records may still return older messages.
// Keep these compatibility checks separate from translated display strings.
export function isAlreadyExistsError(message: string): boolean {
  return /already exists/i.test(message) || message.includes("이미 존재") || message.includes("已存在");
}

export function isArchiveNotFoundError(message: string): boolean {
  return (
    /^Archive not found$/i.test(message.trim()) ||
    message.includes("보관을 찾을 수 없습니다") ||
    message.includes("归档不存在")
  );
}
