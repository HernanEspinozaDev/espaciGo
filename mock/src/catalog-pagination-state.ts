export class CatalogPaginationState {
  private generation=0;
  private cursor="";
  loading=false;

  beginRequest():number { this.generation++;this.loading=true;return this.generation; }
  accepts(generation:number,sessionAtStart:string,sessionNow:string):boolean { return generation===this.generation&&sessionAtStart===sessionNow; }
  finish(generation:number,nextCursor:string):void { if(generation!==this.generation)return;this.loading=false;this.cursor=nextCursor; }
  invalidate():void { this.generation++;this.loading=false;this.cursor=""; }
  beginNext():string|null { if(this.loading||!this.cursor)return null;this.generation++;this.loading=true;return this.cursor; }
  get canNext():boolean { return !this.loading&&Boolean(this.cursor); }
}
