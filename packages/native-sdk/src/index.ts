export interface ZotOneShotBytes {
  readonly provider: Uint8Array;
  readonly apiKey: Uint8Array;
  readonly accessToken: Uint8Array;
  readonly accountId: Uint8Array;
  readonly model: Uint8Array;
  readonly systemPrompt: Uint8Array;
  readonly history: Uint8Array;
  readonly message: Uint8Array;
}

// Encodes one turn for `zot-bridge oneshot`. This helper is valid in Native
// SDK's compiled TypeScript subset. Store the history event as model bytes and
// pass it back on the next turn to preserve the conversation.
export interface ZotNativeLine {
  readonly tag: number; // 1 text, 2 event, 3 error, 4 history, 5 done, 0 invalid
  readonly kind: Uint8Array;
  readonly payload: Uint8Array;
}

export function encodeZotOneShot(options: ZotOneShotBytes): Uint8Array {
  const fields = [
    options.provider,
    options.apiKey,
    options.accessToken,
    options.accountId,
    options.model,
    options.systemPrompt,
    options.history,
    options.message,
  ];
  let length = 7;
  for (let index = 0; index < fields.length; index += 1) length += fields[index].length;
  const output = new Uint8Array(length);
  let offset = 0;
  for (let index = 0; index < fields.length; index += 1) {
    output.set(fields[index], offset);
    offset += fields[index].length;
    if (index < fields.length - 1) output[offset++] = 0;
  }
  return output;
}

function hexValue(value: number): number {
  if (value >= 48 && value <= 57) return value - 48;
  if (value >= 97 && value <= 102) return value - 87;
  return -1;
}

function decodeHex(line: Uint8Array, start: number, end: number): Uint8Array {
  if ((end - start) % 2 !== 0) return new Uint8Array(0);
  const output = new Uint8Array((end - start) >> 1);
  for (let input = start, offset = 0; input < end; input += 2, offset += 1) {
    const high = hexValue(line[input]);
    const low = hexValue(line[input + 1]);
    if (high < 0 || low < 0) return new Uint8Array(0);
    output[offset] = high * 16 + low;
  }
  return output;
}

function hasPrefix(line: Uint8Array, prefix: readonly number[]): boolean {
  if (line.length < prefix.length) return false;
  for (let index = 0; index < prefix.length; index += 1) {
    if (line[index] !== prefix[index]) return false;
  }
  return true;
}

export function decodeZotNativeLine(line: Uint8Array): ZotNativeLine {
  const empty = new Uint8Array(0);
  if (line.length === 4 && hasPrefix(line, [100, 111, 110, 101])) return { tag: 5, kind: empty, payload: empty };
  if (hasPrefix(line, [116, 101, 120, 116, 32])) return { tag: 1, kind: empty, payload: decodeHex(line, 5, line.length) };
  if (hasPrefix(line, [101, 114, 114, 111, 114, 32])) return { tag: 3, kind: empty, payload: decodeHex(line, 6, line.length) };
  if (hasPrefix(line, [104, 105, 115, 116, 111, 114, 121, 32])) return { tag: 4, kind: empty, payload: decodeHex(line, 8, line.length) };
  if (hasPrefix(line, [101, 118, 101, 110, 116, 32])) {
    let split = 6;
    while (split < line.length && line[split] !== 32) split += 1;
    if (split < line.length) return { tag: 2, kind: decodeHex(line, 6, split), payload: decodeHex(line, split + 1, line.length) };
  }
  return { tag: 0, kind: empty, payload: empty };
}
