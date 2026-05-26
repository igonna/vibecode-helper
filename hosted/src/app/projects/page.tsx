"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Bell,
  Copy,
  Pause,
  Play,
  Plus,
  RotateCcw,
  Search,
  Terminal,
  Trash2,
  User,
  X,
} from "lucide-react";

type Project = {
  id: string;
  name: string;
  lastStatus: string | null;
  displayStatus: string;
};

type DashboardStats = {
  prompts: number;
  successes: number;
  failures: number;
  attention: number;
  promptTimeMs: number;
};

type Subscription = {
  state: string;
  effectiveState: string;
  plan: string;
  priceRub: number;
  trialEndsAt: string | null;
  currentPeriodEnd: string | null;
};

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [stats, setStats] = useState<DashboardStats>({
    prompts: 0,
    successes: 0,
    failures: 0,
    attention: 0,
    promptTimeMs: 0,
  });
  const [query, setQuery] = useState("");
  const [newProjectName, setNewProjectName] = useState("");
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isAccountOpen, setIsAccountOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isCreatingProject, setIsCreatingProject] = useState(false);
  const [installCommand, setInstallCommand] = useState<string | null>(null);
  const [installExpiresAt, setInstallExpiresAt] = useState<string | null>(null);
  const [isInstallLoading, setIsInstallLoading] = useState(false);
  const [isSoundEnabled, setIsSoundEnabled] = useState(false);
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [commandText, setCommandText] = useState("");
  const [healthCommand, setHealthCommand] = useState("");
  const [pauseDuringPrompt, setPauseDuringPrompt] = useState(false);
  const [autoPauseAfterReady, setAutoPauseAfterReady] = useState(0);
  const [afterPrompt, setAfterPrompt] = useState<"resume" | "restart" | "none">(
    "resume",
  );
  const [commandMessage, setCommandMessage] = useState<string | null>(null);
  const [isCommandSaving, setIsCommandSaving] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const previousStats = useRef<DashboardStats | null>(null);
  const audioContext = useRef<AudioContext | null>(null);
  const soundEnabled = useRef(false);

  const filteredProjects = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return projects;
    }
    return projects.filter((project) =>
      project.name.toLowerCase().includes(normalizedQuery),
    );
  }, [projects, query]);

  async function createProject() {
    setError(null);
    setCreateError(null);
    const name = newProjectName.trim();
    if (!name) {
      setCreateError("Project name is required.");
      return;
    }
    setIsCreatingProject(true);
    try {
      const response = await fetch("/api/projects", {
        method: "POST",
        credentials: "same-origin",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name }),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => null);
        setCreateError(body?.error?.message ?? "Project was not created.");
        return;
      }

      const body = (await response.json()) as { project: Project };
      const project = { ...body.project, displayStatus: "Setup" };
      setProjects((current) => [...current, project]);
      setSelectedProject(project);
      setNewProjectName("");
      setIsCreateOpen(false);
    } catch {
      setCreateError("Project was not created. Check connection and try again.");
    } finally {
      setIsCreatingProject(false);
    }
  }

  async function deleteProject(project: Project) {
    setError(null);
    const response = await fetch(`/api/projects/${project.id}`, {
      method: "DELETE",
    });

    if (!response.ok) {
      setError("Project was not deleted");
      return;
    }

    setProjects((current) => current.filter((item) => item.id !== project.id));
    setSelectedProject(null);
  }

  async function refreshInstallCommand(project: Project) {
    setError(null);
    setIsInstallLoading(true);
    const response = await fetch(`/api/projects/${project.id}/bootstrap-token`, {
      method: "POST",
    });
    setIsInstallLoading(false);

    if (!response.ok) {
      setInstallCommand(null);
      setError("Install command was not created");
      return null;
    }

    const body = (await response.json()) as {
      expiresAt: string;
      commands: { shell: string };
    };
    setInstallCommand(body.commands.shell);
    setInstallExpiresAt(body.expiresAt);
    return body.commands.shell;
  }

  async function copyInstallCommand(project: Project) {
    const command = await refreshInstallCommand(project);
    if (!command) {
      return;
    }
    await navigator.clipboard.writeText(command);
  }

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST" });
    window.location.href = "/login";
  }

  async function loadSubscription() {
    const response = await fetch("/api/subscription");
    if (!response.ok) {
      return;
    }
    const body = (await response.json()) as { subscription: Subscription };
    setSubscription(body.subscription);
  }

  async function subscribe() {
    setError(null);
    const response = await fetch("/api/subscription/subscribe", {
      method: "POST",
    });
    if (!response.ok) {
      const body = await response.json().catch(() => null);
      setError(body?.error?.message ?? "Checkout was not created");
      return;
    }
    const body = (await response.json()) as { checkoutUrl: string };
    window.location.href = body.checkoutUrl;
  }

  async function cancelSubscription() {
    setError(null);
    const response = await fetch("/api/subscription/cancel", {
      method: "POST",
    });
    if (!response.ok) {
      setError("Subscription was not canceled");
      return;
    }
    await loadSubscription();
  }

  async function exportData() {
    setError(null);
    setIsExporting(true);
    const response = await fetch("/api/account/export", { method: "POST" });
    setIsExporting(false);
    if (!response.ok) {
      const body = await response.json().catch(() => null);
      setError(body?.error?.message ?? "Export was not created");
      return;
    }
    const body = (await response.json()) as { downloadUrl: string };
    window.location.href = body.downloadUrl;
  }

  async function disconnectInstalls() {
    setError(null);
    const response = await fetch("/api/account/disconnect-installs", {
      method: "POST",
    });
    if (!response.ok) {
      setError("Installs were not disconnected");
      return;
    }
    setError("Local installs disconnected. Re-run the install command to reconnect.");
  }

  async function proposeCommandSettings(project: Project) {
    setError(null);
    setCommandMessage(null);
    setIsCommandSaving(true);
    const response = await fetch(`/api/projects/${project.id}/command-proposals`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        command: commandText,
        pauseDuringPrompt,
        autoPauseAfterReadySeconds: autoPauseAfterReady,
        healthCommand,
        afterPrompt,
      }),
    });
    setIsCommandSaving(false);
    if (!response.ok) {
      const body = await response.json().catch(() => null);
      setError(body?.error?.message ?? "Command proposal was not saved");
      return;
    }
    setCommandMessage("Pending local approval");
  }

  async function sendCommandAction(
    project: Project,
    action: "start" | "pause" | "resume" | "restart",
  ) {
    setError(null);
    setCommandMessage(null);
    const response = await fetch(`/api/projects/${project.id}/actions`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ action }),
    });
    if (!response.ok) {
      setError("Command action was not queued");
      return;
    }
    setCommandMessage(`${action} queued`);
  }

  const playTone = useCallback(async (frequency: number) => {
    const context = audioContext.current ?? new AudioContext();
    audioContext.current = context;
    if (context.state === "suspended") {
      await context.resume();
    }
    const oscillator = context.createOscillator();
    const gain = context.createGain();
    oscillator.frequency.value = frequency;
    gain.gain.value = 0.04;
    oscillator.connect(gain);
    gain.connect(context.destination);
    oscillator.start();
    oscillator.stop(context.currentTime + 0.12);
  }, []);

  const playSoundsForStats = useCallback((nextStats: DashboardStats) => {
    const previous = previousStats.current;
    previousStats.current = nextStats;
    if (!previous || !soundEnabled.current) {
      return;
    }
    if (nextStats.successes > previous.successes) {
      void playTone(760);
    } else if (
      nextStats.attention > previous.attention ||
      nextStats.failures > previous.failures
    ) {
      void playTone(320);
    }
  }, [playTone]);

  function enableSound() {
    const context = new AudioContext();
    audioContext.current = context;
    localStorage.setItem("vibeengine:sound", "enabled");
    soundEnabled.current = true;
    setIsSoundEnabled(true);
    void playTone(660);
  }

  useEffect(() => {
    let cancelled = false;

    async function loadProjects() {
      const timezone =
        Intl.DateTimeFormat().resolvedOptions().timeZone ?? "UTC";
      const response = await fetch(
        `/api/projects?timezone=${encodeURIComponent(timezone)}`,
      );
      if (cancelled) {
        return;
      }
      if (!response.ok) {
        setError("Projects unavailable");
        setIsLoading(false);
        return;
      }
      const body = (await response.json()) as {
        projects: Project[];
        stats: DashboardStats;
      };
      if (!cancelled) {
        setProjects(body.projects);
        playSoundsForStats(body.stats);
        setStats(body.stats);
        setIsLoading(false);
      }
    }

    void loadProjects();
    const interval = window.setInterval(() => void loadProjects(), 5000);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [playSoundsForStats]);

  return (
    <main className="min-h-screen bg-[#050505] px-5 py-5 text-white">
      <div className="mx-auto flex w-full max-w-6xl items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Projects</h1>
          <p className="mt-1 text-sm text-white/50">Today across all projects</p>
        </div>
        <button
          className="flex size-10 items-center justify-center rounded-md border border-white/10 bg-white/[0.045] hover:bg-white/[0.07]"
          onClick={() => {
            setIsAccountOpen(true);
            void loadSubscription();
          }}
          type="button"
        >
            <User size={18} />
        </button>
      </div>

      <section className="mx-auto mt-7 grid w-full max-w-6xl gap-4 md:grid-cols-3">
        <div className="rounded-lg border border-white/10 bg-white/[0.045] p-4">
          <p className="text-sm text-white/50">Prompts</p>
          <p className="mt-2 text-3xl font-semibold">{stats.prompts}</p>
        </div>
        <div className="rounded-lg border border-white/10 bg-white/[0.045] p-4">
          <p className="text-sm text-white/50">Prompt time</p>
          <p className="mt-2 text-3xl font-semibold">
            {Math.round(stats.promptTimeMs / 60000)}m
          </p>
        </div>
        <div className="rounded-lg border border-white/10 bg-white/[0.045] p-4">
          <p className="text-sm text-white/50">Sound</p>
          <button
            className="mt-3 inline-flex items-center gap-2 rounded-md border border-white/10 px-3 py-2 text-sm text-white/80"
            onClick={enableSound}
            type="button"
          >
            <Bell size={15} />
            {isSoundEnabled ? "Sound on" : "Enable sound"}
          </button>
        </div>
      </section>

      <section className="mx-auto mt-5 w-full max-w-6xl">
        <div className="flex flex-col gap-3 sm:flex-row">
          <label className="flex h-11 flex-1 items-center gap-2 rounded-md border border-white/10 bg-white/[0.045] px-3">
            <Search size={17} className="text-white/45" />
            <input
              className="w-full bg-transparent text-sm outline-none placeholder:text-white/35"
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search projects"
              value={query}
            />
          </label>
          <button
            className="inline-flex h-11 items-center justify-center gap-2 rounded-md bg-white px-4 text-sm font-medium text-black"
            onClick={() => {
              setCreateError(null);
              setIsCreateOpen(true);
            }}
            type="button"
          >
            <Plus size={17} />
            Project
          </button>
        </div>
        {error ? <p className="mt-3 text-sm text-red-300">{error}</p> : null}
        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {isLoading ? (
            <div className="rounded-lg border border-white/10 bg-white/[0.045] p-4 text-sm text-white/50">
              Loading
            </div>
          ) : null}
          {filteredProjects.map((project) => (
            <button
              className="rounded-lg border border-white/10 bg-white/[0.045] p-4 text-left hover:bg-white/[0.07]"
              key={project.id}
              onClick={() => {
                setSelectedProject(project);
                setInstallCommand(null);
                setInstallExpiresAt(null);
                setCommandMessage(null);
              }}
              type="button"
            >
              <p className="font-medium">{project.name}</p>
              <p className="mt-2 text-sm text-white/52">
                {project.displayStatus}
              </p>
            </button>
          ))}
          {!isLoading && filteredProjects.length === 0 ? (
            <div className="rounded-lg border border-white/10 bg-white/[0.045] p-4 text-sm text-white/50">
              No projects
            </div>
          ) : null}
        </div>
      </section>

      {isCreateOpen ? (
        <div className="fixed inset-0 z-20 grid place-items-center bg-black/70 p-5 backdrop-blur-md">
          <div className="w-full max-w-md rounded-lg border border-white/10 bg-[#111] p-5">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold">New project</h2>
              <button
                className="flex size-8 items-center justify-center rounded-md hover:bg-white/8"
                onClick={() => setIsCreateOpen(false)}
                type="button"
              >
                <X size={17} />
              </button>
            </div>
            <form
              className="mt-5 grid gap-3"
              onSubmit={(event) => {
                event.preventDefault();
                if (!isCreatingProject) {
                  void createProject();
                }
              }}
            >
              <input
                autoFocus
                className="h-11 w-full rounded-md border border-white/10 bg-black/35 px-3 text-sm outline-none ring-emerald-300/40 placeholder:text-white/35 focus:ring-2"
                disabled={isCreatingProject}
                onChange={(event) => setNewProjectName(event.target.value)}
                placeholder="Project name"
                value={newProjectName}
              />
              {createError ? (
                <div className="rounded-md border border-red-300/25 bg-red-300/10 px-3 py-2 text-sm text-red-100">
                  {createError}
                </div>
              ) : null}
              <button
                className="h-11 w-full rounded-md bg-white text-sm font-medium text-black hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={isCreatingProject}
                type="submit"
              >
                {isCreatingProject ? "Creating" : "Create"}
              </button>
            </form>
          </div>
        </div>
      ) : null}

      {isAccountOpen ? (
        <div className="fixed inset-0 z-20 grid place-items-center bg-black/70 p-5 backdrop-blur-md">
          <div className="w-full max-w-sm rounded-lg border border-white/10 bg-[#111] p-5">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold">Account</h2>
              <button
                className="flex size-8 items-center justify-center rounded-md hover:bg-white/8"
                onClick={() => setIsAccountOpen(false)}
                type="button"
              >
                <X size={17} />
              </button>
            </div>
            <div className="mt-5 grid gap-2">
              <div className="rounded-md border border-white/10 px-3 py-3 text-sm text-white/80">
                <div className="flex items-center justify-between gap-3">
                  <span>Subscription</span>
                  <span className="rounded-full border border-white/10 px-2 py-1 text-xs text-white/60">
                    {subscription?.effectiveState ?? "loading"}
                  </span>
                </div>
                <p className="mt-2 text-xs text-white/45">
                  {subscription
                    ? `${subscription.plan} - ${subscription.priceRub} RUB / month`
                    : "Loading"}
                </p>
                <div className="mt-3 flex gap-2">
                  {subscription?.effectiveState === "inactive" ||
                  subscription?.effectiveState === "past_due" ||
                  subscription?.state === "canceled" ? (
                    <button
                      className="rounded-md bg-white px-3 py-2 text-xs font-medium text-black"
                      onClick={() => void subscribe()}
                      type="button"
                    >
                      Subscribe
                    </button>
                  ) : null}
                  {subscription?.state === "active" ||
                  subscription?.state === "trialing" ||
                  subscription?.state === "grace" ? (
                    <button
                      className="rounded-md border border-white/10 px-3 py-2 text-xs text-white/70 hover:bg-white/8"
                      onClick={() => void cancelSubscription()}
                      type="button"
                    >
                      Cancel
                    </button>
                  ) : null}
                </div>
              </div>
              <button
                className="rounded-md border border-white/10 px-3 py-3 text-left text-sm text-white/80 hover:bg-white/8 disabled:opacity-45"
                disabled={isExporting}
                onClick={() => void exportData()}
                type="button"
              >
                {isExporting ? "Preparing export" : "Export data"}
              </button>
              <button
                className="rounded-md border border-white/10 px-3 py-3 text-left text-sm text-white/80 hover:bg-white/8"
                onClick={() => void disconnectInstalls()}
                type="button"
              >
                Disconnect installs
              </button>
              <button
                className="rounded-md border border-white/10 px-3 py-3 text-left text-sm text-white/80 hover:bg-white/8"
                onClick={() => void logout()}
                type="button"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {selectedProject ? (
        <aside className="fixed bottom-0 right-0 top-0 z-10 w-full max-w-md border-l border-white/10 bg-[#0d0d0d] p-5 shadow-2xl shadow-black/50">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold">{selectedProject.name}</h2>
              <p className="mt-1 text-sm text-white/50">
                {selectedProject.displayStatus}
              </p>
            </div>
            <button
              className="flex size-8 items-center justify-center rounded-md hover:bg-white/8"
              onClick={() => {
                setSelectedProject(null);
                setInstallCommand(null);
                setInstallExpiresAt(null);
              }}
              type="button"
            >
              <X size={17} />
            </button>
          </div>
          <div className="mt-7 grid gap-3">
            <div className="rounded-md border border-white/10 p-3">
              <div className="flex items-center justify-between gap-3">
                <button
                  className="inline-flex items-center gap-2 text-left text-sm text-white/80"
                  disabled={isInstallLoading}
                  onClick={() => void refreshInstallCommand(selectedProject)}
                  type="button"
                >
                  <Terminal size={16} />
                  {isInstallLoading ? "Creating command" : "Install command"}
                </button>
                <button
                  className="flex size-8 items-center justify-center rounded-md border border-white/10 text-white/75 hover:bg-white/8 disabled:opacity-45"
                  disabled={isInstallLoading}
                  onClick={() => void copyInstallCommand(selectedProject)}
                  title="Copy fresh install command"
                  type="button"
                >
                  <Copy size={15} />
                </button>
              </div>
              {installCommand ? (
                <div className="mt-3">
                  <pre className="max-h-32 overflow-auto rounded-md bg-black/35 p-3 text-xs leading-5 text-white/70">
                    {installCommand}
                  </pre>
                  {installExpiresAt ? (
                    <p className="mt-2 text-xs text-white/40">
                      Expires {new Date(installExpiresAt).toLocaleTimeString()}
                    </p>
                  ) : null}
                </div>
              ) : null}
            </div>
            <div className="rounded-md border border-white/10 p-3">
              <div className="flex items-center justify-between gap-3">
                <p className="text-sm font-medium text-white/80">Command</p>
                {commandMessage ? (
                  <span className="rounded-full border border-white/10 px-2 py-1 text-xs text-white/50">
                    {commandMessage}
                  </span>
                ) : null}
              </div>
              <textarea
                className="mt-3 min-h-20 w-full resize-none rounded-md border border-white/10 bg-black/35 px-3 py-2 text-sm text-white/80 outline-none placeholder:text-white/35"
                onChange={(event) => setCommandText(event.target.value)}
                placeholder="npm run dev"
                value={commandText}
              />
              <div
                className={`mt-3 grid gap-3 text-sm ${
                  commandText.trim() ? "opacity-100" : "opacity-45"
                }`}
              >
                <label className="flex items-center gap-2 text-white/70">
                  <input
                    checked={pauseDuringPrompt}
                    className="size-4"
                    onChange={(event) => setPauseDuringPrompt(event.target.checked)}
                    type="checkbox"
                  />
                  Pause during prompt
                </label>
                <label className="grid gap-1 text-white/70">
                  <span className="text-xs text-white/45">
                    Auto-pause after ready, seconds
                  </span>
                  <input
                    className="h-9 rounded-md border border-white/10 bg-black/35 px-3 outline-none"
                    min={0}
                    onChange={(event) =>
                      setAutoPauseAfterReady(Number(event.target.value))
                    }
                    type="number"
                    value={autoPauseAfterReady}
                  />
                </label>
                <label className="grid gap-1 text-white/70">
                  <span className="text-xs text-white/45">After prompt</span>
                  <select
                    className="h-9 rounded-md border border-white/10 bg-black/35 px-3 outline-none"
                    onChange={(event) =>
                      setAfterPrompt(
                        event.target.value as "resume" | "restart" | "none",
                      )
                    }
                    value={afterPrompt}
                  >
                    <option value="resume">Resume</option>
                    <option value="restart">Restart</option>
                    <option value="none">Do nothing</option>
                  </select>
                </label>
                <input
                  className="h-9 rounded-md border border-white/10 bg-black/35 px-3 text-sm outline-none placeholder:text-white/35"
                  onChange={(event) => setHealthCommand(event.target.value)}
                  placeholder="Optional health command"
                  value={healthCommand}
                />
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                <button
                  className="rounded-md bg-white px-3 py-2 text-xs font-medium text-black disabled:opacity-45"
                  disabled={!commandText.trim() || isCommandSaving}
                  onClick={() => void proposeCommandSettings(selectedProject)}
                  type="button"
                >
                  {isCommandSaving ? "Saving" : "Propose"}
                </button>
                <button
                  className="inline-flex items-center gap-1 rounded-md border border-white/10 px-3 py-2 text-xs text-white/70 hover:bg-white/8"
                  onClick={() => void sendCommandAction(selectedProject, "resume")}
                  type="button"
                >
                  <Play size={13} />
                  Resume
                </button>
                <button
                  className="inline-flex items-center gap-1 rounded-md border border-white/10 px-3 py-2 text-xs text-white/70 hover:bg-white/8"
                  onClick={() => void sendCommandAction(selectedProject, "pause")}
                  type="button"
                >
                  <Pause size={13} />
                  Pause
                </button>
                <button
                  className="inline-flex items-center gap-1 rounded-md border border-white/10 px-3 py-2 text-xs text-white/70 hover:bg-white/8"
                  onClick={() => void sendCommandAction(selectedProject, "restart")}
                  type="button"
                >
                  <RotateCcw size={13} />
                  Restart
                </button>
              </div>
            </div>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-red-300/20 px-3 py-3 text-left text-sm text-red-200 hover:bg-red-300/10"
              onClick={() => void deleteProject(selectedProject)}
              type="button"
            >
              <Trash2 size={16} />
              Delete project
            </button>
          </div>
        </aside>
      ) : null}
    </main>
  );
}
