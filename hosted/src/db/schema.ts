import { sql } from "drizzle-orm";
import {
  bigint,
  index,
  integer,
  jsonb,
  pgEnum,
  pgTable,
  primaryKey,
  text,
  timestamp,
  uniqueIndex,
  uuid,
} from "drizzle-orm/pg-core";

const timestamps = {
  createdAt: timestamp("created_at", { withTimezone: true })
    .defaultNow()
    .notNull(),
  updatedAt: timestamp("updated_at", { withTimezone: true }),
};

export const projectState = pgEnum("project_state", ["active", "deleted"]);
export const subscriptionState = pgEnum("subscription_state", [
  "trialing",
  "active",
  "grace",
  "past_due",
  "inactive",
  "canceled",
]);
export const remoteActionState = pgEnum("remote_action_state", [
  "pending",
  "sent",
  "applied",
  "failed",
]);
export const commandProposalState = pgEnum("command_proposal_state", [
  "pending_device",
  "pending_local_approval",
  "approved",
  "rejected",
  "applied",
  "failed",
]);

export const users = pgTable(
  "users",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    email: text("email").notNull(),
    emailVerifiedAt: timestamp("email_verified_at", { withTimezone: true }),
    engineAuthEpoch: integer("engine_auth_epoch").default(0).notNull(),
    deletedAt: timestamp("deleted_at", { withTimezone: true }),
    createdAt: timestamps.createdAt,
  },
  (table) => [uniqueIndex("users_email_idx").on(table.email)],
);

export const accounts = pgTable(
  "accounts",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    ownerUserId: uuid("owner_user_id")
      .notNull()
      .references(() => users.id),
    name: text("name").notNull(),
    createdAt: timestamps.createdAt,
    deletedAt: timestamp("deleted_at", { withTimezone: true }),
  },
  (table) => [index("accounts_owner_user_id_idx").on(table.ownerUserId)],
);

export const sessions = pgTable(
  "sessions",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    userId: uuid("user_id")
      .notNull()
      .references(() => users.id),
    sessionHash: text("session_hash").notNull(),
    expiresAt: timestamp("expires_at", { withTimezone: true }).notNull(),
    createdAt: timestamps.createdAt,
    revokedAt: timestamp("revoked_at", { withTimezone: true }),
  },
  (table) => [
    uniqueIndex("sessions_session_hash_idx").on(table.sessionHash),
    index("sessions_user_id_idx").on(table.userId),
  ],
);

export const emailLoginCodes = pgTable(
  "email_login_codes",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    email: text("email").notNull(),
    codeHash: text("code_hash").notNull(),
    purpose: text("purpose").notNull(),
    inviteCodeHash: text("invite_code_hash"),
    expiresAt: timestamp("expires_at", { withTimezone: true }).notNull(),
    attemptCount: integer("attempt_count").default(0).notNull(),
    consumedAt: timestamp("consumed_at", { withTimezone: true }),
    createdAt: timestamps.createdAt,
  },
  (table) => [
    index("email_login_codes_email_purpose_idx").on(table.email, table.purpose),
  ],
);

export const inviteCodes = pgTable(
  "invite_codes",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    codeHash: text("code_hash").notNull(),
    label: text("label").notNull(),
    maxRedemptions: integer("max_redemptions"),
    redeemedCount: integer("redeemed_count").default(0).notNull(),
    expiresAt: timestamp("expires_at", { withTimezone: true }),
    disabledAt: timestamp("disabled_at", { withTimezone: true }),
    createdAt: timestamps.createdAt,
  },
  (table) => [uniqueIndex("invite_codes_code_hash_idx").on(table.codeHash)],
);

export const projects = pgTable(
  "projects",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    name: text("name").notNull(),
    state: projectState("state").default("active").notNull(),
    createdAt: timestamps.createdAt,
    deletedAt: timestamp("deleted_at", { withTimezone: true }),
    lastSeenAt: timestamp("last_seen_at", { withTimezone: true }),
    lastStatus: text("last_status"),
  },
  (table) => [
    uniqueIndex("projects_account_name_active_idx")
      .on(table.accountId, table.name)
      .where(sql`${table.deletedAt} is null`),
    index("projects_account_id_idx").on(table.accountId),
  ],
);

export const projectEvents = pgTable(
  "project_events",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    projectId: uuid("project_id")
      .notNull()
      .references(() => projects.id),
    localEventId: text("local_event_id").notNull(),
    toolAdapter: text("tool_adapter").notNull(),
    nativeEventName: text("native_event_name"),
    nativeEventId: text("native_event_id"),
    type: text("type").notNull(),
    occurredAt: timestamp("occurred_at", { withTimezone: true }).notNull(),
    durationMs: integer("duration_ms"),
    result: text("result"),
    status: text("status"),
    createdAt: timestamps.createdAt,
  },
  (table) => [
    uniqueIndex("project_events_local_event_idx").on(
      table.accountId,
      table.projectId,
      table.localEventId,
    ),
    index("project_events_project_occurred_idx").on(
      table.projectId,
      table.occurredAt,
    ),
  ],
);

export const dailyProjectStats = pgTable(
  "daily_project_stats",
  {
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    projectId: uuid("project_id")
      .notNull()
      .references(() => projects.id),
    dayKey: text("day_key").notNull(),
    timezone: text("timezone").notNull(),
    promptCount: integer("prompt_count").default(0).notNull(),
    promptDurationMs: bigint("prompt_duration_ms", { mode: "number" })
      .default(0)
      .notNull(),
    successCount: integer("success_count").default(0).notNull(),
    failureCount: integer("failure_count").default(0).notNull(),
    updatedAt: timestamp("updated_at", { withTimezone: true })
      .defaultNow()
      .notNull(),
  },
  (table) => [
    primaryKey({
      columns: [table.accountId, table.projectId, table.dayKey, table.timezone],
    }),
  ],
);

export const engineCredentials = pgTable(
  "engine_credentials",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    userId: uuid("user_id")
      .notNull()
      .references(() => users.id),
    refreshTokenHash: text("refresh_token_hash").notNull(),
    engineAuthEpoch: integer("engine_auth_epoch").default(0).notNull(),
    createdAt: timestamps.createdAt,
    lastSeenAt: timestamp("last_seen_at", { withTimezone: true }),
    revokedAt: timestamp("revoked_at", { withTimezone: true }),
  },
  (table) => [
    uniqueIndex("engine_credentials_refresh_hash_idx").on(
      table.refreshTokenHash,
    ),
    index("engine_credentials_account_id_idx").on(table.accountId),
    index("engine_credentials_user_id_idx").on(table.userId),
  ],
);

export const bootstrapTokens = pgTable(
  "bootstrap_tokens",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    projectId: uuid("project_id")
      .notNull()
      .references(() => projects.id),
    userId: uuid("user_id")
      .notNull()
      .references(() => users.id),
    tokenHash: text("token_hash").notNull(),
    expiresAt: timestamp("expires_at", { withTimezone: true }).notNull(),
    consumedAt: timestamp("consumed_at", { withTimezone: true }),
    createdAt: timestamps.createdAt,
  },
  (table) => [
    uniqueIndex("bootstrap_tokens_token_hash_idx").on(table.tokenHash),
    index("bootstrap_tokens_project_id_idx").on(table.projectId),
  ],
);

export const remoteActions = pgTable(
  "remote_actions",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    projectId: uuid("project_id")
      .notNull()
      .references(() => projects.id),
    type: text("type").notNull(),
    targetCommandHash: text("target_command_hash"),
    state: remoteActionState("state").default("pending").notNull(),
    createdAt: timestamps.createdAt,
    sentAt: timestamp("sent_at", { withTimezone: true }),
    finishedAt: timestamp("finished_at", { withTimezone: true }),
    resultMessage: text("result_message"),
  },
  (table) => [index("remote_actions_project_state_idx").on(table.projectId, table.state)],
);

export const commandProposals = pgTable(
  "command_proposals",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    projectId: uuid("project_id")
      .notNull()
      .references(() => projects.id),
    type: text("type").notNull(),
    state: commandProposalState("state").default("pending_device").notNull(),
    payloadJson: jsonb("payload_json").notNull(),
    commandHash: text("command_hash"),
    createdAt: timestamps.createdAt,
    seenAt: timestamp("seen_at", { withTimezone: true }),
    decidedAt: timestamp("decided_at", { withTimezone: true }),
    appliedAt: timestamp("applied_at", { withTimezone: true }),
  },
  (table) => [
    index("command_proposals_project_state_idx").on(
      table.projectId,
      table.state,
    ),
  ],
);

export const subscriptions = pgTable(
  "subscriptions",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id")
      .notNull()
      .references(() => accounts.id),
    state: subscriptionState("state").default("trialing").notNull(),
    plan: text("plan").notNull(),
    priceRub: integer("price_rub").notNull(),
    trialEndsAt: timestamp("trial_ends_at", { withTimezone: true }),
    currentPeriodEnd: timestamp("current_period_end", { withTimezone: true }),
    yookassaCustomerId: text("yookassa_customer_id"),
    yookassaPaymentMethodId: text("yookassa_payment_method_id"),
    yookassaLastPaymentId: text("yookassa_last_payment_id"),
    createdAt: timestamps.createdAt,
    updatedAt: timestamps.updatedAt,
  },
  (table) => [uniqueIndex("subscriptions_account_id_idx").on(table.accountId)],
);

export const webhookEvents = pgTable(
  "webhook_events",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    provider: text("provider").notNull(),
    providerEventId: text("provider_event_id").notNull(),
    payloadJson: jsonb("payload_json"),
    processedAt: timestamp("processed_at", { withTimezone: true }),
    createdAt: timestamps.createdAt,
  },
  (table) => [
    uniqueIndex("webhook_events_provider_event_idx").on(
      table.provider,
      table.providerEventId,
    ),
  ],
);

export const billingEvents = pgTable(
  "billing_events",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id").references(() => accounts.id),
    provider: text("provider").notNull(),
    type: text("type").notNull(),
    payloadJson: jsonb("payload_json"),
    createdAt: timestamps.createdAt,
  },
  (table) => [index("billing_events_account_id_idx").on(table.accountId)],
);

export const auditEvents = pgTable(
  "audit_events",
  {
    id: uuid("id").primaryKey().defaultRandom(),
    accountId: uuid("account_id").references(() => accounts.id),
    userId: uuid("user_id").references(() => users.id),
    type: text("type").notNull(),
    ipHash: text("ip_hash"),
    userAgentHash: text("user_agent_hash"),
    metadataJson: jsonb("metadata_json"),
    createdAt: timestamps.createdAt,
  },
  (table) => [
    index("audit_events_account_type_idx").on(table.accountId, table.type),
    index("audit_events_user_id_idx").on(table.userId),
  ],
);
