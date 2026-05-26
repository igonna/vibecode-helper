export type SubscriptionLike = {
  state: string;
  trialEndsAt: Date | null;
  currentPeriodEnd?: Date | null;
};

export function effectiveSubscriptionState(subscription?: SubscriptionLike) {
  if (!subscription) {
    return "inactive";
  }

  const now = new Date();
  if (subscription.state === "trialing") {
    return !subscription.trialEndsAt || subscription.trialEndsAt > now
      ? "trialing"
      : "inactive";
  }
  if (subscription.state === "active") {
    if (!subscription.currentPeriodEnd || subscription.currentPeriodEnd > now) {
      return "active";
    }
    const graceEndsAt = new Date(
      subscription.currentPeriodEnd.getTime() + 3 * 24 * 60 * 60 * 1000,
    );
    return graceEndsAt > now ? "grace" : "past_due";
  }
  if (subscription.state === "grace") {
    if (!subscription.currentPeriodEnd || subscription.currentPeriodEnd > now) {
      return "grace";
    }
    return "past_due";
  }
  return subscription.state;
}

export function cloudEntitlementActive(subscription?: SubscriptionLike) {
  const state = effectiveSubscriptionState(subscription);
  return state === "trialing" || state === "active" || state === "grace";
}
