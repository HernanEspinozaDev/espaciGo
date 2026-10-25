"use client";

import { useEffect, useMemo } from "react";
import { OnboardingFacade } from "../application/onboarding-facade";
import { OpenApiOnboardingAdapter } from "../infrastructure/onboarding-openapi-adapter";

export function useOnboarding(): OnboardingFacade {
  const facade = useMemo(
    () => new OnboardingFacade(new OpenApiOnboardingAdapter()),
    [],
  );
  useEffect(() => () => facade.cancel(), [facade]);
  return facade;
}
