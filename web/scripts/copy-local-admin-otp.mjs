import { createHmac } from "node:crypto";
import { chmod, readFile, rename, rm, stat, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";

const credentialPath = path.join(homedir(), ".local/share/bidora/integration/local-platform-admin.env");
const watch = process.argv.includes("--watch");

function decodeBase32(encoded) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = 0;
  let value = 0;
  const bytes = [];
  for (const character of encoded) {
    const digit = alphabet.indexOf(character);
    if (digit < 0) throw new Error("The local administrator TOTP secret is invalid.");
    value = (value << 5) | digit;
    bits += 5;
    if (bits >= 8) {
      bits -= 8;
      bytes.push((value >>> bits) & 0xff);
    }
  }
  return Buffer.from(bytes);
}

function currentCode(secret) {
  const counter = Math.floor(Date.now() / 1000 / 30);
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac("sha1", secret).update(message).digest();
  const offset = digest[digest.length - 1] & 0x0f;
  return String((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).padStart(6, "0");
}

async function loadPrivateFile() {
  const metadata = await stat(credentialPath);
  if ((metadata.mode & 0o077) !== 0 || metadata.uid !== process.getuid?.()) {
    throw new Error("Local administrator credentials must be owned by the current user with mode 0600.");
  }
  return readFile(credentialPath, "utf8");
}

function readSecret(source) {
  const match = source.match(/^export BIDORA_LOCAL_PLATFORM_ADMIN_TOTP_SECRET=(.+)$/m);
  if (!match) throw new Error("The local administrator TOTP secret is unavailable.");
  let encoded;
  try {
    encoded = JSON.parse(match[1]);
  } catch {
    throw new Error("The local administrator TOTP secret has an unsupported encoding.");
  }
  if (typeof encoded !== "string" || !/^[A-Z2-7]{32}$/.test(encoded)) throw new Error("The local administrator TOTP secret is invalid.");
  return decodeBase32(encoded);
}

async function updatePrivateOTP(source, code) {
  const line = `export BIDORA_LOCAL_OTP=${JSON.stringify(code)}`;
  const next = /^export BIDORA_LOCAL_OTP=.*$/m.test(source)
    ? source.replace(/^export BIDORA_LOCAL_OTP=.*$/m, line)
    : `${source.replace(/\n?$/, "\n")}${line}\n`;
  const temporary = `${credentialPath}.otp-${process.pid}.tmp`;
  try {
    await writeFile(temporary, next, { mode: 0o600, flag: "wx" });
    await chmod(temporary, 0o600);
    await rename(temporary, credentialPath);
  } finally {
    await rm(temporary, { force: true });
  }
}

async function publish({ copy }) {
  const source = await loadPrivateFile();
  const secret = readSecret(source);
  const code = currentCode(secret);
  secret.fill(0);
  await updatePrivateOTP(source, code);
  if (copy) {
    const copied = spawnSync("/usr/bin/pbcopy", [], { input: code, encoding: "utf8", stdio: ["pipe", "ignore", "inherit"] });
    if (copied.status !== 0) throw new Error("Could not copy the local verification code.");
  }
}

if (watch) {
  console.log("Synchronizing BIDORA_LOCAL_OTP every 30 seconds. Press Ctrl+C to stop.");
  for (;;) {
    await publish({ copy: false });
    const milliseconds = Date.now();
    const delay = 30_250 - (milliseconds % 30_000);
    await new Promise((resolve) => setTimeout(resolve, delay));
  }
} else {
  const remaining = 30 - (Math.floor(Date.now() / 1000) % 30);
  if (remaining < 8) await new Promise((resolve) => setTimeout(resolve, (remaining + 1) * 1000));
  await publish({ copy: true });
  console.log("Fresh local Platform Owner OTP saved to BIDORA_LOCAL_OTP and copied. Use it once before it changes.");
}
