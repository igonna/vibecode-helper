import { InfoPage } from "@/components/info-page";

export default function PrivacyPage() {
  return (
    <InfoPage title="Privacy">
      <p>
        Normal event upload is metadata-only: prompt timing, status, adapter
        name, project id, and delivery state.
      </p>
      <p>
        Vibeengine does not upload prompt text, assistant responses, command
        logs, terminal output, environment variables, tokens, repo metadata, file
        paths, or source snippets in normal events.
      </p>
      <p>
        Command proposal text is stored only for local review and approval. It is
        not used as analytics data or exported in normal event summaries.
      </p>
      <p>
        Account export is one short-lived ZIP with account metadata, project
        records, and CSV prompt event summaries. It excludes prompt text,
        responses, command text, env vars, tokens, file paths, repo metadata,
        logs, and raw database dumps.
      </p>
      <p>
        Local engine retention keeps delivered diagnostics briefly and keeps
        undelivered events within the documented local queue cap so hooks stay
        reliable during outages.
      </p>
    </InfoPage>
  );
}
