import { InfoPage } from "@/components/info-page";

export default function TermsPage() {
  return (
    <InfoPage title="Terms">
      <p>
        Vibeengine is a developer tool for local prompt status, project status,
        and optional command automation. Users are responsible for commands they
        configure and locally approve.
      </p>
      <p>
        Web-saved shell command changes do not run automatically as executable
        code. Risky changes require local approval through the engine.
      </p>
      <p>
        Trial access lasts 14 days. After trial expiration, cloud upload and the
        hosted dashboard pause until subscription is active.
      </p>
    </InfoPage>
  );
}
