import "client-only";

import { createRentalQuoteService } from "../application/rental-quote.service";
import { createRentalQuoteAdapter } from "./rental-quote-openapi-adapter";

export const publicRentalQuote = createRentalQuoteService(
  createRentalQuoteAdapter(),
);
