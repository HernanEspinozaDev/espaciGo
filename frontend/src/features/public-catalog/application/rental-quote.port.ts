import type {
  RentalQuoteRequest,
  RentalQuoteResult,
} from "../domain/rental-quote";

export interface RentalQuotePort {
  createQuote(
    request: RentalQuoteRequest,
    signal?: AbortSignal,
  ): Promise<RentalQuoteResult>;
}
