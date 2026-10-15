import { CommonModule,DatePipe } from '@angular/common';
import { Component,inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { GlobalAdminFacade } from './application/global-admin-facade';
@Component({selector:'app-global-admin-page',imports:[CommonModule,FormsModule,DatePipe],templateUrl:'./global-admin-page.html',styleUrl:'./global-admin-page.scss',providers:[GlobalAdminFacade]})
export class GlobalAdminPage {
 readonly vm=inject(GlobalAdminFacade);readonly overview=this.vm.overview;readonly page=this.vm.page;readonly selected=this.vm.selected;readonly loading=this.vm.loading;readonly error=this.vm.error;
 accountId='';emailPrefix='';namePrefix='';role='';state='';blocked='';createdFrom='';createdUntil='';
 load(cursor=''):void{void this.vm.load({account_id:this.accountId.trim()||undefined,email_prefix:this.emailPrefix.trim()||undefined,name_prefix:this.namePrefix.trim()||undefined,role:this.role||undefined,state:this.state||undefined,administrative_blocked:this.blocked||undefined,created_from:this.createdFrom?new Date(this.createdFrom).toISOString():undefined,created_until:this.createdUntil?new Date(this.createdUntil).toISOString():undefined},cursor)}
 open(id:string):void{void this.vm.open(id)}
 clearFilters():void{this.accountId='';this.emailPrefix='';this.namePrefix='';this.role='';this.state='';this.blocked='';this.createdFrom='';this.createdUntil='';this.load()}
 next():void{const cursor=this.page()?.next_cursor;if(cursor)this.load(cursor)}
}
