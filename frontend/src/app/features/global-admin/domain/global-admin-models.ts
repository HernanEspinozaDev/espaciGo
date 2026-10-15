export type Account = { account_id:string; email:string; display_name:string|null; state:string; created_at:string; administrative_blocked:boolean; roles:string[]; publication_count:number; reservation_count:number; incident_count:number };
export type Page = { items:Account[]; next_cursor:string; matching_total:number; generated_at:string };
export type Overview = { total_accounts:number; administrative_blocked:number; by_state:Record<string,number>; by_role:Record<string,number>; generated_at:string; definition:string };
