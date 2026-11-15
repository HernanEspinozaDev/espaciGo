import type { components } from "@/lib/api/generated/openapi";

export type DemoSession = components["schemas"]["DemoSession"];
export type DemoActor = DemoSession["actor"];
export type DemoInterval = components["schemas"]["DemoInterval"];
export type DemoDay = components["schemas"]["DemoDay"];
export type DemoOwnerSpace = components["schemas"]["DemoSpace"];
export type DemoPublicSpace = components["schemas"]["DemoPublicSpace"];
export type DemoOwnerMonth = components["schemas"]["DemoOwnerMonth"];
export type DemoPublicMonth = components["schemas"]["DemoMonth"];
export type DemoMonth = DemoOwnerMonth | DemoPublicMonth;
export type DemoQuote = components["schemas"]["DemoQuote"];
export type DemoConfirmation = components["schemas"]["DemoConfirmation"];
export type DemoSpaceConfiguration = components["schemas"]["DemoConfig"];
export type DemoBlockRequest = components["schemas"]["DemoBlockRequest"];
export type DemoConfirmRequest = components["schemas"]["DemoConfirmRequest"];
export type DemoMode = "host" | "public";
