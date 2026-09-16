/**
 * Pure copy-text builder for the assistant "copy answer" action
 * (Vue botmsg.vue handleCopyAnswer). The raw content is what lands on the
 * clipboard; thinking blocks are stripped so only the visible answer copies.
 */

const THINK_BLOCK = new RegExp("<think>[\\s\\S]*?</think>", "gi")
const OPEN_THINK = new RegExp("<think>[\\s\\S]*$", "i")

export function copyAnswerText(content: string | undefined | null): string {
  if (typeof content !== "string") return ""
  return content.replace(THINK_BLOCK, "").replace(OPEN_THINK, "").trim()
}
