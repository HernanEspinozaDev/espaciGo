import type {
  DemoActor,
  DemoBlockRequest,
  DemoConfirmation,
  DemoMonth,
  DemoOwnerSpace,
  DemoPublicSpace,
  DemoQuote,
  DemoSession,
  DemoSpaceConfiguration,
} from "../domain/types";

export interface DemoPort {
  session(signal?: AbortSignal): Promise<DemoSession | null>;
  login(actor: DemoActor, signal?: AbortSignal): Promise<DemoSession>;
  logout(signal?: AbortSignal): Promise<void>;
  spaces(
    owner: boolean,
    signal?: AbortSignal,
  ): Promise<readonly (DemoOwnerSpace | DemoPublicSpace)[]>;
  month(
    spaceId: string,
    month: string,
    duration: number,
    owner: boolean,
    signal?: AbortSignal,
  ): Promise<DemoMonth>;
  updateSpace(
    spaceId: string,
    config: DemoSpaceConfiguration,
    signal?: AbortSignal,
  ): Promise<DemoOwnerSpace>;
  createBlock(
    spaceId: string,
    block: DemoBlockRequest,
    signal?: AbortSignal,
  ): Promise<string>;
  deleteBlock(
    spaceId: string,
    blockId: string,
    revision: string,
    signal?: AbortSignal,
  ): Promise<void>;
  quote(
    spaceId: string,
    startAt: string,
    endAt: string,
    signal?: AbortSignal,
  ): Promise<DemoQuote>;
  confirm(
    quoteId: string,
    request: { approved_contract: true; intent_id: string },
    signal?: AbortSignal,
  ): Promise<DemoConfirmation>;
  attempt(intentId: string, signal?: AbortSignal): Promise<DemoConfirmation>;
}
