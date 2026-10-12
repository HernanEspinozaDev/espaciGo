import type {
  LocalRentalOperation,
  LocalRentalOperationInput,
  LocalRentalOperationListEnvelope,
  LocalSyntheticContract,
} from '../../../../core/contracts/models';
export type Envelope<T> = { data: T; notice: string };
export type Attempt = { key: string; payload: string };

export abstract class BookingLifecyclePort {
  abstract createOrGetContract(
    token: string,
    reservationId: string,
  ): Promise<Envelope<LocalSyntheticContract>>;
  abstract getContract(
    token: string,
    contractId: string,
  ): Promise<Envelope<LocalSyntheticContract>>;
  abstract signContract(
    token: string,
    contractId: string,
  ): Promise<Envelope<LocalSyntheticContract>>;
  abstract rejectContract(
    token: string,
    contractId: string,
  ): Promise<Envelope<LocalSyntheticContract>>;
  abstract contractPdf(token: string, contractId: string): Promise<Blob>;
  abstract operations(
    token: string,
    reservationId: string,
  ): Promise<LocalRentalOperationListEnvelope>;
  abstract record(
    token: string,
    reservationId: string,
    action: 'check-in' | 'check-out' | 'reception',
    body: LocalRentalOperationInput,
    key: string,
  ): Promise<Envelope<LocalRentalOperation>>;
  abstract evidence(token: string, reservationId: string, evidenceId: string): Promise<Blob>;
}
