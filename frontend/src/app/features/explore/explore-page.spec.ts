import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExplorePage } from './explore-page';
import { ExploreApi } from './infrastructure/explore-api';

describe('ExplorePage public catalog',()=>{
 afterEach(()=>TestBed.resetTestingModule());
 it('loads categories and listings anonymously and preserves exact CLP strings',async()=>{
  const query=new BehaviorSubject(convertToParamMap({}));
  const api={publicCategories:vi.fn().mockResolvedValue([{code:'oficina',name:'Oficina',order:1}]),publicSearch:vi.fn().mockResolvedValue({items:[{space_id:'public-1',category_code:'oficina',category_name:'Oficina',title:'Taller',description:'Espacio visible',area_m2:24,rate_unit:'hora',base_price_clp:'9007199254740993',currency:'CLP'}]})};
  TestBed.configureTestingModule({imports:[ExplorePage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{queryParamMap:query.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(ExplorePage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicSearch).toHaveBeenCalled());await vi.waitFor(()=>expect((fixture.componentInstance as unknown as {items:()=>unknown[]}).items()).toHaveLength(1));fixture.detectChanges();
  expect(api.publicCategories).toHaveBeenCalledOnce();expect(api.publicSearch.mock.calls[0][0]).toMatchObject({page_size:20,sort:'default'});
  expect(fixture.nativeElement.textContent).toContain('CLP 9.007.199.254.740.993');expect(fixture.nativeElement.textContent).not.toContain('CATÁLOGO AUTENTICADO');fixture.destroy();
 });
 it('invalidates an in-flight request when public criteria change',async()=>{
  const query=new BehaviorSubject(convertToParamMap({}));let resolve!: (value:unknown)=>void;const pending=new Promise(r=>resolve=r);
  const api={publicCategories:vi.fn().mockResolvedValue([]),publicSearch:vi.fn().mockReturnValueOnce(pending).mockResolvedValue({items:[]})};
  TestBed.configureTestingModule({imports:[ExplorePage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{queryParamMap:query.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(ExplorePage);fixture.detectChanges();const page=fixture.componentInstance as unknown as {items:()=>unknown[];updateFilter(key:string,value:string):void;search():Promise<void>};
  await vi.waitFor(()=>expect(api.publicSearch).toHaveBeenCalledTimes(1));page.updateFilter('category_code','oficina');const next=page.search();resolve({items:[{space_id:'stale'}]});await next;expect(page.items()).toEqual([]);fixture.destroy();
 });
});
