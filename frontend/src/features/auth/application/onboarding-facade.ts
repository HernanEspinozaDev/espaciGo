import type {
  ActionLinkStatus,
  EmailRequest,
  OnboardingPort,
  RecoveryConsumeRequest,
  RegisterRequest,
} from "./onboarding.port";
import type { ActionCredential } from "../domain/action-credential";

export class OnboardingFacade {
  #generation = 0;
  #controller: AbortController | null = null;

  constructor(private readonly port: OnboardingPort) {}

  cancel(): void {
    this.#generation += 1;
    this.#controller?.abort();
    this.#controller = null;
  }

  terms(): Promise<Awaited<ReturnType<OnboardingPort["terms"]>>> {
    return this.#run((signal) => this.port.terms(signal));
  }

  register(input: RegisterRequest): Promise<201 | 202> {
    return this.#run((signal) => this.port.register(input, signal));
  }

  reissueVerification(input: EmailRequest): Promise<void> {
    return this.#run((signal) => this.port.reissueVerification(input, signal));
  }

  verifyEmail(credential: ActionCredential): Promise<void> {
    return this.#run((signal) => this.port.verifyEmail(credential, signal));
  }

  inspectVerification(credential: ActionCredential): Promise<ActionLinkStatus> {
    return this.#run((signal) =>
      this.port.inspectVerification(credential, signal),
    );
  }

  requestRecovery(input: EmailRequest): Promise<void> {
    return this.#run((signal) => this.port.requestRecovery(input, signal));
  }

  consumeRecovery(input: RecoveryConsumeRequest): Promise<void> {
    return this.#run((signal) => this.port.consumeRecovery(input, signal));
  }

  inspectRecovery(credential: ActionCredential): Promise<ActionLinkStatus> {
    return this.#run((signal) => this.port.inspectRecovery(credential, signal));
  }

  async #run<T>(operation: (signal: AbortSignal) => Promise<T>): Promise<T> {
    this.#controller?.abort();
    const controller = new AbortController();
    const generation = ++this.#generation;
    this.#controller = controller;
    const result = await operation(controller.signal);
    if (controller.signal.aborted || generation !== this.#generation) {
      throw new DOMException("Operation was cancelled.", "AbortError");
    }
    if (this.#controller === controller) this.#controller = null;
    return result;
  }
}
