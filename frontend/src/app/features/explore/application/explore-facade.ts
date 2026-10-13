import { DestroyRef, Injectable, OnInit, inject, signal } from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, Router } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ExplorePort, PublicCatalogFilters, PublicSpace, PublicCategory } from '../domain/ports/explore-port';

@Injectable()
export class ExploreFacade implements OnInit {
  private readonly api=inject(ExplorePort); private readonly route=inject(ActivatedRoute); private readonly router=inject(Router);
  private readonly meta=inject(Meta); private readonly title=inject(Title); private readonly destroyRef=inject(DestroyRef);
  private generation=0;
  readonly categories=signal<PublicCategory[]>([]); readonly items=signal<PublicSpace[]>([]);
  readonly nextCursor=signal(''); readonly busy=signal(false); readonly error=signal('');
  readonly filters=signal({category_code:'',rate_unit:'',min_unit_price_clp:'',max_unit_price_clp:'',sort:'default'});

  constructor(){this.title.setTitle('Explorar espacios | EspaciGo');this.meta.addTag({name:'robots',content:'noindex, nofollow'});this.destroyRef.onDestroy(()=>this.meta.removeTag("name='robots'"));}
  ngOnInit():void{
    this.route.queryParamMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(params=>{
      this.filters.set({category_code:params.get('category_code')??'',rate_unit:params.get('rate_unit')??'',min_unit_price_clp:params.get('min_unit_price_clp')??'',max_unit_price_clp:params.get('max_unit_price_clp')??'',sort:params.get('sort')??'default'});
      void this.loadCategories(); void this.search(true,false);
    });
  }
  updateFilter(key:keyof ReturnType<typeof this.filters>,value:string):void{
    const next={...this.filters(),[key]:value};this.filters.set(next);this.generation++;this.items.set([]);this.nextCursor.set('');this.error.set('');
    void this.router.navigate([], {relativeTo:this.route,queryParams:{category_code:next.category_code||null,rate_unit:next.rate_unit||null,min_unit_price_clp:next.min_unit_price_clp||null,max_unit_price_clp:next.max_unit_price_clp||null,sort:next.sort==='default'?null:next.sort},replaceUrl:true});
  }
  async search(reset=true,updateUrl=true):Promise<void>{
    if(updateUrl){const f=this.filters();await this.router.navigate([], {relativeTo:this.route,queryParams:{category_code:f.category_code||null,rate_unit:f.rate_unit||null,min_unit_price_clp:f.min_unit_price_clp||null,max_unit_price_clp:f.max_unit_price_clp||null,sort:f.sort==='default'?null:f.sort},replaceUrl:true});}
    const generation=++this.generation;this.error.set('');this.busy.set(true);if(reset){this.items.set([]);this.nextCursor.set('');}
    const f=this.filters();const filters:PublicCatalogFilters={...f,category_code:f.category_code||undefined,rate_unit:f.rate_unit||undefined,min_unit_price_clp:f.min_unit_price_clp||undefined,max_unit_price_clp:f.max_unit_price_clp||undefined,sort:f.sort||'default',page_size:20,cursor:reset?undefined:this.nextCursor()||undefined};
    if(!validPrice(filters.min_unit_price_clp)||!validPrice(filters.max_unit_price_clp)){this.error.set('Ingresa precios CLP enteros válidos.');this.busy.set(false);return;}
    if((filters.min_unit_price_clp||filters.max_unit_price_clp||filters.sort!=='default')&&!filters.rate_unit){this.error.set('Elige hora, día o mes para comparar tarifas.');this.busy.set(false);return;}
    try{const page=await this.api.publicSearch(filters);if(generation!==this.generation)return;this.items.update(items=>reset?page.items:[...items,...page.items]);this.nextCursor.set(page.next_cursor??'');}
    catch{if(generation===this.generation)this.error.set('No fue posible cargar los espacios públicos. Inténtalo nuevamente.');}
    finally{if(generation===this.generation)this.busy.set(false);}
  }
  clearFilters():void{this.filters.set({category_code:'',rate_unit:'',min_unit_price_clp:'',max_unit_price_clp:'',sort:'default'});this.items.set([]);this.nextCursor.set('');this.error.set('');void this.router.navigate([], {relativeTo:this.route,queryParams:{category_code:null,rate_unit:null,min_unit_price_clp:null,max_unit_price_clp:null,sort:null},replaceUrl:true});void this.search(true,false);}
  loadMore():void{if(this.nextCursor()&&!this.busy())void this.search(false,false);}
  price(value:string):string{return `CLP ${BigInt(value).toLocaleString('es-CL')}`;}
  private async loadCategories():Promise<void>{try{this.categories.set(await this.api.publicCategories());}catch{this.error.set('No fue posible cargar las categorías públicas.');}}
}
function validPrice(value?:string):boolean{return !value||/^(0|[1-9]\d*)$/.test(value);}
