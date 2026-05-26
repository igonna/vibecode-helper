import { readFile, stat } from "node:fs/promises";
import path from "node:path";

type ReleaseContext = {
  params: Promise<{ artifact: string }>;
};

const artifactName =
  /^(vibeengine-(linux|darwin|windows)-(amd64|arm64)(\.exe)?(\.sha256|\.sig)?|release-signing-public\.pem)$/;

export async function GET(_request: Request, context: ReleaseContext) {
  const { artifact } = await context.params;
  if (!artifactName.test(artifact)) {
    return Response.json(
      { error: { code: "artifact_not_found", message: "Release not found." } },
      { status: 404 },
    );
  }

  const artifactPath =
    artifact === "release-signing-public.pem"
      ? path.join(process.cwd(), "release-public", artifact)
      : path.join(process.cwd(), "releases", artifact);

  try {
    const artifactStat = await stat(artifactPath);
    if (!artifactStat.isFile()) {
      throw new Error("not a file");
    }

    const artifactBody = await readFile(artifactPath);
    return new Response(artifactBody, {
      headers: {
        "content-type": contentTypeForArtifact(artifact),
        "content-length": String(artifactStat.size),
        "cache-control": "no-store",
      },
    });
  } catch {
    return Response.json(
      {
        error: {
          code: "artifact_not_available",
          message: "Engine artifact is not available for this platform yet.",
        },
      },
      { status: 404 },
    );
  }
}

function contentTypeForArtifact(artifact: string) {
  if (artifact.endsWith(".sha256") || artifact.endsWith(".pem")) {
    return "text/plain; charset=utf-8";
  }
  return "application/octet-stream";
}
