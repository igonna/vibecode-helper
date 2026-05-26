import { createHash, createSign } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

const releasesDir = path.join(process.cwd(), "releases");
const privateKeyPath =
  process.env.RELEASE_SIGNING_PRIVATE_KEY_PATH ??
  path.join(os.homedir(), ".config", "vibeengine", "release-signing-private.pem");

async function main() {
  const privateKey = await readFile(privateKeyPath, "utf8");
  const files = await readdir(releasesDir);
  const artifacts = files.filter((file) =>
    /^vibeengine-(linux|darwin|windows)-(amd64|arm64)(\.exe)?$/.test(file),
  );

  for (const artifact of artifacts) {
    const artifactPath = path.join(releasesDir, artifact);
    const body = await readFile(artifactPath);
    const sha256 = createHash("sha256").update(body).digest("hex");
    const signer = createSign("RSA-SHA256");
    signer.update(body);
    signer.end();
    const signature = signer.sign(privateKey);

    await writeFile(`${artifactPath}.sha256`, `${sha256}  ${artifact}\n`, {
      mode: 0o644,
    });
    await writeFile(`${artifactPath}.sig`, signature, { mode: 0o644 });
    console.log(`signed ${artifact}`);
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
