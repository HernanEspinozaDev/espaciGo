import type {
  RentalQuoteRequest,
  RentalQuoteResult,
} from "../domain/rental-quote";
import type { RentalQuotePort } from "./rental-quote.port";

export function createRentalQuoteService(port: RentalQuotePort) {
  return {
    createQuote(
      request: RentalQuoteRequest,
      signal?: AbortSignal,
    ): Promise<RentalQuoteResult> {
      return port.createQuote(request, signal);
    },
  };
}
