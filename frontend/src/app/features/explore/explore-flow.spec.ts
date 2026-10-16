import { provideHttpClient } from '@angular/common/http';
import { provideLocationMocks } from '@angular/common/testing';
import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterOutlet, provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExploreApi } from './infrastructure/explore-api';
import { ExplorePage } from './explore-page';
import { SpaceDetailPage } from './space-detail-page';

@Component({standalone:true,imports:[RouterOutlet],template:'<router-outlet />'}) class Host {}
describe('anonymous public catalog journey',()=>{
 afterEach(()=>TestBed.resetTestingModule());
 it('opens explore and a public detail route without renter guards',async()=>{
  const listing={space_id:'space-public',category_code:'oficina',category_name:'Oficina',title:'Taller abierto',description:'Descripción pública',area_m2:22,rate_unit:'hora',base_price_clp:'9007199254740993',currency:'CLP'};
  const api={publicCategories:vi.fn().mockResolvedValue([{code:'oficina',name:'Oficina',order:1}]),publicSearch:vi.fn().mockResolvedValue({items:[listing]}),publicDetail:vi.fn().mockResolvedValue(listing)};
  TestBed.configureTestingModule({imports:[Host],providers:[...provideFrontendPorts(),provideHttpClient(),provideLocationMocks(),provideRouter([{path:'explorar',component:ExplorePage},{path:'espacios/:id',component:SpaceDetailPage}]),{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(Host);fixture.detectChanges();const router=TestBed.inject(Router);await router.navigateByUrl('/explorar');fixture.detectChanges();await vi.waitFor(()=>expect(fixture.nativeElement.textContent).toContain('Taller abierto'));
  await router.navigateByUrl('/espacios/space-public');fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-public'));await vi.waitFor(()=>expect(fixture.nativeElement.textContent).toContain('CLP 9.007.199.254.740.993'));fixture.destroy();
 });
});
