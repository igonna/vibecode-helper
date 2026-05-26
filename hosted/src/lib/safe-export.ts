import { randomUUID } from "node:crypto";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

export const exportTtlMs = 15 * 60 * 1000;

export function exportRoot() {
  return path.join(os.tmpdir(), "vibeengine-exports");
}

export function newExportId() {
  return randomUUID();
}

export function exportPath(exportId: string) {
  return path.join(exportRoot(), `${exportId}.zip`);
}

export function exportMetaPath(exportId: string) {
  return path.join(exportRoot(), `${exportId}.json`);
}

export async function writeSafeExportZip(
  exportId: string,
  accountId: string,
  files: { name: string; body: string }[],
) {
  await mkdir(exportRoot(), { recursive: true, mode: 0o700 });
  await writeFile(exportPath(exportId), createZip(files), { mode: 0o600 });
  await writeFile(
    exportMetaPath(exportId),
    JSON.stringify({ account_id: accountId, created_at: new Date().toISOString() }),
    { mode: 0o600 },
  );
}

export async function readSafeExportZip(exportId: string, accountId: string) {
  if (!/^[0-9a-f-]{36}$/.test(exportId)) {
    return null;
  }

  const file = exportPath(exportId);
  const metaFile = exportMetaPath(exportId);
  const info = await stat(file).catch(() => null);
  if (!info || Date.now() - info.mtimeMs > exportTtlMs) {
    return null;
  }
  const meta = await readFile(metaFile, "utf8")
    .then((value) => JSON.parse(value) as { account_id?: string })
    .catch(() => null);
  if (meta?.account_id !== accountId) {
    return null;
  }

  return readFile(file);
}

export function csvEscape(value: unknown) {
  const text = value === null || value === undefined ? "" : String(value);
  return `"${text.replaceAll('"', '""')}"`;
}

function createZip(files: { name: string; body: string }[]) {
  const localParts: Buffer[] = [];
  const centralParts: Buffer[] = [];
  let offset = 0;

  for (const file of files) {
    const name = Buffer.from(file.name, "utf8");
    const body = Buffer.from(file.body, "utf8");
    const crc = crc32(body);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(0, 6);
    local.writeUInt16LE(0, 8);
    local.writeUInt16LE(0, 10);
    local.writeUInt16LE(0, 12);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(body.length, 18);
    local.writeUInt32LE(body.length, 22);
    local.writeUInt16LE(name.length, 26);
    local.writeUInt16LE(0, 28);
    localParts.push(local, name, body);

    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE(20, 4);
    central.writeUInt16LE(20, 6);
    central.writeUInt16LE(0, 8);
    central.writeUInt16LE(0, 10);
    central.writeUInt16LE(0, 12);
    central.writeUInt16LE(0, 14);
    central.writeUInt32LE(crc, 16);
    central.writeUInt32LE(body.length, 20);
    central.writeUInt32LE(body.length, 24);
    central.writeUInt16LE(name.length, 28);
    central.writeUInt16LE(0, 30);
    central.writeUInt16LE(0, 32);
    central.writeUInt16LE(0, 34);
    central.writeUInt16LE(0, 36);
    central.writeUInt32LE(0, 38);
    central.writeUInt32LE(offset, 42);
    centralParts.push(central, name);

    offset += local.length + name.length + body.length;
  }

  const centralSize = centralParts.reduce((sum, part) => sum + part.length, 0);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(0, 4);
  end.writeUInt16LE(0, 6);
  end.writeUInt16LE(files.length, 8);
  end.writeUInt16LE(files.length, 10);
  end.writeUInt32LE(centralSize, 12);
  end.writeUInt32LE(offset, 16);
  end.writeUInt16LE(0, 20);

  return Buffer.concat([...localParts, ...centralParts, end]);
}

const crcTable = new Uint32Array(256).map((_, index) => {
  let value = index;
  for (let bit = 0; bit < 8; bit += 1) {
    value = value & 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1;
  }
  return value >>> 0;
});

function crc32(body: Buffer) {
  let crc = 0xffffffff;
  for (const byte of body) {
    crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}
