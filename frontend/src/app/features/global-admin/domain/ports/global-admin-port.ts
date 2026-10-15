import { Account,Overview,Page } from '../global-admin-models';
export abstract class GlobalAdminPort { abstract overview(token:string):Promise<Overview>;abstract list(token:string,filters:Record<string,string|number|undefined>):Promise<Page>;abstract account(token:string,id:string):Promise<Account> }
