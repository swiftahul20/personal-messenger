export const BOT_USERNAME = "CyberKid00";

export function isBotUsername(value: string): boolean {
  return value.trim().toLowerCase() === BOT_USERNAME.toLowerCase();
}
